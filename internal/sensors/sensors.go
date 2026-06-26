// Package sensors lee temperaturas desde /sys/class/hwmon.
package sensors

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Temps agrupa las temperaturas relevantes (en °C).
type Temps struct {
	BatteryC   float64 // sensor propio de la batería (no todos los equipos lo tienen)
	HasBattery bool
	AmbientC   float64 // acpitz
	HasAmbient bool
	CPUC       float64 // k10temp / coretemp
	HasCPU     bool
	GPUC       float64 // amdgpu / nouveau / nvidia
	HasGPU     bool
}

func readMilliTemp(path string) (float64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, false
	}
	return float64(v) / 1000.0, true
}

// maxTempInput devuelve la mayor tempN_input de un directorio hwmon.
func maxTempInput(dir string) (float64, bool) {
	matches, _ := filepath.Glob(filepath.Join(dir, "temp*_input"))
	best := 0.0
	found := false
	for _, m := range matches {
		if t, ok := readMilliTemp(m); ok {
			if !found || t > best {
				best = t
				found = true
			}
		}
	}
	return best, found
}

// Read recorre todos los hwmon y clasifica las temperaturas.
func Read() Temps {
	var out Temps
	dirs, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, d := range dirs {
		nameB, err := os.ReadFile(filepath.Join(d, "name"))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(nameB))
		t, ok := maxTempInput(d)
		if !ok {
			continue
		}
		switch {
		case strings.HasPrefix(name, "BAT"):
			out.BatteryC, out.HasBattery = t, true
		case name == "acpitz":
			out.AmbientC, out.HasAmbient = t, true
		case name == "k10temp" || name == "coretemp":
			out.CPUC, out.HasCPU = t, true
		case name == "amdgpu" || name == "nouveau" || name == "nvidia":
			out.GPUC, out.HasGPU = t, true
		}
	}
	return out
}

// BatteryTemp devuelve la temperatura propia de la batería, si el equipo la
// expone. Es la única apta para alarmas con umbrales tipo 42/48 °C.
func (t Temps) BatteryTemp() (float64, bool) {
	if t.HasBattery {
		return t.BatteryC, true
	}
	return 0, false
}

// SystemTemp devuelve la temperatura más alta del sistema (CPU/GPU/chip),
// solo informativa: corre mucho más caliente que la batería.
func (t Temps) SystemTemp() (float64, bool) {
	best, found := 0.0, false
	for _, c := range []struct {
		v  float64
		ok bool
	}{{t.CPUC, t.HasCPU}, {t.GPUC, t.HasGPU}, {t.AmbientC, t.HasAmbient}} {
		if c.ok && (!found || c.v > best) {
			best, found = c.v, true
		}
	}
	return best, found
}
