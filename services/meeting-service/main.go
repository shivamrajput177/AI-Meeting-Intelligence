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

	meetingmigrations "github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/migrations"

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
	Port                  string `json:"port"`
	LogLevel              string `json:"log_level"`
	DatabaseURL           string `json:"database_url"`
	MinIOInternalEndpoint string `json:"minio_internal_endpoint"`
	MinIOPublicEndpoint   string `json:"minio_public_endpoint"`
	MinIOAccessKey        string `json:"minio_access_key"`
	MinIOSecretKey        string `json:"minio_secret_key"`
	MinIOBucket           string `json:"minio_bucket"`
	MinIOUseSSL           bool   `json:"minio_use_ssl"`
}

func main() {
	configPath := flag.String("config", "deployments/configs/meeting-service.json", "path to config JSON file")
	flag.Parse()

	cfg, err := config.Load[serviceConfig](*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}

	log := logger.New("meeting-service", logger.ParseLevel(cfg.LogLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := dbx.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect to postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := dbx.RunMigrations(ctx, pool, "meeting", meetingmigrations.FS, "."); err != nil {
		log.Error("run migrations", "err", err)
		os.Exit(1)
	}

	// See storage/minio's doc comment for why the internal/public
	// endpoints are two different values.
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

	repo := meetingpg.NewMeetingRepository(pool)
	handler := NewHandler(
		usecase.NewCreateUploadIntentUseCase(repo, storage),
		usecase.NewConfirmUploadUseCase(repo, storage),
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
