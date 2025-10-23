# Nombre del provider y metadata
NAME=gorules
NAMESPACE=andredelgadoruiz
VERSION=0.0.1

# Detecta OS y arquitectura automáticamente
OS := $(shell go env GOOS)
ARCH := $(shell go env GOARCH)

# Ruta donde Terraform busca providers locales
PLUGINDIR := $(HOME)/.terraform.d/plugins/registry.terraform.io/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS)_$(ARCH)

# Binario local
BINARY := bin/terraform-provider-$(NAME)_v$(VERSION)

# ========================================
# 🏗️  Build & Install targets
# ========================================

# Compila el provider
build:
	@echo "🚧 Compilando provider $(NAME) v$(VERSION) para $(OS)/$(ARCH)..."
	go mod tidy
	go build -o $(BINARY) .

# Instala el provider en la ruta local de Terraform
install: build
	@echo "📦 Instalando en $(PLUGINDIR)..."
	mkdir -p $(PLUGINDIR)
	cp $(BINARY) $(PLUGINDIR)/
	@echo "✅ Provider instalado correctamente:"
	@echo "   $(PLUGINDIR)/$(notdir $(BINARY))"

# Limpieza (opcional)
clean:
	rm -rf bin

# ========================================
# 🔁 Utilidades (no obligatorias)
# ========================================

# Muestra la ruta de instalación que Terraform usará
show-path:
	@echo "$(PLUGINDIR)"

# ========================================
# 🚀 Release targets
# ========================================

# Instalar GoReleaser (si no está instalado)
install-goreleaser:
	@if ! command -v goreleaser >/dev/null 2>&1; then \
		echo "Installing GoReleaser..."; \
		go install github.com/goreleaser/goreleaser@latest; \
	else \
		echo "GoReleaser already installed"; \
	fi

# Crear un release local para testing (sin firma)
release-snapshot:
	goreleaser release --snapshot --clean --skip=sign

# Crear un release local con firma (requiere GPG_FINGERPRINT)
release-snapshot-signed:
	goreleaser release --snapshot --clean

# Validar configuración de GoReleaser
release-check:
	goreleaser check

# Generar changelog (requiere git)
changelog:
	@echo "## Changelog\n" > CHANGELOG_NEW.md
	@git log --oneline --decorate --since="$(shell git describe --tags --abbrev=0 2>/dev/null || echo '1 year ago')" >> CHANGELOG_NEW.md
	@echo "Generated changelog in CHANGELOG_NEW.md"

# Preparar para release
prepare-release:
	@echo "🔍 Verificando prerrequisitos para release..."
	./scripts/release.sh check
	@echo "✅ Listo para release"

# Crear release (requiere versión como parámetro)
# Uso: make release VERSION=v0.1.0
release:
	@if [ -z "$(VERSION)" ]; then \
		echo "❌ Error: Debes especificar VERSION"; \
		echo "Uso: make release VERSION=v0.1.0"; \
		exit 1; \
	fi
	./scripts/release.sh release $(VERSION)

# Mostrar ayuda de release
release-help:
	./scripts/release.sh help

.PHONY: install-goreleaser release-snapshot release-check changelog prepare-release release release-help