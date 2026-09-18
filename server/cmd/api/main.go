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
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/memory"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/sample"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/sqlite"
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

	st, err := buildStore(cfg, logger)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           httpapi.NewHandler(httpapi.Deps{Config: cfg, Logger: logger, Store: st}),
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

// buildStore builds the configured store and loads the sample data into it, logging what was
// loaded (D-66).
func buildStore(cfg config.Config, logger *slog.Logger) (store.Store, error) {
	switch cfg.Store {
	case config.StoreMemory:
		st := memory.New()
		counts, err := sample.Load(cfg.SampleDataPath, cfg.BcryptCost(), st)
		if err != nil {
			return nil, fmt.Errorf("loading sample data from %s: %w", cfg.SampleDataPath, err)
		}
		logger.Info("loaded sample data",
			"store", cfg.Store,
			"users", counts.Users,
			"tweets", counts.Tweets,
			"follows", counts.Follows,
			"likes", counts.Likes,
		)
		return st, nil
	case config.StoreSQLite:
		return buildSQLiteStore(cfg, logger)
	default:
		return nil, fmt.Errorf("unsupported STORE %q", cfg.Store)
	}
}

// buildSQLiteStore opens (creating on first run) the SQLite file at cfg.SQLitePath and seeds it
// from cfg.SampleDataPath only when the users table is empty, so a fresh clone boots seeded
// while later restarts keep whatever the API has since persisted (D-66).
func buildSQLiteStore(cfg config.Config, logger *slog.Logger) (store.Store, error) {
	st, err := sqlite.Open(cfg.SQLitePath)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite store at %s: %w", cfg.SQLitePath, err)
	}

	empty, err := st.IsEmpty()
	if err != nil {
		return nil, fmt.Errorf("checking sqlite store at %s: %w", cfg.SQLitePath, err)
	}
	if !empty {
		logger.Info("opened sqlite store", "store", cfg.Store, "path", cfg.SQLitePath)
		return st, nil
	}

	counts, err := sample.Load(cfg.SampleDataPath, cfg.BcryptCost(), st)
	if err != nil {
		return nil, fmt.Errorf("seeding sqlite store from %s: %w", cfg.SampleDataPath, err)
	}
	logger.Info("seeded empty sqlite store",
		"store", cfg.Store,
		"path", cfg.SQLitePath,
		"users", counts.Users,
		"tweets", counts.Tweets,
		"follows", counts.Follows,
		"likes", counts.Likes,
	)
	return st, nil
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
