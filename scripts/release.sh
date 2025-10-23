#!/bin/bash

# GoRules Terraform Provider Release Helper
# Este script ayuda a preparar y crear releases del provider

set -e

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Funciones de utilidad
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Verificar prerrequisitos
check_prerequisites() {
    log_info "Verificando prerrequisitos..."
    
    # Verificar Git
    if ! command -v git &> /dev/null; then
        log_error "Git no está instalado"
        exit 1
    fi
    
    # Verificar Go
    if ! command -v go &> /dev/null; then
        log_error "Go no está instalado"
        exit 1
    fi
    
    # Verificar que estamos en un repo Git
    if ! git rev-parse --git-dir &> /dev/null; then
        log_error "No estás en un repositorio Git"
        exit 1
    fi
    
    # Verificar que estamos en la rama main
    current_branch=$(git branch --show-current)
    if [ "$current_branch" != "main" ]; then
        log_warning "No estás en la rama main (actual: $current_branch)"
        read -p "¿Continuar? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
    
    log_success "Prerrequisitos verificados"
}

# Verificar que no hay cambios sin commitear
check_clean_state() {
    log_info "Verificando estado del repositorio..."
    
    if ! git diff-index --quiet HEAD --; then
        log_error "Hay cambios sin commitear"
        git status --porcelain
        exit 1
    fi
    
    log_success "Repositorio limpio"
}

# Compilar el provider
build_provider() {
    log_info "Compilando el provider..."
    
    if ! make clean && make build; then
        log_error "Error al compilar el provider"
        exit 1
    fi
    
    log_success "Provider compilado correctamente"
}

# Crear tag y release
create_release() {
    local version=$1
    
    if [ -z "$version" ]; then
        log_error "Versión no especificada"
        exit 1
    fi
    
    # Verificar formato de versión (debe ser semantic version)
    if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?$ ]]; then
        log_error "Formato de versión inválido. Usa formato: v1.2.3 o v1.2.3-beta"
        exit 1
    fi
    
    log_info "Creando release $version..."
    
    # Verificar que el tag no existe
    if git tag -l | grep -q "^$version$"; then
        log_error "El tag $version ya existe"
        exit 1
    fi
    
    # Crear tag
    git tag -a "$version" -m "Release $version"
    
    # Push tag
    git push origin "$version"
    
    log_success "Release $version creado y pusheado"
    log_info "GitHub Actions debería estar ejecutando el build automático"
    log_info "Puedes monitorear en: https://github.com/andredelgadoruiz/terraform-provider-gorules/actions"
}

# Mostrar ayuda
show_help() {
    echo "GoRules Terraform Provider Release Helper"
    echo ""
    echo "Uso: $0 [comando] [opciones]"
    echo ""
    echo "Comandos:"
    echo "  check       Verificar prerrequisitos y estado del repo"
    echo "  build       Compilar el provider"
    echo "  release     Crear un nuevo release"
    echo "  help        Mostrar esta ayuda"
    echo ""
    echo "Ejemplos:"
    echo "  $0 check"
    echo "  $0 build"
    echo "  $0 release v0.1.0"
    echo "  $0 release v0.1.1-beta"
}

# Función principal
main() {
    local command=$1
    
    case $command in
        "check")
            check_prerequisites
            check_clean_state
            ;;
        "build")
            check_prerequisites
            build_provider
            ;;
        "release")
            local version=$2
            check_prerequisites
            check_clean_state
            build_provider
            create_release "$version"
            ;;
        "help"|"--help"|"-h"|"")
            show_help
            ;;
        *)
            log_error "Comando desconocido: $command"
            show_help
            exit 1
            ;;
    esac
}

# Ejecutar función principal con todos los argumentos
main "$@"