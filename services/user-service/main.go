// Command user-service is the User Service binary — see
// docs/architecture/microservices.md §3.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	usermigrations "github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/migrations"

	userpg "github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/repository/postgres"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// serviceConfig is user-service's whole configuration surface — see
// configs/user-service.template.json for the shape and dev-safe defaults.
type serviceConfig struct {
	Port                 string `json:"port"`
	LogLevel             string `json:"log_level"`
	DatabaseURL          string `json:"database_url"`
	InternalServiceToken string `json:"internal_service_token"`
}

func main() {
	cfg := loadConfig()
	log := logger.New("user-service", logger.ParseLevel(cfg.LogLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool := initPostgres(ctx, cfg, log)
	defer pool.Close()

	repo := userpg.NewUserRepository(pool)
	handler := NewHandler(
		usecase.NewCreateUserUseCase(repo),
		usecase.NewGetUserUseCase(repo),
		usecase.NewUpdateProfileUseCase(repo),
		usecase.NewLookupByEmailUseCase(repo),
	)

	srv := httpserver.New("user-service", log)
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
	configPath := flag.String("config", "deployments/configs/user-service.json", "path to config JSON file")
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
	if err := dbx.RunMigrations(ctx, pool, "user", usermigrations.FS, "."); err != nil {
		log.Error("run migrations", "err", err)
		os.Exit(1)
	}
	return pool
}
