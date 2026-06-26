// Command guardian es el punto de entrada del daemon Battery Guardian.
package main

import (
	"flag"
	"fmt"
	"os"

	"battery-guardian/internal/config"
	"battery-guardian/internal/daemon"
	"battery-guardian/internal/logger"
	"battery-guardian/internal/notifier"
)

// version se inyecta al compilar con -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cfgPath := flag.String("config", "/etc/battery-guardian/config.yaml", "ruta al archivo de configuración")
	status := flag.Bool("status", false, "imprime el estado actual y sale (no requiere root)")
	testNotify := flag.Bool("test-notify", false, "envía una notificación de prueba y sale (ejecútalo con sudo para probar la ruta del servicio)")
	showVersion := flag.Bool("version", false, "imprime la versión y sale")
	flag.Parse()

	if *showVersion {
		fmt.Println("battery-guardian", version)
		return
	}

	if *testNotify {
		n := notifier.New(true)
		if err := n.Notify(notifier.Info, "🔋 Battery Guardian", "Notificación de prueba — ¡funciona!"); err != nil {
			fmt.Fprintf(os.Stderr, "la notificación falló: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Notificación enviada correctamente.")
		return
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aviso: %v — usando configuración por defecto\n", err)
	}

	log := logger.New(cfg.LogEnabled)

	d, err := daemon.New(cfg, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error de inicialización: %v\n", err)
		os.Exit(1)
	}

	if *status {
		fmt.Print(d.Status())
		return
	}

	log.Infof("battery-guardian versión %s", version)
	if err := d.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
