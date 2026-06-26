// Package daemon une todas las piezas: lee, evalúa reglas, aplica protección,
// notifica y registra, en un bucle controlado por un ticker.
package daemon

import (
	"context"
	"fmt"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"battery-guardian/internal/battery"
	"battery-guardian/internal/charge"
	"battery-guardian/internal/config"
	"battery-guardian/internal/logger"
	"battery-guardian/internal/notifier"
	"battery-guardian/internal/rules"
	"battery-guardian/internal/sensors"
)

type Daemon struct {
	cfg  config.Config
	log  *logger.Logger
	mon  *battery.Monitor
	ctrl charge.Controller
	caps charge.Capabilities
	note *notifier.Notifier
	eng  *rules.Engine

	lastCap  int
	lastTime time.Time
}

// New construye el daemon detectando batería y método de control de carga.
func New(cfg config.Config, log *logger.Logger) (*Daemon, error) {
	mon, err := battery.Detect()
	if err != nil {
		return nil, err
	}
	ctrl, err := charge.Detect()
	if err != nil {
		return nil, err
	}
	caps := ctrl.Capabilities()
	eng := rules.NewEngine(rules.Config{
		Upper: cfg.UpperLimit, Lower: cfg.LowerLimit,
		TempWarning: cfg.TempWarning, TempCritical: cfg.TempCritical,
		HealthWarning: cfg.HealthWarning,
		ApproxLimit:   caps.ApproxLimit, Configurable: caps.Configurable,
	})
	return &Daemon{
		cfg: cfg, log: log, mon: mon, ctrl: ctrl, caps: caps,
		note: notifier.New(cfg.NotifyEnabled), eng: eng,
	}, nil
}

