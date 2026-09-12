// Command auth-service is the Auth Service binary — see
// docs/architecture/microservices.md §2.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	authmigrations "github.com/shivamrajput177/ai-meeting-intelligence/migrations/auth"

	authclient "github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/client"
	authhttp "github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/delivery/http"
	authpg "github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/repository/postgres"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/logger"
)

func main() {
	log := logger.New("auth-service")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := dbx.NewPool(ctx, config.Env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/meetingintel"))
	if err != nil {
		log.Error("connect to postgres", slog.Any("err", err))
		os.Exit(1)
	}
	defer pool.Close()

	if err := dbx.RunMigrations(ctx, pool, "auth", authmigrations.FS, "."); err != nil {
		log.Error("run migrations", slog.Any("err", err))
		os.Exit(1)
	}

	internalToken := config.Env("INTERNAL_SERVICE_TOKEN", "dev-internal-token")
	orgClient := authclient.NewOrgClient(config.Env("ORG_SERVICE_URL", "http://localhost:8082"), internalToken)
	userClient := authclient.NewUserClient(config.Env("USER_SERVICE_URL", "http://localhost:8081"), internalToken)

	credentialsRepo := authpg.NewCredentialsRepository(pool)
	refreshRepo := authpg.NewRefreshTokenRepository(pool)
	resetRepo := authpg.NewPasswordResetRepository(pool)

	jwtSecret := []byte(config.Env("JWT_SIGNING_KEY", "dev-only-signing-key-change-me"))
	accessTTL := config.EnvDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	refreshTTL := config.EnvDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour)
	tokenIssuer := usecase.NewTokenIssuer(jwtSecret, accessTTL, refreshTTL, refreshRepo)

	handler := authhttp.NewHandler(
		usecase.NewSignupUseCase(orgClient, userClient, credentialsRepo, tokenIssuer),
		usecase.NewLoginUseCase(userClient, credentialsRepo, tokenIssuer),
		usecase.NewRefreshUseCase(refreshRepo, tokenIssuer),
		usecase.NewLogoutUseCase(refreshRepo),
		usecase.NewRequestPasswordResetUseCase(userClient, resetRepo, log, config.EnvBool("AUTH_DEV_EXPOSE_RESET_TOKEN", false)),
		usecase.NewConfirmPasswordResetUseCase(resetRepo, credentialsRepo),
	)

	app := httpserver.New("auth-service", log)
	authhttp.RegisterRoutes(app, handler)

	addr := ":" + config.Env("PORT", "8080")
	log.Info("starting", slog.String("addr", addr))
	if err := app.Listen(addr); err != nil {
		log.Error("server stopped", slog.Any("err", err))
		os.Exit(1)
	}
}
