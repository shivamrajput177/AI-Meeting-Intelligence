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
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/tracing"
)

// serviceConfig is meeting-service's whole configuration surface — see
// configs/meeting-service.template.json for the shape and dev-safe
// defaults.
type serviceConfig struct {
	Port                  string   `json:"port"`
	LogLevel              string   `json:"log_level"`
	OtelCollectorEndpoint string   `json:"otel_collector_endpoint"`
	DatabaseURL           string   `json:"database_url"`
	MinIOInternalEndpoint string   `json:"minio_internal_endpoint"`
	MinIOPublicEndpoint   string   `json:"minio_public_endpoint"`
	MinIOAccessKey        string   `json:"minio_access_key"`
	MinIOSecretKey        string   `json:"minio_secret_key"`
	MinIOBucket           string   `json:"minio_bucket"`
	MinIOUseSSL           bool     `json:"minio_use_ssl"`
	KafkaBrokers          []string `json:"kafka_brokers"`
	InternalServiceToken  string   `json:"internal_service_token"`
	// MaxUploadBytes is Phase 7's public-demo clip-length guard — 0 (every
	// non-demo config) means unlimited. See ConfirmUploadUseCase's doc
	// comment for why this is a size cap, not a true duration cap.
	MaxUploadBytes int64 `json:"max_upload_bytes"`
}

func main() {
	cfg := loadConfig()
	log := logger.New("meeting-service", logger.ParseLevel(cfg.LogLevel))
	shutdownTracing, err := tracing.Init(context.Background(), "meeting-service", cfg.OtelCollectorEndpoint)
	if err != nil {
		log.Error("init tracing", "err", err)
	} else {
		defer func() { _ = shutdownTracing(context.Background()) }()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool := initPostgres(ctx, cfg, log)
	defer pool.Close()

	storage := initStorage(ctx, cfg, log)

	publisher := eventskafka.NewPublisher(cfg.KafkaBrokers)
	defer func() { _ = publisher.Close() }()

	repo := meetingpg.NewMeetingRepository(pool)
	updateStatus := usecase.NewUpdateStatusUseCase(repo, publisher, log)
	handler := NewHandler(
		usecase.NewCreateUploadIntentUseCase(repo, storage),
		usecase.NewConfirmUploadUseCase(repo, storage, publisher, log, cfg.MaxUploadBytes),
		usecase.NewGetMeetingUseCase(repo),
		usecase.NewListMeetingsUseCase(repo),
		updateStatus,
		usecase.NewDeleteMeetingUseCase(repo, storage),
		usecase.NewGetParticipantsUseCase(repo),
	)

	// Kafka connects lazily (see shared/kafkax.NewReader's doc comment on
	// the writer side) — a broker that's down at startup doesn't block the
	// REST server from coming up; each consumeStatusEvent loop just
	// retries its own fetch until one's reachable. One reader per topic
	// (see statusConsumers), all under this service's own consumer group.
	for _, c := range statusConsumers {
		reader := kafkax.NewReader(cfg.KafkaBrokers, c.topic, "meeting-service")
		defer func() { _ = reader.Close() }()
		go consumeStatusEvent(ctx, reader, updateStatus, c.status, log)
	}

	srv := httpserver.New("meeting-service", log)
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
	configPath := flag.String("config", "deployments/configs/meeting-service.json", "path to config JSON file")
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
