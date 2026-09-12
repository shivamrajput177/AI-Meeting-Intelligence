// Command organization-service is the Organization Service binary — see
// docs/architecture/microservices.md §4.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	orgmigrations "github.com/shivamrajput177/ai-meeting-intelligence/migrations/org"

	orghttp "github.com/shivamrajput177/ai-meeting-intelligence/internal/orgsvc/delivery/http"
	orgpg "github.com/shivamrajput177/ai-meeting-intelligence/internal/orgsvc/repository/postgres"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/orgsvc/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/logger"
)

func main() {
	log := logger.New("organization-service")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := dbx.NewPool(ctx, config.Env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/meetingintel"))
	if err != nil {
		log.Error("connect to postgres", slog.Any("err", err))
		os.Exit(1)
	}
	defer pool.Close()

	if err := dbx.RunMigrations(ctx, pool, "org", orgmigrations.FS, "."); err != nil {
		log.Error("run migrations", slog.Any("err", err))
		os.Exit(1)
	}

	repo := orgpg.NewOrgRepository(pool)
	handler := orghttp.NewHandler(
		usecase.NewCreateOrgUseCase(repo),
		usecase.NewGetOrgUseCase(repo),
	)

	app := httpserver.New("organization-service", log)
	orghttp.RegisterRoutes(app, handler, config.Env("INTERNAL_SERVICE_TOKEN", "dev-internal-token"))

	addr := ":" + config.Env("PORT", "8082")
	log.Info("starting", slog.String("addr", addr))
	if err := app.Listen(addr); err != nil {
		log.Error("server stopped", slog.Any("err", err))
		os.Exit(1)
	}
}
