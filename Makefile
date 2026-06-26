.PHONY: build status run install update uninstall clean fmt vet

# Versión a partir de git (tag, o hash corto, o "dev").
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/battery-guardian ./cmd/guardian

# Resumen rápido del estado (no requiere root).
status: build
	./bin/battery-guardian -status

# Ejecuta en primer plano usando el config local (necesita root para escribir).
run: build
	sudo ./bin/battery-guardian -config ./config.yaml

# Instala/actualiza el servicio: compila, copia binario+unidad y reinicia.
install: build
	sudo bash install.sh

# Alias explícito para actualizar una instalación existente.
update: install

uninstall:
	sudo bash uninstall.sh

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -rf bin
