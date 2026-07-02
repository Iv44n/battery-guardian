# 🔋 Battery Guardian

Servicio ligero para Linux que cuida la carga y la salud de la batería del portátil
mediante **reglas y cálculos** (sin IA). Despierta cada 15–30 s, consume <0.1% de CPU
y solo usa la **biblioteca estándar de Go** (cero dependencias externas).

Detecta automáticamente el método de control de carga del fabricante detrás de una
interfaz común (`ChargeController`), así que el mismo daemon sirve para varios equipos:

| Fabricante | Método | Configurable |
|------------|--------|--------------|
| Lenovo **IdeaPad** | `conservation_mode` (tope ~60%) | No (on/off) |
| **ThinkPad** / genérico | `charge_control_end_threshold` | Sí (umbral exacto) |

> Detectado en **este** equipo (IdeaPad Gaming 3 15ACH6): `conservation_mode`.

## ⚠️ Nota importante para IdeaPad

`conservation_mode` **no mantiene un umbral exacto** como en ThinkPad: cuando se activa,
reapunta el firmware a **~60%**. Por eso, con:

```yaml
battery:
  upper_limit: 80
  lower_limit: 65
```

el daemon **activa** la protección al llegar a 80% y la **desactiva** al bajar a 65%.
El comportamiento físico resultante (mantenerse cerca de 80%, o derivar hacia ~60%
mientras está protegido) depende del firmware y **se observa en los logs**:

```bash
journalctl -u battery-guardian -f
```

En equipos con umbral configurable (ThinkPad), `EnableProtection()` fija directamente
el límite (p.ej. 80%) y sí se mantiene esa banda.

## Qué hace

- **Protección por histéresis**: corta la carga en `upper_limit`, la reanuda en `lower_limit`.
- **Temperatura**: avisa al superar `warning` / `critical` °C.
- **Batería baja**: aviso urgente por debajo del 20% sin cargador.
- **Salud**: aviso (como mucho semanal) si `energy_full/diseño` cae por debajo del umbral.
- **Cálculos**: potencia (W), tiempo a lleno / autonomía, ritmo (%/min), ciclos, salud.
- **Notificaciones** de escritorio vía `notify-send`.
- **Logs** al journal de systemd.

## Estructura

```
battery-guardian/
├── cmd/guardian/         # main
├── internal/
│   ├── battery/          # lectura de /sys/class/power_supply (charge_* o energy_*)
│   ├── charge/           # ChargeController: ideapad + threshold + autodetección
│   ├── sensors/          # temperaturas desde hwmon
│   ├── rules/            # máquina de estados + reglas
│   ├── notifier/         # notify-send en la sesión del usuario
│   ├── logger/           # logger mínimo
│   └── daemon/           # bucle principal
├── config.yaml
├── battery-guardian.service
├── install.sh / uninstall.sh
└── Makefile
```

## Compilar y probar (sin root)

```bash
git clone https://github.com/Iv44n/battery-guardian.git
cd battery-guardian
make build                            # compila (inyecta la versión desde git)
./bin/battery-guardian -status        # leer estado, sin escribir nada
./bin/battery-guardian -version       # versión compilada
```

## Instalar (requiere root)

Requisitos: Linux con systemd, Go 1.26+ para compilar y `libnotify-bin` para
las notificaciones de escritorio.

```bash
git clone https://github.com/Iv44n/battery-guardian.git
cd battery-guardian
make install                          # compila, instala y arranca el servicio
sudo apt install -y libnotify-bin     # notify-send, para las notificaciones
```

`make install` hace, en orden:

1. Compila `bin/battery-guardian` inyectando la versión desde git.
2. Copia el binario a `/usr/local/bin/battery-guardian`.
3. Instala `config.yaml` en `/etc/battery-guardian/` — si ya existe uno,
   **se conserva** y el ejemplo nuevo queda como `config.yaml.new`.
4. Instala la unidad en `/etc/systemd/system/`, la habilita y (re)arranca el servicio.

Comprueba que quedó funcionando:

```bash
systemctl status battery-guardian     # debe estar «active (running)»
journalctl -u battery-guardian -f     # logs en vivo
battery-guardian -status              # resumen del estado (sin root)
sudo battery-guardian -test-notify    # prueba la ruta de notificaciones del servicio
```

## Actualizar

```bash
git pull                              # trae la última versión del repo
make update                           # recompila, reinstala y reinicia el servicio
```

o, para además actualizar las dependencias de Go (si las hubiera) y pasar
`go vet` + tests antes de instalar:

```bash
make upgrade
```

En ambos casos tu `/etc/battery-guardian/config.yaml` no se toca. Tras
actualizar, verifica con `systemctl status battery-guardian`.

## Desinstalar (revierte todo)

```bash
make uninstall                        # o: sudo bash uninstall.sh
```

El desinstalador:

1. Detiene y deshabilita el servicio.
2. **Desactiva la protección** — la batería vuelve a cargar con normalidad.
3. Elimina el binario, la unidad de systemd y `/opt/battery-guardian`.
4. Conserva `/etc/battery-guardian/config.yaml`; para borrarlo también:
   `sudo rm -rf /etc/battery-guardian`.

## Configuración

Ver `config.yaml`. Tras editarlo: `sudo systemctl restart battery-guardian`.

## Contribuir

Las PRs son bienvenidas, sobre todo para añadir implementaciones de
`ChargeController` de otros fabricantes (Dell, ASUS, Framework…). El núcleo
(reglas, cálculos, notificaciones) no cambia: basta con implementar la interfaz
en `internal/charge` y añadir su detección.

## Licencia

[MIT](LICENSE) © 2026 Iván (Iv44n).

