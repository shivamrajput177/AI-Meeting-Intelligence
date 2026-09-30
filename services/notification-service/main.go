// Command notification-service is the Notification Service binary — see
// docs/architecture/microservices.md §10.
//
// Phase 4.2 gave this service its first REST routes: the mock Jira board
// (handler.go/routes.go).
//
// Phase 4.4 adds the reminder scheduler (scheduler.go) — a leader-elected
// goroutine, unlike every other loop here, since it's the one place two
// replicas acting at once could double-publish the same reminder (see
// scheduler.go's own doc comment).
//
// Phase 4.5 makes the Slack webhook and ticket provider per-org:
// DispatchUseCase now reads each row's org's own org.integration_configs
// (via orgs.Client, against Organization Service's internal API) and
// only falls back to this service's dev-config-wide SlackWebhookURL when
// an org hasn't set its own. cfg.SMTPHost/Port/From remain single
// dev-config-wide values — Phase 4.5's scope (per
// docs/ROADMAP.md) was Slack webhook + ticket_provider + Jira
// project/token, not SMTP. ticketProviders is a registry keyed by
// ticket_provider value; only "mock_jira" has an entry — an org
// configured for "atlassian_jira" gets a clear "not implemented yet"
// dispatch error (see usecase.resolveTicketProvider) rather than
// silently using the wrong provider, since Phase 4.3's real Jira
// provider is still an unbuilt stretch job. Phase 4.5 also finally builds
// POST /orgs/{orgId}/integrations/test (handler.go's TestIntegration),
// deferred since Phase 4.2 for lack of anything real to test against —
// it now has one, via the same orgs.Client/ticketProviders resolution
// DispatchUseCase itself uses.
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

	actionitemsclient "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/actionitems/http"
	emailsmtp "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/email/smtp"
	eventskafka "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/events/kafka"
	meetingsclient "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/meetings/http"
	orgsclient "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/orgs/http"
	notificationpg "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/repository/postgres"
	slackhttp "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/slack/http"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/ticketprovider"
	mockjira "github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/ticketprovider/mock"
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
	ActionItemServiceURL string   `json:"action_item_service_url"`
	OrgServiceURL        string   `json:"org_service_url"`
	InternalServiceToken string   `json:"internal_service_token"`
	KafkaBrokers         []string `json:"kafka_brokers"`
	SlackWebhookURL      string   `json:"slack_webhook_url"`
	SMTPHost             string   `json:"smtp_host"`
	SMTPPort             string   `json:"smtp_port"`
	SMTPFrom             string   `json:"smtp_from"`
	// PublicAPIBaseURL is where a created mock ticket's URL points (e.g.
	// "http://localhost:8000/api/v1") — see ticketprovider/mock's own doc
	// comment on why the mock board needs one supplied rather than
	// deriving it from a real Jira Cloud site's own address.
	PublicAPIBaseURL string `json:"public_api_base_url"`
	// DemoOrgID scopes GET /demo/board to one operator-designated org —
	// see handler.DemoBoard's doc comment. Empty by default: seeding an
	// actual public demo org is Phase 7's job
	// (deployment-demo-strategy.md §2), not this phase's.
	DemoOrgID string `json:"demo_org_id"`
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
	actionItemsC := actionitemsclient.NewClient(cfg.ActionItemServiceURL, cfg.InternalServiceToken)
	orgsC := orgsclient.NewClient(cfg.OrgServiceURL, cfg.InternalServiceToken)
	slackSender := slackhttp.New()
	emailSender := emailsmtp.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)

	publisher := eventskafka.NewPublisher(cfg.KafkaBrokers)
	defer func() { _ = publisher.Close() }()

	repo := notificationpg.NewOutboxRepository(pool)
	jiraRepo := notificationpg.NewJiraRepository(pool)
	reminderRepo := notificationpg.NewReminderRepository(pool)
	// ticketProviders is keyed by org.integration_configs.ticket_provider
	// values — only mockjira.ProviderName ("mock_jira") has an entry as of
	// this phase; see DispatchUseCase.ticketProviderFor's doc comment for
	// what happens when an org is configured for anything else.
	ticketProviders := map[string]ticketprovider.Provider{
		mockjira.ProviderName: mockjira.New(jiraRepo, cfg.PublicAPIBaseURL),
	}

	enqueueSummary := usecase.NewEnqueueSummaryNotificationsUseCase(meetingsC, usersC, repo, log)
	enqueueDigest := usecase.NewEnqueueActionItemDigestUseCase(meetingsC, repo)
	enqueueJiraTicket := usecase.NewEnqueueJiraTicketUseCase(repo)
	enqueueReminder := usecase.NewEnqueueReminderUseCase(repo)
	publishDueReminders := usecase.NewPublishDueRemindersUseCase(reminderRepo, publisher, log)
	dispatch := usecase.NewDispatchUseCase(repo, jiraRepo, slackSender, emailSender, ticketProviders, actionItemsC, orgsC, publisher, log, cfg.SlackWebhookURL)
	testIntegration := usecase.NewTestIntegrationUseCase(orgsC, slackSender, emailSender, ticketProviders, cfg.SlackWebhookURL, log)

	handler := NewHandler(
		usecase.NewGetMockBoardUseCase(jiraRepo),
		usecase.NewTransitionMockIssueUseCase(jiraRepo, actionItemsC, log),
		testIntegration,
		cfg.DemoOrgID,
	)

	// Kafka connects lazily (see shared/kafkax.NewReader's doc comment on
	// the writer side) — a broker that's down at startup doesn't block
	// the REST server from coming up; each Consume* loop just retries its
	// own fetch loop until one's reachable.
	summaryReader := kafkax.NewReader(cfg.KafkaBrokers, topicSummaryCompleted, consumeGroupID)
	actionItemReader := kafkax.NewReader(cfg.KafkaBrokers, topicActionItemExtracted, consumeGroupID)
	jiraRequestedReader := kafkax.NewReader(cfg.KafkaBrokers, topicActionItemJiraRequested, consumeGroupID)
	reminderDueReader := kafkax.NewReader(cfg.KafkaBrokers, topicActionItemReminderDue, consumeGroupID)
	defer func() { _ = summaryReader.Close() }()
	defer func() { _ = actionItemReader.Close() }()
	defer func() { _ = jiraRequestedReader.Close() }()
	defer func() { _ = reminderDueReader.Close() }()

	go ConsumeSummaryCompleted(ctx, summaryReader, enqueueSummary, log)
	go ConsumeActionItemExtracted(ctx, actionItemReader, enqueueDigest, log)
	go ConsumeActionItemJiraRequested(ctx, jiraRequestedReader, enqueueJiraTicket, log)
	go ConsumeActionItemReminderDue(ctx, reminderDueReader, enqueueReminder, log)
	go RunDispatchPoller(ctx, dispatch, log)
	go RunReminderScheduler(ctx, publishDueReminders, log)

	srv := httpserver.New("notification-service", log)
	RegisterRoutes(srv.Mux, handler)

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
