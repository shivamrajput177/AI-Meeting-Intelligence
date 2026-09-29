// Command analytics-service is the Analytics Service binary — see
// docs/architecture/microservices.md §11.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	analyticsmigrations "github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/migrations"

	actionitemsclient "github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/actionitems/http"
	meetingsclient "github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/meetings/http"
	analyticspg "github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/repository/postgres"
	summaryclient "github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/summary/http"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// serviceConfig is Analytics Service's whole configuration surface — see
// configs/analytics-service.template.json for the shape and dev-safe
// defaults.
type serviceConfig struct {
	Port                 string   `json:"port"`
	LogLevel             string   `json:"log_level"`
	DatabaseURL          string   `json:"database_url"`
	MeetingServiceURL    string   `json:"meeting_service_url"`
	ActionItemServiceURL string   `json:"action_item_service_url"`
	AISummaryServiceURL  string   `json:"ai_summary_service_url"`
	InternalServiceToken string   `json:"internal_service_token"`
	KafkaBrokers         []string `json:"kafka_brokers"`
}

func main() {
	cfg := loadConfig()
	log := logger.New("analytics-service", logger.ParseLevel(cfg.LogLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool := initPostgres(ctx, cfg, log)
	defer pool.Close()

	meetingsC := meetingsclient.NewClient(cfg.MeetingServiceURL, cfg.InternalServiceToken)
	actionItemsC := actionitemsclient.NewClient(cfg.ActionItemServiceURL, cfg.InternalServiceToken)
	summaryC := summaryclient.NewClient(cfg.AISummaryServiceURL, cfg.InternalServiceToken)

	repo := analyticspg.NewRollupRepository(pool)
	recordMeetingStatus := usecase.NewRecordMeetingStatusChangedUseCase(meetingsC, repo)
	recordActionItemExtracted := usecase.NewRecordActionItemsExtractedUseCase(actionItemsC, repo)
	recordActionItemStatus := usecase.NewRecordActionItemStatusChangedUseCase(repo)
	recordTopics := usecase.NewRecordTopicsFromSummaryUseCase(summaryC, repo)

	handler := NewHandler(
		usecase.NewGetMeetingTrendsUseCase(repo),
		usecase.NewGetProductivityUseCase(repo),
		usecase.NewGetCompletionRateUseCase(repo),
		usecase.NewGetTopicsUseCase(repo),
	)

	// Kafka connects lazily (see shared/kafkax.NewReader's doc comment on
	// the writer side) — a broker that's down at startup doesn't block the
	// REST server from coming up; each Consume* loop just retries its own
	// fetch loop until one's reachable. One reader per topic, all under
	// this service's own consumer group (see consumer.go's doc comment).
	meetingStatusReader := kafkax.NewReader(cfg.KafkaBrokers, topicMeetingStatusChanged, consumeGroupID)
	actionItemExtractedReader := kafkax.NewReader(cfg.KafkaBrokers, topicActionItemExtracted, consumeGroupID)
	actionItemStatusReader := kafkax.NewReader(cfg.KafkaBrokers, topicActionItemStatusChanged, consumeGroupID)
	summaryCompletedReader := kafkax.NewReader(cfg.KafkaBrokers, topicSummaryCompleted, consumeGroupID)
	defer func() { _ = meetingStatusReader.Close() }()
	defer func() { _ = actionItemExtractedReader.Close() }()
	defer func() { _ = actionItemStatusReader.Close() }()
	defer func() { _ = summaryCompletedReader.Close() }()

	go ConsumeMeetingStatusChanged(ctx, meetingStatusReader, recordMeetingStatus, log)
	go ConsumeActionItemExtracted(ctx, actionItemExtractedReader, recordActionItemExtracted, log)
	go ConsumeActionItemStatusChanged(ctx, actionItemStatusReader, recordActionItemStatus, log)
	go ConsumeSummaryCompleted(ctx, summaryCompletedReader, recordTopics, log)

	srv := httpserver.New("analytics-service", log)
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
	configPath := flag.String("config", "deployments/configs/analytics-service.json", "path to config JSON file")
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
	if err := dbx.RunMigrations(ctx, pool, "analytics", analyticsmigrations.FS, "."); err != nil {
		log.Error("run migrations", "err", err)
		os.Exit(1)
	}
	return pool
}
