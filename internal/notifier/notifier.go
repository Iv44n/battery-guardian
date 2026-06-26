// Package notifier envía notificaciones de escritorio con notify-send.
// Como el daemon corre como root, lanza notify-send en la sesión del usuario
// gráfico (su bus de D-Bus) mediante runuser.
package notifier

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
)

// Level es la urgencia de la notificación.
type Level int

const (
	Info Level = iota
	Warning
	Critical
)

type Notifier struct {
	enabled bool
	uid     string
	uname   string
}

// New detecta al usuario gráfico activo (primer UID >= 1000 en /run/user).
func New(enabled bool) *Notifier {
	n := &Notifier{enabled: enabled}
	n.uid, n.uname = detectUser()
	return n
}

func detectUser() (string, string) {
	entries, err := os.ReadDir("/run/user")
	if err != nil {
		return "", ""
	}
	var uids []int
	for _, e := range entries {
		if id, err := strconv.Atoi(e.Name()); err == nil && id >= 1000 {
			uids = append(uids, id)
		}
	}
	if len(uids) == 0 {
		return "", ""
	}
	sort.Ints(uids)
	uid := strconv.Itoa(uids[0])
	name := uid
	if u, err := user.LookupId(uid); err == nil {
		name = u.Username
	}
	return uid, name
}

func urgency(l Level) string {
	switch l {
	case Critical:
		return "critical"
	case Warning:
		return "normal"
	default:
		return "low"
	}
}

// Notify muestra una notificación. Si ya corremos como usuario normal,
// usa notify-send directamente; si somos root (caso del servicio), lo lanza
// en la sesión del usuario gráfico mediante runuser.
func (n *Notifier) Notify(level Level, title, body string) error {
	if !n.enabled {
		return nil
	}
	if os.Getuid() != 0 {
		cmd := exec.Command("notify-send", "-a", "Battery Guardian", "-u", urgency(level), title, body)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("notify-send falló: %v (%s)", err, string(out))
		}
		return nil
	}
	if n.uid == "" {
		return nil
	}
	runtime := filepath.Join("/run/user", n.uid)
	bus := "unix:path=" + filepath.Join(runtime, "bus")
	cmd := exec.Command("runuser", "-u", n.uname, "--",
		"env",
		"DBUS_SESSION_BUS_ADDRESS="+bus,
		"XDG_RUNTIME_DIR="+runtime,
		"DISPLAY=:0",
		"notify-send", "-a", "Battery Guardian", "-u", urgency(level), title, body,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("notify-send falló: %v (%s)", err, string(out))
	}
	return nil
}
