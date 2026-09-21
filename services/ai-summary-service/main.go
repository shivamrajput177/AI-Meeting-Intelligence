// Command ai-summary-service is the AI Summary Service binary — see
// docs/architecture/microservices.md §7.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	aisummarymigrations "github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/migrations"

	eventskafka "github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/events/kafka"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/llm/ollama"
	aisummarypg "github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/repository/postgres"
	transcriptclient "github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/transcript/http"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// serviceConfig is ai-summary-service's whole configuration surface —
// see configs/ai-summary-service.template.json for the shape and
// dev-safe defaults.
type serviceConfig struct {
	Port                    string   `json:"port"`
	LogLevel                string   `json:"log_level"`
	DatabaseURL             string   `json:"database_url"`
	TranscriptionServiceURL string   `json:"transcription_service_url"`
	InternalServiceToken    string   `json:"internal_service_token"`
	OllamaURL               string   `json:"ollama_url"`
	OllamaModel             string   `json:"ollama_model"`
	KafkaBrokers            []string `json:"kafka_brokers"`
}

func main() {
	cfg := loadConfig()
	log := logger.New("ai-summary-service", logger.ParseLevel(cfg.LogLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool := initPostgres(ctx, cfg, log)
	defer pool.Close()

	transcriptClient := transcriptclient.NewClient(cfg.TranscriptionServiceURL, cfg.InternalServiceToken)
	summarizer := ollama.New(cfg.OllamaURL, cfg.OllamaModel)

	publisher := eventskafka.NewPublisher(cfg.KafkaBrokers)
	defer func() { _ = publisher.Close() }()

	repo := aisummarypg.NewSummaryRepository(pool)
	processTranscript := usecase.NewProcessTranscriptUseCase(
		transcriptClient, summarizer, repo, publisher, log, cfg.OllamaModel, ollama.PromptVersion(),
	)
	handler := NewHandler(usecase.NewGetSummaryUseCase(repo), processTranscript)

	// Kafka connects lazily (see shared/kafkax.NewReader's doc comment on
	// the writer side) — a broker that's down at startup doesn't block
	// the REST server from coming up; ConsumeTranscriptionCompleted just
	// retries its fetch loop until one's reachable.
	reader := kafkax.NewReader(cfg.KafkaBrokers, topicTranscriptionCompleted, "ai-summary-service")
	defer func() { _ = reader.Close() }()
	go ConsumeTranscriptionCompleted(ctx, reader, processTranscript, publisher, log)

	srv := httpserver.New("ai-summary-service", log)
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
	configPath := flag.String("config", "deployments/configs/ai-summary-service.json", "path to config JSON file")
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
	if err := dbx.RunMigrations(ctx, pool, "ai", aisummarymigrations.FS, "."); err != nil {
		log.Error("run migrations", "err", err)
		os.Exit(1)
	}
	return pool
}
