// Command meeting-service is the Meeting Service binary — see
// docs/architecture/microservices.md §5.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	meetingmigrations "github.com/shivamrajput177/ai-meeting-intelligence/migrations/meeting"

	meetinghttp "github.com/shivamrajput177/ai-meeting-intelligence/internal/meetingsvc/delivery/http"
	meetingpg "github.com/shivamrajput177/ai-meeting-intelligence/internal/meetingsvc/repository/postgres"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/meetingsvc/storage/minio"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/meetingsvc/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/logger"
)

func main() {
	log := logger.New("meeting-service", logger.WithLevel(logger.ParseLevel(config.Env("LOG_LEVEL", "info"))))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := dbx.NewPool(ctx, config.Env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/meetingintel"))
	if err != nil {
		log.Error("connect to postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := dbx.RunMigrations(ctx, pool, "meeting", meetingmigrations.FS, "."); err != nil {
		log.Error("run migrations", "err", err)
		os.Exit(1)
	}

	// See internal/meetingsvc/storage/minio's doc comment for why
	// internal/public are two different endpoints.
	storage, err := minio.New(
		config.Env("MINIO_INTERNAL_ENDPOINT", "localhost:9000"),
		config.Env("MINIO_PUBLIC_ENDPOINT", "localhost:9000"),
		config.Env("MINIO_ACCESS_KEY", "minioadmin"),
		config.Env("MINIO_SECRET_KEY", "minioadmin"),
		config.Env("MINIO_BUCKET", "recordings"),
		config.EnvBool("MINIO_USE_SSL", false),
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
	handler := meetinghttp.NewHandler(
		usecase.NewCreateUploadIntentUseCase(repo, storage),
		usecase.NewConfirmUploadUseCase(repo, storage),
		usecase.NewGetMeetingUseCase(repo),
		usecase.NewListMeetingsUseCase(repo),
		usecase.NewUpdateStatusUseCase(repo),
		usecase.NewDeleteMeetingUseCase(repo, storage),
	)

	srv := httpserver.New("meeting-service", log)
	meetinghttp.RegisterRoutes(srv.Mux, handler)

	addr := ":" + config.Env("PORT", "8083")
	log.Info("starting", "addr", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
