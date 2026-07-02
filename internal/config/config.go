// Package config carga la configuración desde un archivo YAML (subconjunto
// simple, sin dependencias externas).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config es la configuración efectiva del daemon.
type Config struct {
	PollInterval  time.Duration
	UpperLimit    int
	LowerLimit    int
	NotifyEnabled bool
	TempWarning   float64
	TempCritical  float64
	HealthWarning float64
	LogEnabled    bool
}

// Default devuelve valores sensatos si falta el archivo o alguna clave.
func Default() Config {
	return Config{
		PollInterval:  20 * time.Second,
		UpperLimit:    80,
		LowerLimit:    65,
		NotifyEnabled: true,
		TempWarning:   42,
		TempCritical:  48,
		HealthWarning: 80,
		LogEnabled:    true,
	}
}

// Load lee y valida la configuración; ante error devuelve Default + el error.
func Load(path string) (Config, error) {
	cfg := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	flat := parse(string(b))

	if v, ok := flat["poll_interval"]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.PollInterval = d
		}
	}
	if cfg.PollInterval < time.Second {
		return Default(), fmt.Errorf("poll_interval (%v) debe ser de al menos 1s", cfg.PollInterval)
	}
	if v, ok := flat["battery.upper_limit"]; ok {
		cfg.UpperLimit = atoiDef(v, cfg.UpperLimit)
	}
	if v, ok := flat["battery.lower_limit"]; ok {
		cfg.LowerLimit = atoiDef(v, cfg.LowerLimit)
	}
	if v, ok := flat["notifications.enabled"]; ok {
		cfg.NotifyEnabled = boolDef(v, cfg.NotifyEnabled)
	}
	if v, ok := flat["temperature.warning"]; ok {
		cfg.TempWarning = floatDef(v, cfg.TempWarning)
	}
	if v, ok := flat["temperature.critical"]; ok {
		cfg.TempCritical = floatDef(v, cfg.TempCritical)
	}
	if v, ok := flat["health.warning"]; ok {
		cfg.HealthWarning = floatDef(v, cfg.HealthWarning)
	}
	if v, ok := flat["logging.enabled"]; ok {
		cfg.LogEnabled = boolDef(v, cfg.LogEnabled)
	}

	if cfg.UpperLimit < 1 || cfg.UpperLimit > 100 {
		return Default(), fmt.Errorf("upper_limit (%d) debe estar entre 1 y 100", cfg.UpperLimit)
	}
	if cfg.LowerLimit < 0 {
		return Default(), fmt.Errorf("lower_limit (%d) no puede ser negativo", cfg.LowerLimit)
	}
	if cfg.LowerLimit >= cfg.UpperLimit {
		return Default(), fmt.Errorf("lower_limit (%d) debe ser menor que upper_limit (%d)", cfg.LowerLimit, cfg.UpperLimit)
	}
	if cfg.TempCritical <= cfg.TempWarning {
		return Default(), fmt.Errorf("temperature.critical (%.0f°C) debe ser mayor que temperature.warning (%.0f°C)", cfg.TempCritical, cfg.TempWarning)
	}
	if cfg.HealthWarning < 0 || cfg.HealthWarning > 100 {
		return Default(), fmt.Errorf("health.warning (%.0f) debe estar entre 0 y 100", cfg.HealthWarning)
	}
	return cfg, nil
}

// parse aplana un YAML simple a un mapa "seccion.clave" -> valor.
func parse(s string) map[string]string {
	out := map[string]string{}
	section := ""
	for raw := range strings.SplitSeq(s, "\n") {
		line := strings.TrimRight(raw, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		content := trimmed
		if idx := strings.Index(content, " #"); idx >= 0 {
			content = strings.TrimSpace(content[:idx])
		}
		rawKey, rawVal, found := strings.Cut(content, ":")
		if !found {
			continue
		}
		key := strings.TrimSpace(rawKey)
		val := strings.Trim(strings.TrimSpace(rawVal), `"'`)

		if val == "" { // encabezado de sección
			if indent == 0 {
				section = key
			}
			continue
		}
		if indent == 0 {
			section = ""
			out[key] = val
		} else if section != "" {
			out[section+"."+key] = val
		} else {
			out[key] = val
		}
	}
	return out
}

func atoiDef(s string, d int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return d
}

func floatDef(s string, d float64) float64 {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	return d
}

func boolDef(s string, d bool) bool {
	switch strings.ToLower(s) {
	case "true", "yes", "on", "1":
		return true
	case "false", "no", "off", "0":
		return false
	}
	return d
}
