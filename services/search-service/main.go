// Command search-service is the Search Service binary — see
// docs/architecture/microservices.md §9.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	searchmigrations "github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/migrations"

	chunksclient "github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/chunks/http"
	eventskafka "github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/events/kafka"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/llm/ollama"
	meetingsclient "github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/meetings/http"
	searchpg "github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/repository/postgres"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/tracing"
)

// serviceConfig is Search Service's whole configuration surface — see
// configs/search-service.template.json for the shape and dev-safe
// defaults.
type serviceConfig struct {
	Port                  string   `json:"port"`
	LogLevel              string   `json:"log_level"`
	OtelCollectorEndpoint string   `json:"otel_collector_endpoint"`
	DatabaseURL           string   `json:"database_url"`
	AISummaryServiceURL   string   `json:"ai_summary_service_url"`
	MeetingServiceURL     string   `json:"meeting_service_url"`
	InternalServiceToken  string   `json:"internal_service_token"`
	OllamaURL             string   `json:"ollama_url"`
	OllamaEmbedModel      string   `json:"ollama_embed_model"`
	OllamaChatModel       string   `json:"ollama_chat_model"`
	KafkaBrokers          []string `json:"kafka_brokers"`
}

func main() {
	cfg := loadConfig()
	log := logger.New("search-service", logger.ParseLevel(cfg.LogLevel))
	shutdownTracing, err := tracing.Init(context.Background(), "search-service", cfg.OtelCollectorEndpoint)
	if err != nil {
		log.Error("init tracing", "err", err)
	} else {
		defer func() { _ = shutdownTracing(context.Background()) }()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool := initPostgres(ctx, cfg, log)
	defer pool.Close()

	chunksClient := chunksclient.NewClient(cfg.AISummaryServiceURL, cfg.InternalServiceToken)
	meetingsClient := meetingsclient.NewClient(cfg.MeetingServiceURL, cfg.InternalServiceToken)
	embedder := ollama.NewEmbedClient(cfg.OllamaURL, cfg.OllamaEmbedModel)
	answerer := ollama.NewAnswerClient(cfg.OllamaURL, cfg.OllamaChatModel)

	publisher := eventskafka.NewPublisher(cfg.KafkaBrokers)
	defer func() { _ = publisher.Close() }()

	repo := searchpg.NewSearchRepository(pool)
	embedChunks := usecase.NewEmbedChunksUseCase(chunksClient, embedder, repo, publisher, log, cfg.OllamaEmbedModel)
	search := usecase.NewSearchUseCase(embedder, repo, meetingsClient)

	handler := NewHandler(
		search,
		usecase.NewSimilarMeetingsUseCase(repo, meetingsClient),
		usecase.NewAskUseCase(search, answerer, repo, cfg.OllamaChatModel),
		usecase.NewGetHistoryUseCase(repo),
		usecase.NewReindexUseCase(meetingsClient, embedChunks, log),
	)

	// Kafka connects lazily (see shared/kafkax.NewReader's doc comment on
	// the writer side) — a broker that's down at startup doesn't block
	// the REST server from coming up; ConsumeChunkCreated just retries
	// its fetch loop until one's reachable.
	reader := kafkax.NewReader(cfg.KafkaBrokers, topicChunkCreated, "search-service")
	defer func() { _ = reader.Close() }()
	go ConsumeChunkCreated(ctx, reader, embedChunks, publisher, log)

	srv := httpserver.New("search-service", log)
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
	configPath := flag.String("config", "deployments/configs/search-service.json", "path to config JSON file")
	secretsPath := flag.String("secrets", "", "optional path to a second JSON file overlaid onto -config (e.g. a Kubernetes Secret-mounted file holding credentials) — see shared/config.LoadMerged")
	flag.Parse()

	cfg, err := config.LoadMerged[serviceConfig](*configPath, *secretsPath)
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
	if err := dbx.RunMigrations(ctx, pool, "search", searchmigrations.FS, "."); err != nil {
		log.Error("run migrations", "err", err)
		os.Exit(1)
	}
	return pool
}