// Run ejecuta el bucle principal hasta recibir SIGINT/SIGTERM.
func (d *Daemon) Run() error {
	d.log.Infof("Battery Guardian iniciado — batería=%s método=%s (%s) tope≈%d%% configurable=%v",
		d.mon.BatteryName(), d.caps.Method, d.caps.Vendor, d.caps.ApproxLimit, d.caps.Configurable)
	d.log.Infof("Umbrales: activar=%d%% desactivar=%d%% intervalo=%s",
		d.cfg.UpperLimit, d.cfg.LowerLimit, d.cfg.PollInterval)
	if _, ok := sensors.Read().BatteryTemp(); !ok {
		d.log.Infof("Sin sensor de temperatura de batería: las alertas térmicas de batería quedan inactivas (la temp de sistema es solo informativa).")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()

	d.tick(time.Now()) // primer tick inmediato
	for {
		select {
		case <-ctx.Done():
			d.log.Infof("Battery Guardian detenido.")
			return nil
		case now := <-ticker.C:
			d.tick(now)
		}
	}
}

func (d *Daemon) tick(now time.Time) {
	s := d.mon.Read()
	t := sensors.Read()
	protected, err := d.ctrl.IsProtected()
	if err != nil {
		d.log.Errorf("no se pudo leer el estado de protección: %v", err)
	}

	dec := d.eng.Evaluate(s, t, protected, now)

	if dec.SetProtection != nil {
		if *dec.SetProtection {
			if err := d.ctrl.EnableProtection(d.cfg.UpperLimit); err != nil {
				d.log.Errorf("no se pudo activar la protección: %v", err)
			} else {
				d.log.Infof("Protección ACTIVADA a %d%% (método %s)", s.CapacityPct, d.caps.Method)
			}
		} else {
			if err := d.ctrl.DisableProtection(); err != nil {
				d.log.Errorf("no se pudo desactivar la protección: %v", err)
			} else {
				d.log.Infof("Protección DESACTIVADA a %d%% — carga reanudada", s.CapacityPct)
			}
		}
	}

	for _, ev := range dec.Events {
		if err := d.note.Notify(ev.Level, ev.Title, ev.Body); err != nil {
			d.log.Warnf("notificación no enviada: %v", err)
		}
	}

	d.log.Infof("cap=%d%% %s ac=%v prot=%v pow=%.1fW%s%s%s",
		s.CapacityPct, s.Status, s.OnAC, protected, s.PowerW,
		d.remainingStr(s), d.rateStr(s, now), tempStr(t))

	d.lastCap, d.lastTime = s.CapacityPct, now
}

func (d *Daemon) rateStr(s battery.Sample, now time.Time) string {
	if d.lastTime.IsZero() {
		return ""
	}
	dt := now.Sub(d.lastTime).Minutes()
	if dt <= 0 {
		return ""
	}
	return fmt.Sprintf(" rate=%+.2f%%/min", float64(s.CapacityPct-d.lastCap)/dt)
}

func (d *Daemon) remainingStr(s battery.Sample) string {
	switch s.Status {
	case "Charging":
		if r := s.RemainingToFull(); r > 0 {
			return fmt.Sprintf(" full_in=%s", r.Round(time.Minute))
		}
	case "Discharging":
		if r := s.RemainingToEmpty(); r > 0 {
			return fmt.Sprintf(" empty_in=%s", r.Round(time.Minute))
		}
	}
	return ""
}

func tempStr(t sensors.Temps) string {
	if tp, ok := t.BatteryTemp(); ok {
		return fmt.Sprintf(" battemp=%.0f°C", tp)
	}
	if tp, ok := t.SystemTemp(); ok {
		return fmt.Sprintf(" systemp=%.0f°C", tp)
	}
	return ""
}

// Status devuelve un resumen legible del estado actual (modo -status).
func (d *Daemon) Status() string {
	s := d.mon.Read()
	t := sensors.Read()
	protected, _ := d.ctrl.IsProtected()

	var b strings.Builder
	fmt.Fprintf(&b, "Battery Guardian — estado actual\n")
	fmt.Fprintf(&b, "  Batería:        %s\n", d.mon.BatteryName())
	fmt.Fprintf(&b, "  Carga:          %d%% (%s)\n", s.CapacityPct, s.Status)
	fmt.Fprintf(&b, "  En AC:          %v\n", s.OnAC)
	fmt.Fprintf(&b, "  Voltaje:        %.2f V\n", s.VoltageV)
	fmt.Fprintf(&b, "  Corriente:      %.2f A\n", s.CurrentA)
	fmt.Fprintf(&b, "  Potencia:       %.1f W\n", s.PowerW)
	fmt.Fprintf(&b, "  Ciclos:         %d\n", s.CycleCount)
	if s.HealthPct > 0 {
		fmt.Fprintf(&b, "  Salud:          %.0f%% (real vs. diseño)\n", s.HealthPct)
	}
	if r := s.RemainingToFull(); r > 0 && s.Status == "Charging" {
		fmt.Fprintf(&b, "  Tiempo a lleno: %s\n", r.Round(time.Minute))
	}
	if r := s.RemainingToEmpty(); r > 0 && s.Status == "Discharging" {
		fmt.Fprintf(&b, "  Autonomía:      %s\n", r.Round(time.Minute))
	}
	if tp, ok := t.BatteryTemp(); ok {
		fmt.Fprintf(&b, "  Temp batería:   %.0f°C\n", tp)
	}
	if tp, ok := t.SystemTemp(); ok {
		fmt.Fprintf(&b, "  Temp sistema:   %.0f°C (CPU/GPU, informativa)\n", tp)
	}
	fmt.Fprintf(&b, "  Método:         %s (%s)\n", d.caps.Method, d.caps.Vendor)
	fmt.Fprintf(&b, "  Protección:     %v (tope≈%d%%)\n", protected, d.caps.ApproxLimit)
	fmt.Fprintf(&b, "  Umbrales cfg:   activar=%d%% / desactivar=%d%%\n", d.cfg.UpperLimit, d.cfg.LowerLimit)
	return b.String()
}
