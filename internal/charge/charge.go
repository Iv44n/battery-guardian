// Package charge abstrae el control de carga del fabricante detrás de una
// interfaz común (ChargeController). Cada equipo implementa el método que
// expone su hardware; el resto del daemon es idéntico para todos.
package charge

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Capabilities describe qué método de control de carga ofrece el equipo.
type Capabilities struct {
	Vendor       string
	Method       string
	Configurable bool // ¿se puede fijar un umbral arbitrario (p.ej. 80)?
	ApproxLimit  int  // tope aproximado al activar protección (p.ej. 60 en IdeaPad)
	Path         string
}

// Controller (ChargeController) es la abstracción multi-fabricante.
type Controller interface {
	Capabilities() Capabilities
	EnableProtection(limit int) error
	DisableProtection() error
	IsProtected() (bool, error)
}

func writeFile(path, val string) error { return os.WriteFile(path, []byte(val), 0644) }

func readTrim(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// ---------- IdeaPad: conservation_mode (on/off, tope ~60%) ----------

type ideapad struct{ path string }

func (i *ideapad) Capabilities() Capabilities {
	return Capabilities{
		Vendor: "Lenovo IdeaPad", Method: "conservation_mode",
		Configurable: false, ApproxLimit: 60, Path: i.path,
	}
}
func (i *ideapad) EnableProtection(_ int) error { return writeFile(i.path, "1") }
func (i *ideapad) DisableProtection() error     { return writeFile(i.path, "0") }
func (i *ideapad) IsProtected() (bool, error) {
	v, err := readTrim(i.path)
	if err != nil {
		return false, err
	}
	return v == "1", nil
}

// ---------- Genérico: charge_control_end_threshold (ThinkPad, etc.) ----------

type threshold struct{ endPath, startPath string }

func (t *threshold) Capabilities() Capabilities {
	return Capabilities{
		Vendor: "Genérico (charge_control)", Method: "charge_control_end_threshold",
		Configurable: true, ApproxLimit: 0, Path: t.endPath,
	}
}
func (t *threshold) EnableProtection(limit int) error {
	if limit <= 0 || limit > 100 {
		limit = 80
	}
	if t.startPath != "" {
		_ = writeFile(t.startPath, strconv.Itoa(max(limit-5, 0)))
	}
	return writeFile(t.endPath, strconv.Itoa(limit))
}
func (t *threshold) DisableProtection() error {
	if t.startPath != "" {
		_ = writeFile(t.startPath, "0")
	}
	return writeFile(t.endPath, "100")
}
func (t *threshold) IsProtected() (bool, error) {
	v, err := readTrim(t.endPath)
	if err != nil {
		return false, err
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return false, err
	}
	return n < 100, nil
}

// ---------- Detección automática ----------

func firstGlob(patterns ...string) string {
	for _, p := range patterns {
		matches, _ := filepath.Glob(p)
		for _, m := range matches {
			if _, err := os.Stat(m); err == nil {
				return m
			}
		}
	}
	return ""
}

// findConservation recorre /sys/devices buscando conservation_mode (fallback).
func findConservation() string {
	var found string
	_ = filepath.WalkDir("/sys/devices", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if found != "" {
			return filepath.SkipAll
		}
		if !d.IsDir() && d.Name() == "conservation_mode" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// Detect elige el controlador según el hardware presente.
// Prefiere umbrales configurables; si no, modo conservación IdeaPad.
func Detect() (Controller, error) {
	if end := firstGlob("/sys/class/power_supply/BAT*/charge_control_end_threshold"); end != "" {
		start := strings.Replace(end, "charge_control_end_threshold", "charge_control_start_threshold", 1)
		if _, err := os.Stat(start); err != nil {
			start = ""
		}
		return &threshold{endPath: end, startPath: start}, nil
	}

	cm := firstGlob(
		"/sys/bus/platform/devices/VPC*/conservation_mode",
		"/sys/bus/platform/drivers/ideapad_acpi/VPC*/conservation_mode",
		"/sys/devices/platform/VPC*/conservation_mode",
	)
	if cm == "" {
		cm = findConservation()
	}
	if cm != "" {
		return &ideapad{path: cm}, nil
	}
	return nil, fmt.Errorf("no se detectó ningún método de control de carga soportado")
}
