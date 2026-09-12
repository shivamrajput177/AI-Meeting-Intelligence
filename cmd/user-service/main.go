// Command user-service is the User Service binary — see
// docs/architecture/microservices.md §3.
package main

import (
	"context"
	"log/slog"
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
	log := logger.New("user-service")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := dbx.NewPool(ctx, config.Env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/meetingintel"))
	if err != nil {
		log.Error("connect to postgres", slog.Any("err", err))
		os.Exit(1)
	}
	defer pool.Close()

	if err := dbx.RunMigrations(ctx, pool, "user", usermigrations.FS, "."); err != nil {
		log.Error("run migrations", slog.Any("err", err))
		os.Exit(1)
	}

	repo := userpg.NewUserRepository(pool)
	handler := userhttp.NewHandler(
		usecase.NewCreateUserUseCase(repo),
		usecase.NewGetUserUseCase(repo),
		usecase.NewUpdateProfileUseCase(repo),
		usecase.NewLookupByEmailUseCase(repo),
	)

	app := httpserver.New("user-service", log)
	userhttp.RegisterRoutes(app, handler, config.Env("INTERNAL_SERVICE_TOKEN", "dev-internal-token"))

	addr := ":" + config.Env("PORT", "8081")
	log.Info("starting", slog.String("addr", addr))
	if err := app.Listen(addr); err != nil {
		log.Error("server stopped", slog.Any("err", err))
		os.Exit(1)
	}
}
