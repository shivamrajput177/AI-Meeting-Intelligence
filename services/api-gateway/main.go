// Command api-gateway is the single public entry point — see
// docs/architecture/microservices.md §1.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/config"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/redisx"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/tracing"
)

// serviceConfig is api-gateway's whole configuration surface — see
// configs/api-gateway.template.json for the shape and dev-safe defaults.
type serviceConfig struct {
	Port                    string `json:"port"`
	LogLevel                string `json:"log_level"`
	OtelCollectorEndpoint   string `json:"otel_collector_endpoint"`
	JWTSigningKey           string `json:"jwt_signing_key"`
	RedisAddr               string `json:"redis_addr"`
	AuthServiceURL          string `json:"auth_service_url"`
	UserServiceURL          string `json:"user_service_url"`
	OrgServiceURL           string `json:"org_service_url"`
	MeetingServiceURL       string `json:"meeting_service_url"`
	TranscriptionServiceURL string `json:"transcription_service_url"`
	AISummaryServiceURL     string `json:"ai_summary_service_url"`
	ActionItemServiceURL    string `json:"action_item_service_url"`
	SearchServiceURL        string `json:"search_service_url"`
	AnalyticsServiceURL     string `json:"analytics_service_url"`
	NotificationServiceURL  string `json:"notification_service_url"`
}

func main() {
	cfg := loadConfig()
	log := logger.New("api-gateway", logger.ParseLevel(cfg.LogLevel))
	shutdownTracing, err := tracing.Init(context.Background(), "api-gateway", cfg.OtelCollectorEndpoint)
	if err != nil {
		log.Error("init tracing", "err", err)
	} else {
		defer func() { _ = shutdownTracing(context.Background()) }()
	}

	rdb := initRedis(cfg)
	jwtSecret := []byte(cfg.JWTSigningKey)

	srv := httpserver.New("api-gateway", log)
	Register(srv.Mux, initServiceURLs(cfg), jwtSecret, rdb, log)

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
	configPath := flag.String("config", "deployments/configs/api-gateway.json", "path to config JSON file")
	secretsPath := flag.String("secrets", "", "optional path to a second JSON file overlaid onto -config (e.g. a Kubernetes Secret-mounted file holding credentials) — see shared/config.LoadMerged")
	flag.Parse()

	cfg, err := config.LoadMerged[serviceConfig](*configPath, *secretsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}
	return cfg
}

func initRedis(cfg serviceConfig) *redis.Client {
	return redisx.NewClient(cfg.RedisAddr)
}

func initServiceURLs(cfg serviceConfig) ServiceURLs {
	return ServiceURLs{
		Auth:          cfg.AuthServiceURL,
		User:          cfg.UserServiceURL,
		Org:           cfg.OrgServiceURL,
		Meeting:       cfg.MeetingServiceURL,
		Transcription: cfg.TranscriptionServiceURL,
		AISummary:     cfg.AISummaryServiceURL,
		ActionItem:    cfg.ActionItemServiceURL,
		Search:        cfg.SearchServiceURL,
		Analytics:     cfg.AnalyticsServiceURL,
		Notification:  cfg.NotificationServiceURL,
	}
}
