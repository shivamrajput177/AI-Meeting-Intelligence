// Command user-service is the User Service binary — see
// docs/architecture/microservices.md §3.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	usermigrations "github.com/shivamrajput177/ai-meeting-intelligence/migrations/user"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/logger"
	userhttp "github.com/shivamrajput177/ai-meeting-intelligence/internal/usersvc/delivery/http"
	userpg "github.com/shivamrajput177/ai-meeting-intelligence/internal/usersvc/repository/postgres"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/usersvc/usecase"
)

func main() {
	log := logger.New("user-service", logger.ParseLevel(config.Env("LOG_LEVEL", "info")))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := dbx.NewPool(ctx, config.Env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/meetingintel"))
	if err != nil {
		log.Error("connect to postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := dbx.RunMigrations(ctx, pool, "user", usermigrations.FS, "."); err != nil {
		log.Error("run migrations", "err", err)
		os.Exit(1)
	}

	repo := userpg.NewUserRepository(pool)
	handler := userhttp.NewHandler(
		usecase.NewCreateUserUseCase(repo),
		usecase.NewGetUserUseCase(repo),
		usecase.NewUpdateProfileUseCase(repo),
		usecase.NewLookupByEmailUseCase(repo),
	)

	srv := httpserver.New("user-service", log)
	userhttp.RegisterRoutes(srv.Mux, handler, config.Env("INTERNAL_SERVICE_TOKEN", "dev-internal-token"))

	addr := ":" + config.Env("PORT", "8081")
	log.Info("starting", "addr", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
