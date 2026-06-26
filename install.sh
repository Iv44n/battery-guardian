#!/usr/bin/env bash
# Instalador de Battery Guardian. Ejecutar como root:
#   sudo bash install.sh
#
# Requiere el binario ya compilado en ./bin/battery-guardian
# (compílalo sin root con:  make build   o   go build -o bin/battery-guardian ./cmd/guardian )

set -euo pipefail
cd "$(dirname "$0")"

BIN=bin/battery-guardian
if [ ! -x "$BIN" ]; then
  echo "ERROR: no existe $BIN. Compílalo primero (sin sudo):"
  echo "       cd $(pwd) && go build -o bin/battery-guardian ./cmd/guardian"
  exit 1
fi

echo "==> Instalando binario en /usr/local/bin"
install -m 0755 "$BIN" /usr/local/bin/battery-guardian

echo "==> Instalando configuración en /etc/battery-guardian"
install -d /etc/battery-guardian
if [ -f /etc/battery-guardian/config.yaml ]; then
  echo "    (config.yaml ya existe — se conserva; nuevo ejemplo en config.yaml.new)"
  install -m 0644 config.yaml /etc/battery-guardian/config.yaml.new
else
  install -m 0644 config.yaml /etc/battery-guardian/config.yaml
fi

echo "==> Instalando documentación en /opt/battery-guardian"
install -d /opt/battery-guardian
install -m 0644 README.md /opt/battery-guardian/README.md 2>/dev/null || true

echo "==> Instalando servicio systemd"
install -m 0644 battery-guardian.service /etc/systemd/system/battery-guardian.service
systemctl daemon-reload
systemctl enable battery-guardian.service
# restart (no enable --now): arranca si está parado y reinicia si ya corría,
# de modo que una actualización del binario/servicio surta efecto siempre.
systemctl restart battery-guardian.service

echo
echo "✅ Instalado y en marcha. Comandos útiles:"
echo "   systemctl status battery-guardian        # estado del servicio"
echo "   journalctl -u battery-guardian -f        # ver los logs en vivo"
echo "   battery-guardian -status                 # resumen rápido (sin root)"
echo
echo "Para que las notificaciones funcionen necesitas notify-send:"
echo "   sudo apt install -y libnotify-bin"
