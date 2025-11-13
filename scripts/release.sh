#!/bin/bash

# GoRules Terraform Provider Release Helper
# This script helps prepare and create provider releases

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Utility functions
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

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check Git
    if ! command -v git &> /dev/null; then
        log_error "Git is not installed"
        exit 1
    fi
    
    # Check Go
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed"
        exit 1
    fi
    
    # Check that we're in a Git repository
    if ! git rev-parse --git-dir &> /dev/null; then
        log_error "You are not in a Git repository"
        exit 1
    fi
    
    # Check that we're on the main branch
    current_branch=$(git branch --show-current)
    if [ "$current_branch" != "main" ]; then
        log_warning "You are not on the main branch (current: $current_branch)"
        read -p "Continue? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
    
    log_success "Prerequisites verified"
}

# Check that there are no uncommitted changes
check_clean_state() {
    log_info "Checking repository state..."
    
    if ! git diff-index --quiet HEAD --; then
        log_error "There are uncommitted changes"
        git status --porcelain
        exit 1
    fi
    
    log_success "Repository is clean"
}

# Build the provider
build_provider() {
    log_info "Building the provider..."
    
    if ! make clean && make build; then
        log_error "Error building the provider"
        exit 1
    fi
    
    log_success "Provider built successfully"
}

# Create tag and release
create_release() {
    local version=$1
    
    if [ -z "$version" ]; then
        log_error "Version not specified"
        exit 1
    fi
    
    # Verify version format (must be semantic version)
    if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?$ ]]; then
        log_error "Invalid version format. Use format: v1.2.3 or v1.2.3-beta"
        exit 1
    fi
    
    log_info "Creating release $version..."
    
    # Verify that the tag doesn't exist
    if git tag -l | grep -q "^$version$"; then
        log_error "Tag $version already exists"
        exit 1
    fi
    
    # Create tag
    git tag -a "$version" -m "Release $version"
    
    # Push tag
    git push origin "$version"
    
    log_success "Release $version created and pushed"
    log_info "GitHub Actions should be running the automatic build"
    log_info "You can monitor at: https://github.com/andredelgado-ruiz/terraform-provider-gorules/actions"
}

# Show help
show_help() {
    echo "GoRules Terraform Provider Release Helper"
    echo ""
    echo "Usage: $0 [command] [options]"
    echo ""
    echo "Commands:"
    echo "  check       Check prerequisites and repository state"
    echo "  build       Build the provider"
    echo "  release     Create a new release"
    echo "  help        Show this help"
    echo ""
    echo "Examples:"
    echo "  $0 check"
    echo "  $0 build"
    echo "  $0 release v0.1.0"
    echo "  $0 release v0.1.1-beta"
}

# Main function
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
            log_error "Unknown command: $command"
            show_help
            exit 1
            ;;
    esac
}

# Execute main function with all arguments
main "$@"