// Package rules implementa la máquina de estados con histéresis y las reglas
// (protección, temperatura, batería baja, salud). No usa IA: solo umbrales.
package rules

import (
	"fmt"
	"time"

	"battery-guardian/internal/battery"
	"battery-guardian/internal/notifier"
	"battery-guardian/internal/sensors"
)

// Config son los parámetros de las reglas.
type Config struct {
	Upper, Lower  int
	TempWarning   float64
	TempCritical  float64
	HealthWarning float64
	ApproxLimit   int
	Configurable  bool
}

// Event es una notificación a emitir.
type Event struct {
	Level Level
	Title string
	Body  string
}

// Level reexporta los niveles del notifier para no acoplar al daemon.
type Level = notifier.Level

const (
	Info     = notifier.Info
	Warning  = notifier.Warning
	Critical = notifier.Critical
)

// Decision es el resultado de evaluar las reglas en un tick.
type Decision struct {
	SetProtection *bool // nil = sin cambio
	Events        []Event
}

// Engine mantiene el estado entre evaluaciones (para no repetir avisos).
type Engine struct {
	cfg            Config
	tempWarned     bool
	tempCritWarned bool
	lowBattWarned  bool
	lastHealthWarn time.Time
}

func NewEngine(cfg Config) *Engine { return &Engine{cfg: cfg} }

// Evaluate aplica todas las reglas y devuelve la decisión.
func (e *Engine) Evaluate(s battery.Sample, t sensors.Temps, protected bool, now time.Time) Decision {
	var d Decision

	// --- Protección por histéresis ---
	switch {
	case s.OnAC && s.CapacityPct >= e.cfg.Upper && !protected:
		d.SetProtection = new(true)
		limTxt := fmt.Sprintf("~%d%%", e.cfg.ApproxLimit)
		if e.cfg.Configurable {
			limTxt = fmt.Sprintf("%d%%", e.cfg.Upper)
		}
		d.Events = append(d.Events, Event{Info, "🔋 Battery Guardian",
			fmt.Sprintf("La batería llegó al %d%%. Protección activada (tope %s).", s.CapacityPct, limTxt)})
	case s.CapacityPct <= e.cfg.Lower && protected:
		d.SetProtection = new(false)
		d.Events = append(d.Events, Event{Info, "🔋 Battery Guardian",
			fmt.Sprintf("La batería bajó al %d%%. Se reanudó la carga.", s.CapacityPct)})
	}

	// --- Temperatura (solo si hay sensor PROPIO de batería; si no, no se
	// alarma para evitar falsos positivos con la temperatura del SoC) ---
	if tp, ok := t.BatteryTemp(); ok {
		switch {
		case tp >= e.cfg.TempCritical:
			if !e.tempCritWarned {
				e.tempCritWarned, e.tempWarned = true, true
				d.Events = append(d.Events, Event{Critical, "🌡 Temperatura crítica",
					fmt.Sprintf("%.0f°C — considera reducir la carga de trabajo.", tp)})
			}
		case tp >= e.cfg.TempWarning:
			if !e.tempWarned {
				e.tempWarned = true
				d.Events = append(d.Events, Event{Warning, "🌡 Temperatura alta",
					fmt.Sprintf("%.0f°C", tp)})
			}
		default:
			if tp < e.cfg.TempWarning-2 { // histéresis para rearmar el aviso
				e.tempWarned, e.tempCritWarned = false, false
			}
		}
	}

	// --- Batería baja (sin AC) ---
	if !s.OnAC && s.CapacityPct <= 20 {
		if !e.lowBattWarned {
			e.lowBattWarned = true
			d.Events = append(d.Events, Event{Critical, "🔋 Batería baja",
				fmt.Sprintf("%d%% — conecta el cargador.", s.CapacityPct)})
		}
	} else if s.OnAC || s.CapacityPct > 25 {
		e.lowBattWarned = false
	}

	// --- Salud (aviso como mucho semanal) ---
	if s.HealthPct > 0 && s.HealthPct < e.cfg.HealthWarning {
		if now.Sub(e.lastHealthWarn) > 7*24*time.Hour {
			e.lastHealthWarn = now
			d.Events = append(d.Events, Event{Warning, "🔋 Salud de la batería",
				fmt.Sprintf("Salud al %.0f%% (capacidad real vs. diseño).", s.HealthPct)})
		}
	}

	return d
}
