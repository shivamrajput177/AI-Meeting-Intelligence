// Command meeting-service is the Meeting Service binary — see
// docs/architecture/microservices.md §5.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	meetingmigrations "github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/migrations"

	eventskafka "github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/events/kafka"
	meetingpg "github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/repository/postgres"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/storage/minio"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// serviceConfig is meeting-service's whole configuration surface — see
// configs/meeting-service.template.json for the shape and dev-safe
// defaults.
type serviceConfig struct {
	Port                  string   `json:"port"`
	LogLevel              string   `json:"log_level"`
	DatabaseURL           string   `json:"database_url"`
	MinIOInternalEndpoint string   `json:"minio_internal_endpoint"`
	MinIOPublicEndpoint   string   `json:"minio_public_endpoint"`
	MinIOAccessKey        string   `json:"minio_access_key"`
	MinIOSecretKey        string   `json:"minio_secret_key"`
	MinIOBucket           string   `json:"minio_bucket"`
	MinIOUseSSL           bool     `json:"minio_use_ssl"`
	KafkaBrokers          []string `json:"kafka_brokers"`
}

func main() {
	cfg := loadConfig()
	log := logger.New("meeting-service", logger.ParseLevel(cfg.LogLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool := initPostgres(ctx, cfg, log)
	defer pool.Close()

	storage := initStorage(ctx, cfg, log)

	publisher := eventskafka.NewPublisher(cfg.KafkaBrokers)
	defer func() { _ = publisher.Close() }()

	repo := meetingpg.NewMeetingRepository(pool)
	handler := NewHandler(
		usecase.NewCreateUploadIntentUseCase(repo, storage),
		usecase.NewConfirmUploadUseCase(repo, storage, publisher, log),
		usecase.NewGetMeetingUseCase(repo),
		usecase.NewListMeetingsUseCase(repo),
		usecase.NewUpdateStatusUseCase(repo),
		usecase.NewDeleteMeetingUseCase(repo, storage),
	)

	srv := httpserver.New("meeting-service", log)
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
	configPath := flag.String("config", "deployments/configs/meeting-service.json", "path to config JSON file")
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
	if err := dbx.RunMigrations(ctx, pool, "meeting", meetingmigrations.FS, "."); err != nil {
		log.Error("run migrations", "err", err)
		os.Exit(1)
	}
	return pool
}

// initStorage builds the MinIO client and makes sure its bucket exists,
// exiting on failure. See storage/minio's doc comment for why the
// internal/public endpoints are two different values.
func initStorage(ctx context.Context, cfg serviceConfig, log *logger.Logger) *minio.Client {
	storage, err := minio.New(
		cfg.MinIOInternalEndpoint,
		cfg.MinIOPublicEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOBucket,
		cfg.MinIOUseSSL,
	)
	if err != nil {
		log.Error("init minio client", "err", err)
		os.Exit(1)
	}
	if err := storage.EnsureBucket(ctx); err != nil {
		log.Error("ensure minio bucket", "err", err)
		os.Exit(1)
	}
	return storage
}
