// Command transcription-service is the Transcription Service binary —
// see docs/architecture/microservices.md §6.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	transcriptionmigrations "github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/migrations"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/asr/whispercpp"
	eventskafka "github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/events/kafka"
	transcriptionpg "github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/repository/postgres"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/storage/minio"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// serviceConfig is transcription-service's whole configuration surface —
// see configs/transcription-service.template.json for the shape and
// dev-safe defaults.
type serviceConfig struct {
	Port                  string   `json:"port"`
	LogLevel              string   `json:"log_level"`
	DatabaseURL           string   `json:"database_url"`
	MinIOInternalEndpoint string   `json:"minio_internal_endpoint"`
	MinIOAccessKey        string   `json:"minio_access_key"`
	MinIOSecretKey        string   `json:"minio_secret_key"`
	MinIOBucket           string   `json:"minio_bucket"`
	MinIOUseSSL           bool     `json:"minio_use_ssl"`
	WhisperServerURL      string   `json:"whisper_server_url"`
	KafkaBrokers          []string `json:"kafka_brokers"`
	InternalServiceToken  string   `json:"internal_service_token"`
}

func main() {
	cfg := loadConfig()
	log := logger.New("transcription-service", logger.ParseLevel(cfg.LogLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool := initPostgres(ctx, cfg, log)
	defer pool.Close()

	objectStorage := initStorage(cfg, log)
	transcriber := whispercpp.New(cfg.WhisperServerURL)

	publisher := eventskafka.NewPublisher(cfg.KafkaBrokers)
	defer func() { _ = publisher.Close() }()

	repo := transcriptionpg.NewTranscriptRepository(pool)
	processUpload := usecase.NewProcessUploadUseCase(repo, objectStorage, transcriber, publisher, log)
	handler := NewHandler(usecase.NewGetTranscriptUseCase(repo))

	// Kafka connects lazily (see shared/kafkax.NewReader's doc comment on
	// the writer side) — a broker that's down at startup doesn't block
	// the REST server from coming up; ConsumeMeetingUploaded just retries
	// its fetch loop until one's reachable.
	reader := kafkax.NewReader(cfg.KafkaBrokers, topicMeetingUploaded, "transcription-service")
	defer func() { _ = reader.Close() }()
	go ConsumeMeetingUploaded(ctx, reader, processUpload, publisher, log)

	srv := httpserver.New("transcription-service", log)
	RegisterRoutes(srv.Mux, handler, cfg.InternalServiceToken)

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
	configPath := flag.String("config", "deployments/configs/transcription-service.json", "path to config JSON file")
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
	if err := dbx.RunMigrations(ctx, pool, "transcription", transcriptionmigrations.FS, "."); err != nil {
		log.Error("run migrations", "err", err)
		os.Exit(1)
	}
	return pool
}

// initStorage builds the read-only MinIO client this service fetches
// recordings through, exiting on failure.
func initStorage(cfg serviceConfig, log *logger.Logger) *minio.Client {
	client, err := minio.New(cfg.MinIOInternalEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
	if err != nil {
		log.Error("init minio client", "err", err)
		os.Exit(1)
	}
	return client
}
