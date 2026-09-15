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

	usermigrations "github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/migrations"

	userhttp "github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/handler"
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
	configPath := flag.String("config", "deployments/configs/user-service.json", "path to config JSON file")
	flag.Parse()

	cfg, err := config.Load[serviceConfig](*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}

	log := logger.New("user-service", logger.ParseLevel(cfg.LogLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := dbx.NewPool(ctx, cfg.DatabaseURL)
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
	userhttp.RegisterRoutes(srv.Mux, handler, cfg.InternalServiceToken)

	addr := ":" + cfg.Port
	log.Info("starting", "addr", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
