#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    echo "usage: $0 <device-ip> [port]" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CANAL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SMOKE_FILE="$CANAL_DIR/domains/sdcard/wifi-repl-smoke.pc"
HOST="$1"
PORT="${2:-2323}"

command -v nc >/dev/null 2>&1 || {
    echo "error: nc (netcat) not found; install it with apt-get install netcat-openbsd or brew install netcat" >&2
    exit 1
}

{
    printf -- '---\n'
    cat "$SMOKE_FILE"
    printf -- '\n---\n'
} | nc "$HOST" "$PORT"
