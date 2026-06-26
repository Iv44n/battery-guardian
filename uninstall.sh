#!/usr/bin/env bash
# Desinstalador de Battery Guardian. Ejecutar como root:
#   sudo bash uninstall.sh
#
# Deja la batería en estado normal (desactiva la protección) antes de salir.

set -uo pipefail

echo "==> Deteniendo y deshabilitando el servicio"
systemctl disable --now battery-guardian.service 2>/dev/null || true

echo "==> Desactivando la protección (reanuda carga normal)"
CM=$(find /sys -name conservation_mode 2>/dev/null | head -n1)
[ -n "$CM" ] && echo 0 > "$CM" 2>/dev/null || true

echo "==> Eliminando archivos"
rm -f /usr/local/bin/battery-guardian
rm -f /etc/systemd/system/battery-guardian.service
rm -rf /opt/battery-guardian
systemctl daemon-reload

echo "    (Se conserva /etc/battery-guardian/config.yaml; bórralo a mano si quieres.)"
echo "✅ Battery Guardian desinstalado."
