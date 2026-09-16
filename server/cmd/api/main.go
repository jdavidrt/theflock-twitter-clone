// Command api is the HTTP API server bootstrap: it loads the root .env, builds the
// configuration, wires the handler and serves it with graceful shutdown. Per D-53 this is
// the only place that reads the process environment.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/config"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/httpapi"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/memory"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/sample"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	loaded, err := config.LoadDotenv(config.DotenvCandidates...)
	if err != nil {
		return err
	}
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	logger := newLogger(cfg)
	if loaded != "" {
		logger.Info("loaded environment file", "path", loaded)
	}

	if err := loadStore(cfg, logger); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           httpapi.NewHandler(httpapi.Deps{Config: cfg, Logger: logger}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", srv.Addr, "env", cfg.AppEnv)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// loadStore builds the configured store and loads the sample data into it, logging what was
// loaded (D-66). The store built here is not yet wired into the handler — that lands with
// the first service in Step 3; today this only proves boot-time loading works.
func loadStore(cfg config.Config, logger *slog.Logger) error {
	switch cfg.Store {
	case config.StoreMemory:
		st := memory.New()
		counts, err := sample.Load(cfg.SampleDataPath, cfg.BcryptCost(), st)
		if err != nil {
			return fmt.Errorf("loading sample data from %s: %w", cfg.SampleDataPath, err)
		}
		logger.Info("loaded sample data",
			"store", cfg.Store,
			"users", counts.Users,
			"tweets", counts.Tweets,
			"follows", counts.Follows,
			"likes", counts.Likes,
		)
		return nil
	default:
		return fmt.Errorf("unsupported STORE %q", cfg.Store)
	}
}

// newLogger follows D-57: text in development, JSON in production, silent in test.
func newLogger(cfg config.Config) *slog.Logger {
	switch {
	case cfg.IsTest():
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	case cfg.IsProduction():
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	default:
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
}
