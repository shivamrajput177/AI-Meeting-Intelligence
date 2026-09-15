// Command organization-service is the Organization Service binary — see
// docs/architecture/microservices.md §4.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	orgmigrations "github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/migrations"

	orghttp "github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/internal/handler"
	orgpg "github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/internal/repository/postgres"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/internal/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// serviceConfig is organization-service's whole configuration surface —
// see configs/organization-service.template.json for the shape and
// dev-safe defaults.
type serviceConfig struct {
	Port                 string `json:"port"`
	LogLevel             string `json:"log_level"`
	DatabaseURL          string `json:"database_url"`
	InternalServiceToken string `json:"internal_service_token"`
}

func main() {
	configPath := flag.String("config", "deployments/configs/organization-service.json", "path to config JSON file")
	flag.Parse()

	cfg, err := config.Load[serviceConfig](*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}

	log := logger.New("organization-service", logger.ParseLevel(cfg.LogLevel))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := dbx.NewPool(ctx, cfg.DatabaseURL)
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
	orghttp.RegisterRoutes(srv.Mux, handler, cfg.InternalServiceToken)

	addr := ":" + cfg.Port
	log.Info("starting", "addr", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
