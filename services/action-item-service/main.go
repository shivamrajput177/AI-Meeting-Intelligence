// Command action-item-service is the Action Item Service binary — see
// docs/architecture/microservices.md §8.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	actionitemmigrations "github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/migrations"

	eventskafka "github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/events/kafka"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/llm/ollama"
	participantsclient "github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/participants/http"
	actionitempg "github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/repository/postgres"
	summaryclient "github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/summary/http"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// serviceConfig is Action Item Service's whole configuration surface —
// see configs/action-item-service.template.json for the shape and
// dev-safe defaults.
type serviceConfig struct {
	Port                 string   `json:"port"`
	LogLevel             string   `json:"log_level"`
	DatabaseURL          string   `json:"database_url"`
	SummaryServiceURL    string   `json:"summary_service_url"`
	MeetingServiceURL    string   `json:"meeting_service_url"`
	InternalServiceToken string   `json:"internal_service_token"`
	OllamaURL            string   `json:"ollama_url"`
	OllamaModel          string   `json:"ollama_model"`
	KafkaBrokers         []string `json:"kafka_brokers"`
}

func main() {
	cfg := loadConfig()
	log := logger.New("action-item-service", logger.ParseLevel(cfg.LogLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool := initPostgres(ctx, cfg, log)
	defer pool.Close()

	summaryC := summaryclient.NewClient(cfg.SummaryServiceURL, cfg.InternalServiceToken)
	participantsC := participantsclient.NewClient(cfg.MeetingServiceURL, cfg.InternalServiceToken)
	extractor := ollama.New(cfg.OllamaURL, cfg.OllamaModel)

	publisher := eventskafka.NewPublisher(cfg.KafkaBrokers)
	defer func() { _ = publisher.Close() }()

	repo := actionitempg.NewActionItemRepository(pool)
	extract := usecase.NewExtractActionItemsUseCase(summaryC, participantsC, extractor, repo, publisher, log)
	handler := NewHandler(
		usecase.NewGetActionItemUseCase(repo),
		usecase.NewListActionItemsByMeetingUseCase(repo),
		usecase.NewListActionItemsUseCase(repo),
		usecase.NewUpdateActionItemUseCase(repo),
	)

	// Kafka connects lazily (see shared/kafkax.NewReader's doc comment on
	// the writer side) — a broker that's down at startup doesn't block
	// the REST server from coming up; ConsumeSummaryCompleted just
	// retries its fetch loop until one's reachable.
	reader := kafkax.NewReader(cfg.KafkaBrokers, topicSummaryCompleted, "action-item-service")
	defer func() { _ = reader.Close() }()
	go ConsumeSummaryCompleted(ctx, reader, extract, publisher, log)

	srv := httpserver.New("action-item-service", log)
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
	configPath := flag.String("config", "deployments/configs/action-item-service.json", "path to config JSON file")
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
	if err := dbx.RunMigrations(ctx, pool, "actionitem", actionitemmigrations.FS, "."); err != nil {
		log.Error("run migrations", "err", err)
		os.Exit(1)
	}
	return pool
}
