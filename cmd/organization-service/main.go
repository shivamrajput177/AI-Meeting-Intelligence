// Command organization-service is the Organization Service binary — see
// docs/architecture/microservices.md §4.
package main

import (
	"context"
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
	log := logger.New("organization-service", logger.ParseLevel(config.Env("LOG_LEVEL", "info")))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := dbx.NewPool(ctx, config.Env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/meetingintel"))
	if err != nil {
		log.Error("connect to postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := dbx.RunMigrations(ctx, pool, "org", orgmigrations.FS, "."); err != nil {
		log.Error("run migrations", "err", err)
		os.Exit(1)
	}

	repo := orgpg.NewOrgRepository(pool)
	handler := orghttp.NewHandler(
		usecase.NewCreateOrgUseCase(repo),
		usecase.NewGetOrgUseCase(repo),
	)

	srv := httpserver.New("organization-service", log)
	orghttp.RegisterRoutes(srv.Mux, handler, config.Env("INTERNAL_SERVICE_TOKEN", "dev-internal-token"))

	addr := ":" + config.Env("PORT", "8082")
	log.Info("starting", "addr", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
