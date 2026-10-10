package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitlab.com/practice/endpoint-monitor/internal/monitor"
)

func main() {
	var (
		configPath  = flag.String("config", "endpoints.json", "path to the endpoints file")
		interval    = flag.Duration("interval", 30*time.Second, "how often endpoints are checked")
		timeout     = flag.Duration("timeout", 5*time.Second, "per-request timeout")
		addr        = flag.String("addr", ":8080", "status server listen address")
		concurrency = flag.Int("concurrency", monitor.DefaultConcurrency, "maximum number of endpoints checked in parallel")
		once        = flag.Bool("once", false, "check all endpoints once and exit non-zero if any is unhealthy")
	)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if *concurrency <= 0 {
		logger.Error("-concurrency must be greater than zero", "concurrency", *concurrency)
		os.Exit(2)
	}

	if *once {
		if err := checkOnce(*configPath, *timeout, *concurrency); err != nil {
			logger.Error("check failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if err := run(logger, *configPath, *interval, *timeout, *concurrency, *addr); err != nil {
		logger.Error("monitor exited with error", "error", err)
		os.Exit(1)
	}
}

func checkOnce(configPath string, timeout time.Duration, concurrency int) error {
	endpoints, err := monitor.LoadEndpoints(configPath)
	if err != nil {
		return err
	}

	// Each request is bounded by timeout via the HTTP client; the context only
	// carries cancellation from signals.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	checker := monitor.NewChecker(timeout, concurrency)
	if f := checker.FirstFailure(ctx, endpoints); f != nil {
		if f.Err != nil {
			return fmt.Errorf("endpoint %s is unhealthy: %w", f.Endpoint.Name, f.Err)
		}
		return fmt.Errorf("endpoint %s is unhealthy: status %d", f.Endpoint.Name, f.StatusCode)
	}

	return nil
}

func run(logger *slog.Logger, configPath string, interval, timeout time.Duration, concurrency int, addr string) error {
	endpoints, err := monitor.LoadEndpoints(configPath)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	m := monitor.NewMonitor(monitor.NewChecker(timeout, concurrency), endpoints, interval, logger)

	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		m.Run(ctx)
	}()
	// On any return path, cancel the monitor and wait for its in-flight run
	// to finish.
	defer func() {
		stop()
		<-monitorDone
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(m.Snapshot()); err != nil {
			logger.Error("encoding status", "error", err)
		}
	})
	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(m.Stats()); err != nil {
			logger.Error("encoding stats", "error", err)
		}
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("status server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}
