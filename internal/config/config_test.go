package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, yaml string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadRejectsNonPositivePollInterval(t *testing.T) {
	path := writeConfig(t, "poll_interval: 0s\nbattery:\n  upper_limit: 80\n  lower_limit: 65\n")

	cfg, err := Load(path)
	if err == nil {
		t.Fatalf("expected error for non-positive poll_interval, got nil")
	}
	if !strings.Contains(err.Error(), "poll_interval") {
		t.Fatalf("expected poll_interval validation error, got %v", err)
	}
	if cfg.PollInterval <= 0 {
		t.Fatalf("expected default poll interval when config is invalid, got %v", cfg.PollInterval)
	}
}

func TestLoadRejectsOutOfRangeValues(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string // subcadena esperada en el error
	}{
		{"poll_interval demasiado corto", "poll_interval: 100ms\n", "poll_interval"},
		{"upper_limit fuera de rango", "battery:\n  upper_limit: 150\n  lower_limit: 65\n", "upper_limit"},
		{"lower_limit negativo", "battery:\n  upper_limit: 80\n  lower_limit: -5\n", "lower_limit"},
		{"lower_limit >= upper_limit", "battery:\n  upper_limit: 60\n  lower_limit: 70\n", "lower_limit"},
		{"critical <= warning", "temperature:\n  warning: 50\n  critical: 45\n", "temperature.critical"},
		{"health.warning fuera de rango", "health:\n  warning: 150\n", "health.warning"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, tc.yaml))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error mentioning %q, got %v", tc.want, err)
			}
			if cfg != Default() {
				t.Fatalf("expected default config on invalid input, got %+v", cfg)
			}
		})
	}
}

func TestLoadParsesValidConfig(t *testing.T) {
	yaml := "poll_interval: 30s\n" +
		"battery:\n  upper_limit: 75\n  lower_limit: 50\n" +
		"notifications:\n  enabled: false\n" +
		"temperature:\n  warning: 40\n  critical: 45\n" +
		"health:\n  warning: 70\n" +
		"logging:\n  enabled: false\n"

	cfg, err := Load(writeConfig(t, yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Config{
		PollInterval: 30 * time.Second, UpperLimit: 75, LowerLimit: 50,
		NotifyEnabled: false, TempWarning: 40, TempCritical: 45,
		HealthWarning: 70, LogEnabled: false,
	}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}
