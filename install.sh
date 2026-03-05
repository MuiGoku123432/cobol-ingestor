#!/usr/bin/env bash
set -euo pipefail

# install.sh — Build and install cobol-graph binaries, set up .env, and
#               optionally start Neo4j via Docker Compose.

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

info()  { printf '\033[1;34m==> %s\033[0m\n' "$*"; }
warn()  { printf '\033[1;33m==> %s\033[0m\n' "$*"; }
error() { printf '\033[1;31m==> %s\033[0m\n' "$*" >&2; }

# ── Pre-flight checks ───────────────────────────────────────────────
check_dependency() {
    if ! command -v "$1" &>/dev/null; then
        error "$1 is required but not installed."
        exit 1
    fi
}

info "Checking dependencies..."
check_dependency go
check_dependency docker

GO_VERSION="$(go version | grep -oE 'go[0-9]+\.[0-9]+' | head -1)"
GO_MINOR="$(echo "$GO_VERSION" | grep -oE '[0-9]+\.[0-9]+' | cut -d. -f2)"
if [ "$GO_MINOR" -lt 22 ]; then
    error "Go 1.22+ required (found $GO_VERSION)"
    exit 1
fi

# ── Build ────────────────────────────────────────────────────────────
info "Building binaries..."
cd "$SCRIPT_DIR"
make build

# ── Install ──────────────────────────────────────────────────────────
info "Installing binaries to $INSTALL_DIR..."
if [ -w "$INSTALL_DIR" ]; then
    make install INSTALL_DIR="$INSTALL_DIR"
else
    warn "Elevated permissions required for $INSTALL_DIR"
    sudo make install INSTALL_DIR="$INSTALL_DIR"
fi

# ── Environment file ────────────────────────────────────────────────
if [ ! -f "$SCRIPT_DIR/.env" ]; then
    info "Creating .env from .env.example..."
    cp "$SCRIPT_DIR/.env.example" "$SCRIPT_DIR/.env"
    warn "Edit .env to set ANTHROPIC_API_KEY and NEO4J_PASSWORD before running."
else
    info ".env already exists, skipping."
fi

# ── Neo4j ────────────────────────────────────────────────────────────
if docker compose version &>/dev/null; then
    COMPOSE_CMD="docker compose"
elif command -v docker-compose &>/dev/null; then
    COMPOSE_CMD="docker-compose"
else
    COMPOSE_CMD=""
fi

if [ -n "$COMPOSE_CMD" ]; then
    read -rp "Start Neo4j via Docker Compose? [y/N] " start_neo4j
    if [[ "$start_neo4j" =~ ^[Yy]$ ]]; then
        info "Starting Neo4j..."
        cd "$SCRIPT_DIR"
        $COMPOSE_CMD up -d
        info "Neo4j browser: http://localhost:7474"
        info "Bolt endpoint: bolt://localhost:7687"
    fi
else
    warn "docker compose not found — start Neo4j manually."
fi

# ── Done ─────────────────────────────────────────────────────────────
info "Installation complete!"
echo ""
echo "  Installed binaries:"
echo "    cobol-graph           — ingestion pipeline"
echo "    cobol-graph-api       — REST API server"
echo "    cobol-graph-mcp       — MCP tool server"
echo ""
echo "  Quick start:"
echo "    1. Edit .env with your ANTHROPIC_API_KEY"
echo "    2. cobol-graph ingest --dir=/path/to/cobol"
echo "    3. cobol-graph-api"
echo ""
