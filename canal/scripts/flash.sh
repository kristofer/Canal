#!/bin/bash
# scripts/flash.sh — build and flash Canal OS (kernel + all domains)
#
# Usage:
#   ./scripts/flash.sh                    # auto-detect port
#   ./scripts/flash.sh /dev/cu.usbmodem… # explicit port
#   ./scripts/flash.sh --domains-only     # skip kernel, re-flash domains only
#   ./scripts/flash.sh --led-only         # re-flash LED domain only

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CANAL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT_DIR="$CANAL_DIR/build/out"
CHIP=esp32s3
BAUD=921600

# ── Partition offsets (must match build/targets/esp32s3/partitions.csv) ──────
LED_ADDR=0x100000
WIFI_ADDR=0x180000
LOGGER_ADDR=0x200000

# ── Port detection ────────────────────────────────────────────────────────────
DOMAINS_ONLY=0
LED_ONLY=0
PORT=""

for arg in "$@"; do
    case "$arg" in
        --domains-only) DOMAINS_ONLY=1 ;;
        --led-only)     LED_ONLY=1 ;;
        /dev/*)         PORT="$arg" ;;
    esac
done

if [ -z "$PORT" ]; then
    if [[ "$(uname)" == "Darwin" ]]; then
        PORT=$(ls /dev/cu.usbmodem* 2>/dev/null | head -1)
    else
        PORT=$(ls /dev/ttyUSB* /dev/ttyACM* 2>/dev/null | head -1)
    fi
fi

if [ -z "$PORT" ]; then
    echo "error: no ESP32 port found — plug in the board or pass PORT as argument"
    exit 1
fi

echo "Canal OS flash  (chip=$CHIP  port=$PORT  baud=$BAUD)"
echo ""

# ── Prereqs ───────────────────────────────────────────────────────────────────
command -v tinygo    >/dev/null || { echo "error: tinygo not found"; exit 1; }
command -v esptool.py >/dev/null || { echo "error: esptool.py not found (pip install esptool)"; exit 1; }

mkdir -p "$OUT_DIR"

# ── LED-only shortcut ─────────────────────────────────────────────────────────
if [ "$LED_ONLY" -eq 1 ]; then
    echo "Building LED domain..."
    make -C "$CANAL_DIR" led
    echo ""
    echo "Flashing LED domain to $PORT at $LED_ADDR..."
    esptool.py --chip $CHIP --port "$PORT" --baud $BAUD \
        write_flash $LED_ADDR "$OUT_DIR/led.elf"
    echo ""
    echo "LED domain flashed — reset the board to apply."
    exit 0
fi

# ── Build ─────────────────────────────────────────────────────────────────────
if [ "$DOMAINS_ONLY" -eq 0 ]; then
    echo "Building kernel..."
    make -C "$CANAL_DIR" build
    echo ""
fi

echo "Building domain binaries..."
make -C "$CANAL_DIR" build-domains
echo ""

# ── Flash kernel ──────────────────────────────────────────────────────────────
if [ "$DOMAINS_ONLY" -eq 0 ]; then
    echo "Flashing kernel to $PORT..."
    make -C "$CANAL_DIR" flash PORT="$PORT"
    echo ""
    echo "Waiting for device to re-enumerate after kernel flash..."
    sleep 3
    # Re-detect port after reset
    if [[ "$(uname)" == "Darwin" ]]; then
        PORT=$(ls /dev/cu.usbmodem* 2>/dev/null | head -1)
    else
        PORT=$(ls /dev/ttyUSB* /dev/ttyACM* 2>/dev/null | head -1)
    fi
    echo "Port: $PORT"
    echo ""
fi

# ── Flash domains ─────────────────────────────────────────────────────────────
echo "Flashing domain binaries..."
echo "  led    -> $LED_ADDR"
echo "  wifi   -> $WIFI_ADDR"
echo "  logger -> $LOGGER_ADDR"
echo ""

esptool.py --chip $CHIP --port "$PORT" --baud $BAUD \
    write_flash \
    $LED_ADDR    "$OUT_DIR/led.elf" \
    $WIFI_ADDR   "$OUT_DIR/wifi.elf" \
    $LOGGER_ADDR "$OUT_DIR/logger.elf"

echo ""
echo "Flash complete."
echo ""
echo "Boot log (Ctrl-] to quit):"
python3 -m serial.tools.miniterm --dtr 0 --rts 0 "$PORT" 115200
