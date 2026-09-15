// Command auth-service is the Auth Service binary — see
// docs/architecture/microservices.md §2.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	authmigrations "github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/migrations"

	authclient "github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/client/http"
	authpg "github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/repository/postgres"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// serviceConfig is auth-service's whole configuration surface — see
// configs/auth-service.template.json for the shape and dev-safe defaults.
type serviceConfig struct {
	Port                    string `json:"port"`
	LogLevel                string `json:"log_level"`
	DatabaseURL             string `json:"database_url"`
	InternalServiceToken    string `json:"internal_service_token"`
	OrgServiceURL           string `json:"org_service_url"`
	UserServiceURL          string `json:"user_service_url"`
	JWTSigningKey           string `json:"jwt_signing_key"`
	AccessTokenTTL          string `json:"access_token_ttl"`
	RefreshTokenTTL         string `json:"refresh_token_ttl"`
	AuthDevExposeResetToken bool   `json:"auth_dev_expose_reset_token"`
}

func main() {
	cfg := loadConfig()
	log := logger.New("auth-service", logger.ParseLevel(cfg.LogLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool := initPostgres(ctx, cfg, log)
	defer pool.Close()

	orgClient, userClient := initClients(cfg)

	credentialsRepo := authpg.NewCredentialsRepository(pool)
	refreshRepo := authpg.NewRefreshTokenRepository(pool)
	resetRepo := authpg.NewPasswordResetRepository(pool)
	tokenIssuer := initTokenIssuer(cfg, refreshRepo)

	handler := NewHandler(
		usecase.NewSignupUseCase(orgClient, userClient, credentialsRepo, tokenIssuer),
		usecase.NewLoginUseCase(userClient, credentialsRepo, tokenIssuer),
		usecase.NewRefreshUseCase(refreshRepo, tokenIssuer),
		usecase.NewLogoutUseCase(refreshRepo),
		usecase.NewRequestPasswordResetUseCase(userClient, resetRepo, log, cfg.AuthDevExposeResetToken),
		usecase.NewConfirmPasswordResetUseCase(resetRepo, credentialsRepo),
	)

	srv := httpserver.New("auth-service", log)
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
	configPath := flag.String("config", "deployments/configs/auth-service.json", "path to config JSON file")
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
	if err := dbx.RunMigrations(ctx, pool, "auth", authmigrations.FS, "."); err != nil {
		log.Error("run migrations", "err", err)
		os.Exit(1)
	}
	return pool
}

// initClients builds the REST clients Auth Service calls through to
// create the org+owner rows during signup — see client.OrgClient/
// UserClient's doc comment for why signup orchestrates two other
// services instead of writing to their tables directly.
func initClients(cfg serviceConfig) (*authclient.OrgClient, *authclient.UserClient) {
	orgClient := authclient.NewOrgClient(cfg.OrgServiceURL, cfg.InternalServiceToken)
	userClient := authclient.NewUserClient(cfg.UserServiceURL, cfg.InternalServiceToken)
	return orgClient, userClient
}

// initTokenIssuer wires the access/refresh TTLs from config into the one
// TokenIssuer signup, login, and refresh all share.
func initTokenIssuer(cfg serviceConfig, refreshRepo *authpg.RefreshTokenRepository) *usecase.TokenIssuer {
	jwtSecret := []byte(cfg.JWTSigningKey)
	accessTTL := config.ParseDuration(cfg.AccessTokenTTL, 15*time.Minute)
	refreshTTL := config.ParseDuration(cfg.RefreshTokenTTL, 7*24*time.Hour)
	return usecase.NewTokenIssuer(jwtSecret, accessTTL, refreshTTL, refreshRepo)
}
