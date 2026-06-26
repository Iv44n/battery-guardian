// Package logger es un logger mínimo con niveles. Escribe a stderr, que
// systemd captura en el journal.
package logger

import (
	"log"
	"os"
)

type Logger struct {
	l       *log.Logger
	enabled bool // si es false, Infof se silencia (Warn/Error siempre salen)
}

func New(enabled bool) *Logger {
	return &Logger{l: log.New(os.Stderr, "", log.LstdFlags), enabled: enabled}
}

func (g *Logger) Infof(format string, a ...any) {
	if g.enabled {
		g.l.Printf("INFO  "+format, a...)
	}
}

func (g *Logger) Warnf(format string, a ...any)  { g.l.Printf("WARN  "+format, a...) }
func (g *Logger) Errorf(format string, a ...any) { g.l.Printf("ERROR "+format, a...) }
