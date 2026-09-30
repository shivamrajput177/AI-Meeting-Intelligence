// Command notification-service is the Notification Service binary — see
// docs/architecture/microservices.md §10.
//
// Phase 4.1 gives this service no REST routes of its own beyond the free
// /healthz and /readyz httpserver.New already registers: "This service
// has no synchronous public surface beyond the routes below" (§10) — and
// both of those routes (POST /orgs/{orgId}/integrations/test,
// GET /demo/board) are deferred: the test-trigger endpoint needs Phase
// 4.5's org-level integration config to have anything real to test
// against, and the mock-board routes are Phase 4.2's job. Today this
// service is purely Kafka consumer + poller.
//
// slackWebhookURL/SMTP settings are a single dev-config-wide value for
// now, not per-org — Phase 4.5's Organization Service integration config
// (Slack webhook URL, ticket_provider, Jira project/token) is what makes
// this per-tenant; wiring that in only changes where DispatchUseCase
// reads its webhook URL from, not its dispatch logic.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	notificationmigrations "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/migrations"

	emailsmtp "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/email/smtp"
	eventskafka "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/events/kafka"
	meetingsclient "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/meetings/http"
	notificationpg "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/repository/postgres"
	slackhttp "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/slack/http"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/usecase"
	usersclient "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/users/http"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// serviceConfig is Notification Service's whole configuration surface —
// see configs/notification-service.template.json for the shape and
// dev-safe defaults.
type serviceConfig struct {
	Port                 string   `json:"port"`
	LogLevel             string   `json:"log_level"`
	DatabaseURL          string   `json:"database_url"`
	MeetingServiceURL    string   `json:"meeting_service_url"`
	UserServiceURL       string   `json:"user_service_url"`
	InternalServiceToken string   `json:"internal_service_token"`
	KafkaBrokers         []string `json:"kafka_brokers"`
	SlackWebhookURL      string   `json:"slack_webhook_url"`
	SMTPHost             string   `json:"smtp_host"`
	SMTPPort             string   `json:"smtp_port"`
	SMTPFrom             string   `json:"smtp_from"`
}

func main() {
	cfg := loadConfig()
	log := logger.New("notification-service", logger.ParseLevel(cfg.LogLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool := initPostgres(ctx, cfg, log)
	defer pool.Close()

	meetingsC := meetingsclient.NewClient(cfg.MeetingServiceURL, cfg.InternalServiceToken)
	usersC := usersclient.NewClient(cfg.UserServiceURL, cfg.InternalServiceToken)
	slackSender := slackhttp.New()
	emailSender := emailsmtp.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)

	publisher := eventskafka.NewPublisher(cfg.KafkaBrokers)
	defer func() { _ = publisher.Close() }()

	repo := notificationpg.NewOutboxRepository(pool)
	enqueueSummary := usecase.NewEnqueueSummaryNotificationsUseCase(meetingsC, usersC, repo, log)
	enqueueDigest := usecase.NewEnqueueActionItemDigestUseCase(meetingsC, repo)
	dispatch := usecase.NewDispatchUseCase(repo, slackSender, emailSender, publisher, log, cfg.SlackWebhookURL)

	// Kafka connects lazily (see shared/kafkax.NewReader's doc comment on
	// the writer side) — a broker that's down at startup doesn't block
	// the REST server from coming up; each Consume* loop just retries its
	// own fetch loop until one's reachable.
	summaryReader := kafkax.NewReader(cfg.KafkaBrokers, topicSummaryCompleted, consumeGroupID)
	actionItemReader := kafkax.NewReader(cfg.KafkaBrokers, topicActionItemExtracted, consumeGroupID)
	defer func() { _ = summaryReader.Close() }()
	defer func() { _ = actionItemReader.Close() }()

	go ConsumeSummaryCompleted(ctx, summaryReader, enqueueSummary, log)
	go ConsumeActionItemExtracted(ctx, actionItemReader, enqueueDigest, log)
	go RunDispatchPoller(ctx, dispatch, log)

	srv := httpserver.New("notification-service", log)

	addr := ":" + cfg.Port
	log.Info("starting", "addr", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// loadConfig parses -config and reads the JSON file it points at,
// exiting the process on failure — there's no sensible fallback for a
// service that can't find out what port to listen on.
func loadConfig() serviceConfig {
	configPath := flag.String("config", "deployments/configs/notification-service.json", "path to config JSON file")
	flag.Parse()

	cfg, err := config.Load[serviceConfig](*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}
	return cfg
}

// initPostgres opens the connection pool and applies this service's own
// migrations, exiting on failure — a service with no database or a
// broken schema has nothing useful to do.
func initPostgres(ctx context.Context, cfg serviceConfig, log *logger.Logger) *pgxpool.Pool {
	pool, err := dbx.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect to postgres", "err", err)
		os.Exit(1)
	}
	if err := dbx.RunMigrations(ctx, pool, "notification", notificationmigrations.FS, "."); err != nil {
		log.Error("run migrations", "err", err)
		os.Exit(1)
	}
	return pool
}
