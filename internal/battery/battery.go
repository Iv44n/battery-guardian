// Package battery lee el estado de la batería desde /sys/class/power_supply.
// Se adapta a equipos que reportan en charge_* (µAh) o energy_* (µWh).
package battery

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const sysPower = "/sys/class/power_supply"

// Sample es una lectura puntual del estado de la batería.
type Sample struct {
	Name        string
	CapacityPct int
	Status      string // Charging, Discharging, Full, Not charging, Unknown
	OnAC        bool
	VoltageV    float64
	CurrentA    float64 // valor absoluto
	PowerW      float64
	HealthPct   float64 // full / design * 100
	CycleCount  int

	// internos (micro-unidades) para estimar tiempos restantes
	nowQ  float64 // charge_now (µAh) o energy_now (µWh)
	fullQ float64 // charge_full o energy_full
	rateQ float64 // current_now (µA) o power_now (µW)
}

// Monitor guarda las rutas detectadas de batería y adaptador.
type Monitor struct {
	batPath string
	acPath  string // .../online
}

func readInt(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, false
	}
	return v, true
}

func readStr(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}

// Detect localiza la primera batería y el adaptador de corriente.
func Detect() (*Monitor, error) {
	entries, err := os.ReadDir(sysPower)
	if err != nil {
		return nil, err
	}
	m := &Monitor{}
	for _, e := range entries {
		p := filepath.Join(sysPower, e.Name())
		typ, _ := readStr(filepath.Join(p, "type"))
		switch typ {
		case "Battery":
			if m.batPath == "" {
				m.batPath = p
			}
		case "Mains":
			if m.acPath == "" {
				m.acPath = filepath.Join(p, "online")
			}
		}
	}
	if m.batPath == "" {
		return nil, fmt.Errorf("no se encontró ninguna batería en %s", sysPower)
	}
	return m, nil
}

// BatteryName devuelve el nombre del dispositivo (p.ej. BAT1).
func (m *Monitor) BatteryName() string { return filepath.Base(m.batPath) }

// Read toma una lectura del estado actual.
func (m *Monitor) Read() Sample {
	p := m.batPath
	s := Sample{Name: filepath.Base(p)}

	if v, ok := readInt(filepath.Join(p, "capacity")); ok {
		s.CapacityPct = v
	}
	if v, ok := readStr(filepath.Join(p, "status")); ok {
		s.Status = v
	}
	if v, ok := readInt(filepath.Join(p, "voltage_now")); ok {
		s.VoltageV = float64(v) / 1e6
	}
	if v, ok := readInt(filepath.Join(p, "current_now")); ok {
		a := float64(v) / 1e6
		if a < 0 {
			a = -a
		}
		s.CurrentA = a
	}
	if v, ok := readInt(filepath.Join(p, "cycle_count")); ok {
		s.CycleCount = v
	}

	// Adaptador de corriente.
	if m.acPath != "" {
		if v, ok := readInt(m.acPath); ok {
			s.OnAC = v == 1
		}
	} else {
		s.OnAC = s.Status == "Charging" || s.Status == "Full"
	}

	// charge_* (µAh) — caso de este IdeaPad — o energy_* (µWh).
	if now, ok := readInt(filepath.Join(p, "charge_now")); ok {
		s.nowQ = float64(now)
		if full, ok := readInt(filepath.Join(p, "charge_full")); ok {
			s.fullQ = float64(full)
		}
		s.rateQ = s.CurrentA * 1e6 // current_now ya leído arriba, en valor absoluto
		if des, ok := readInt(filepath.Join(p, "charge_full_design")); ok && des > 0 {
			s.HealthPct = s.fullQ / float64(des) * 100
		}
		s.PowerW = s.VoltageV * s.CurrentA
	} else if now, ok := readInt(filepath.Join(p, "energy_now")); ok {
		s.nowQ = float64(now)
		if full, ok := readInt(filepath.Join(p, "energy_full")); ok {
			s.fullQ = float64(full)
		}
		if pw, ok := readInt(filepath.Join(p, "power_now")); ok {
			r := float64(pw)
			if r < 0 {
				r = -r
			}
			s.rateQ = r
			s.PowerW = r / 1e6
		} else {
			s.PowerW = s.VoltageV * s.CurrentA
			s.rateQ = s.PowerW * 1e6
		}
		if des, ok := readInt(filepath.Join(p, "energy_full_design")); ok && des > 0 {
			s.HealthPct = s.fullQ / float64(des) * 100
		}
	}
	return s
}

// RemainingToFull estima el tiempo hasta carga completa (0 si se desconoce).
func (s Sample) RemainingToFull() time.Duration {
	if s.rateQ <= 0 || s.fullQ <= 0 || s.nowQ >= s.fullQ {
		return 0
	}
	hours := (s.fullQ - s.nowQ) / s.rateQ
	return time.Duration(hours * float64(time.Hour))
}

// RemainingToEmpty estima la autonomía restante (0 si se desconoce).
func (s Sample) RemainingToEmpty() time.Duration {
	if s.rateQ <= 0 || s.nowQ <= 0 {
		return 0
	}
	hours := s.nowQ / s.rateQ
	return time.Duration(hours * float64(time.Hour))
}
