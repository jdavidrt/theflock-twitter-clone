// Command seed truncates the configured SQLite database and refills it from the sample data
// file (D-39, D-67). It refuses to run under APP_ENV=production unless SEED_FORCE=true.
package main

import (
	"fmt"
	"os"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/config"
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
	if _, err := config.LoadDotenv(config.DotenvCandidates...); err != nil {
		return err
	}
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	if cfg.IsProduction() && !cfg.SeedForce {
		return fmt.Errorf("refusing to seed %s under APP_ENV=production without SEED_FORCE=true (D-39)", cfg.SQLitePath)
	}

	st, err := sqlite.Open(cfg.SQLitePath)
	if err != nil {
		return fmt.Errorf("opening sqlite store at %s: %w", cfg.SQLitePath, err)
	}
	defer st.Close()

	if err := st.Truncate(); err != nil {
		return fmt.Errorf("truncating %s: %w", cfg.SQLitePath, err)
	}
	counts, err := sample.Load(cfg.SampleDataPath, cfg.BcryptCost(), st)
	if err != nil {
		return fmt.Errorf("loading sample data from %s: %w", cfg.SampleDataPath, err)
	}
	fmt.Printf("seeded %s: users=%d tweets=%d follows=%d likes=%d\n", cfg.SQLitePath, counts.Users, counts.Tweets, counts.Follows, counts.Likes)
	return nil
}
