// Command api-gateway is the single public entry point — see
// docs/architecture/microservices.md §1.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/api-gateway/internal/handler"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/redisx"
)

// serviceConfig is api-gateway's whole configuration surface — see
// configs/api-gateway.template.json for the shape and dev-safe defaults.
type serviceConfig struct {
	Port              string `json:"port"`
	LogLevel          string `json:"log_level"`
	JWTSigningKey     string `json:"jwt_signing_key"`
	RedisAddr         string `json:"redis_addr"`
	AuthServiceURL    string `json:"auth_service_url"`
	UserServiceURL    string `json:"user_service_url"`
	OrgServiceURL     string `json:"org_service_url"`
	MeetingServiceURL string `json:"meeting_service_url"`
}

func main() {
	configPath := flag.String("config", "deployments/configs/api-gateway.json", "path to config JSON file")
	flag.Parse()

	cfg, err := config.Load[serviceConfig](*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}

	log := logger.New("api-gateway", logger.ParseLevel(cfg.LogLevel))

	rdb := initRedis(cfg)
	jwtSecret := []byte(cfg.JWTSigningKey)

	srv := httpserver.New("api-gateway", log)
	handler.Register(srv.Mux, initServiceURLs(cfg), jwtSecret, rdb, log)

	addr := ":" + cfg.Port
	log.Info("starting", "addr", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func initRedis(cfg serviceConfig) *redis.Client {
	return redisx.NewClient(cfg.RedisAddr)
}

func initServiceURLs(cfg serviceConfig) handler.ServiceURLs {
	return handler.ServiceURLs{
		Auth:    cfg.AuthServiceURL,
		User:    cfg.UserServiceURL,
		Org:     cfg.OrgServiceURL,
		Meeting: cfg.MeetingServiceURL,
	}
}
