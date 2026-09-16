// Package config builds the API's typed configuration from environment variables
// (DECISIONS.md D-43) and provides the small .env loader the bootstrap uses.
package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Environment names accepted in APP_ENV.
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

// PlaceholderJWTSecret is the value shipped in .env.example. It is refused in production.
const PlaceholderJWTSecret = "change-me-to-a-random-32+-char-string"

// MinJWTSecretLen is the minimum accepted length of JWT_SECRET (D-55).
const MinJWTSecretLen = 32

// DefaultPort is used when PORT is unset or blank.
const DefaultPort = 3000

// Config is the fully validated runtime configuration.
type Config struct {
	Port         int
	AppEnv       string
	JWTSecret    string
	CookieSecure bool
}

// Lookup returns the value of an environment variable and whether it was set.
// os.LookupEnv satisfies it; tests pass a map-backed function.
type Lookup func(key string) (string, bool)

// MapLookup adapts a map to a Lookup — handy for tests.
func MapLookup(m map[string]string) Lookup {
	return func(key string) (string, bool) {
		v, ok := m[key]
		return v, ok
	}
}

// Load builds a Config from the given lookup, applying defaults and validating every value.
// All problems are reported together so a misconfigured .env is fixed in one pass.
func Load(lookup Lookup) (Config, error) {
	var errs []error
	cfg := Config{Port: DefaultPort, AppEnv: EnvDevelopment}

	if raw, ok := nonEmpty(lookup, "PORT"); ok {
		port, err := strconv.Atoi(raw)
		if err != nil || port < 1 || port > 65535 {
			errs = append(errs, fmt.Errorf("PORT must be an integer between 1 and 65535, got %q", raw))
		} else {
			cfg.Port = port
		}
	}

	if raw, ok := nonEmpty(lookup, "APP_ENV"); ok {
		switch raw {
		case EnvDevelopment, EnvTest, EnvProduction:
			cfg.AppEnv = raw
		default:
			errs = append(errs, fmt.Errorf("APP_ENV must be one of development, test, production; got %q", raw))
		}
	}

	if raw, ok := nonEmpty(lookup, "JWT_SECRET"); !ok {
		errs = append(errs, errors.New("JWT_SECRET is required"))
	} else if len(raw) < MinJWTSecretLen {
		errs = append(errs, fmt.Errorf("JWT_SECRET must be at least %d characters", MinJWTSecretLen))
	} else if raw == PlaceholderJWTSecret && cfg.AppEnv == EnvProduction {
		errs = append(errs, errors.New("JWT_SECRET is still the .env.example placeholder; set a real secret in production"))
	} else {
		cfg.JWTSecret = raw
	}

	if raw, ok := nonEmpty(lookup, "COOKIE_SECURE"); ok {
		secure, err := strconv.ParseBool(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("COOKIE_SECURE must be true or false, got %q", raw))
		} else {
			cfg.CookieSecure = secure
		}
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
	}
	return cfg, nil
}

// IsTest reports whether the API runs under APP_ENV=test (lower bcrypt cost, no rate limiting, silent logs).
func (c Config) IsTest() bool { return c.AppEnv == EnvTest }

// IsProduction reports whether the API runs under APP_ENV=production.
func (c Config) IsProduction() bool { return c.AppEnv == EnvProduction }

// nonEmpty looks a key up and treats a blank value as unset, so `PORT=` in .env means "use the default".
func nonEmpty(lookup Lookup, key string) (string, bool) {
	v, ok := lookup(key)
	v = strings.TrimSpace(v)
	return v, ok && v != ""
}
