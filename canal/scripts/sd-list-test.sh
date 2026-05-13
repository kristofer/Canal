#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    echo "usage: $0 <device-ip> [port]" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CANAL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_FILE="$CANAL_DIR/domains/sdcard/sd-list-mg.pc"
HOST="$1"
PORT="${2:-2323}"

command -v nc >/dev/null 2>&1 || {
    echo "error: nc (netcat) not found; install it with brew install netcat" >&2
    exit 1
}

if [ ! -f "$TEST_FILE" ]; then
    echo "error: test file not found: $TEST_FILE" >&2
    exit 1
fi

TMP_OUT="$(mktemp)"
trap 'rm -f "$TMP_OUT"' EXIT

{
    printf -- '---\n'
    cat "$TEST_FILE"
    printf -- '\n---\n'
} | nc "$HOST" "$PORT" | tee "$TMP_OUT"

if grep -qi "list failed" "$TMP_OUT"; then
    echo "sd-list-test: FAIL (fs list reported failure)" >&2
    exit 1
fi

echo "sd-list-test: PASS (no list failure reported)"
