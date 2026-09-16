package config

import (
	"strings"
	"testing"
)

const validSecret = "0123456789abcdef0123456789abcdef" // 32 chars

func TestLoadDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := Load(MapLookup(map[string]string{"JWT_SECRET": validSecret}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != DefaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, DefaultPort)
	}
	if cfg.AppEnv != EnvDevelopment {
		t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, EnvDevelopment)
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure = true, want false")
	}
	if cfg.JWTSecret != validSecret {
		t.Errorf("JWTSecret = %q, want %q", cfg.JWTSecret, validSecret)
	}
	if cfg.IsTest() || cfg.IsProduction() {
		t.Error("development config must report neither IsTest nor IsProduction")
	}
	if cfg.Store != StoreMemory {
		t.Errorf("Store = %q, want %q", cfg.Store, StoreMemory)
	}
	if cfg.SampleDataPath != DefaultSampleDataPath {
		t.Errorf("SampleDataPath = %q, want %q", cfg.SampleDataPath, DefaultSampleDataPath)
	}
}

func TestLoadExplicitValues(t *testing.T) {
	t.Parallel()
	cfg, err := Load(MapLookup(map[string]string{
		"PORT":             "8080",
		"APP_ENV":          "test",
		"JWT_SECRET":       validSecret,
		"COOKIE_SECURE":    "true",
		"STORE":            "memory",
		"SAMPLE_DATA_PATH": "./testdata/sample.json",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if !cfg.IsTest() {
		t.Errorf("AppEnv = %q, want test", cfg.AppEnv)
	}
	if !cfg.CookieSecure {
		t.Error("CookieSecure = false, want true")
	}
	if cfg.SampleDataPath != "./testdata/sample.json" {
		t.Errorf("SampleDataPath = %q, want ./testdata/sample.json", cfg.SampleDataPath)
	}
}

func TestLoadBlankValuesMeanDefault(t *testing.T) {
	t.Parallel()
	cfg, err := Load(MapLookup(map[string]string{
		"PORT":             "   ",
		"APP_ENV":          "",
		"COOKIE_SECURE":    "",
		"JWT_SECRET":       validSecret,
		"STORE":            "",
		"SAMPLE_DATA_PATH": "  ",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != DefaultPort || cfg.AppEnv != EnvDevelopment || cfg.CookieSecure {
		t.Errorf("blank values should fall back to defaults, got %+v", cfg)
	}
	if cfg.Store != StoreMemory || cfg.SampleDataPath != DefaultSampleDataPath {
		t.Errorf("blank STORE/SAMPLE_DATA_PATH should fall back to defaults, got %+v", cfg)
	}
}

func TestBcryptCost(t *testing.T) {
	t.Parallel()
	prod, err := Load(MapLookup(map[string]string{"JWT_SECRET": validSecret}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := prod.BcryptCost(); got != 12 {
		t.Errorf("development BcryptCost() = %d, want 12", got)
	}

	test, err := Load(MapLookup(map[string]string{"JWT_SECRET": validSecret, "APP_ENV": "test"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := test.BcryptCost(); got != 4 {
		t.Errorf("test BcryptCost() = %d, want 4", got)
	}
}

func TestLoadPlaceholderSecretAllowedOutsideProduction(t *testing.T) {
	t.Parallel()
	for _, env := range []string{EnvDevelopment, EnvTest} {
		cfg, err := Load(MapLookup(map[string]string{"APP_ENV": env, "JWT_SECRET": PlaceholderJWTSecret}))
		if err != nil {
			t.Errorf("APP_ENV=%s: placeholder secret should be accepted, got %v", env, err)
		}
		if cfg.JWTSecret != PlaceholderJWTSecret {
			t.Errorf("APP_ENV=%s: JWTSecret = %q", env, cfg.JWTSecret)
		}
	}
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		vars map[string]string
		want []string // substrings that must all appear in the error
	}{
		{"missing secret", map[string]string{}, []string{"JWT_SECRET is required"}},
		{"short secret", map[string]string{"JWT_SECRET": "short"}, []string{"at least 32"}},
		{"placeholder in production", map[string]string{"APP_ENV": "production", "JWT_SECRET": PlaceholderJWTSecret}, []string{"placeholder"}},
		{"port not a number", map[string]string{"JWT_SECRET": validSecret, "PORT": "abc"}, []string{"PORT", `"abc"`}},
		{"port zero", map[string]string{"JWT_SECRET": validSecret, "PORT": "0"}, []string{"PORT"}},
		{"port too large", map[string]string{"JWT_SECRET": validSecret, "PORT": "70000"}, []string{"PORT"}},
		{"unknown env", map[string]string{"JWT_SECRET": validSecret, "APP_ENV": "staging"}, []string{"APP_ENV", `"staging"`}},
		{"bad cookie flag", map[string]string{"JWT_SECRET": validSecret, "COOKIE_SECURE": "yes please"}, []string{"COOKIE_SECURE"}},
		{"unknown store", map[string]string{"JWT_SECRET": validSecret, "STORE": "postgres"}, []string{"STORE", `"postgres"`}},
		{"all problems reported together", map[string]string{"PORT": "x", "APP_ENV": "y", "COOKIE_SECURE": "z"}, []string{"PORT", "APP_ENV", "COOKIE_SECURE", "JWT_SECRET"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := Load(MapLookup(tc.vars))
			if err == nil {
				t.Fatalf("expected an error, got config %+v", cfg)
			}
			if cfg != (Config{}) {
				t.Errorf("on error the returned config must be zero, got %+v", cfg)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err.Error(), want)
				}
			}
		})
	}
}
