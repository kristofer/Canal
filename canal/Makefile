# Canal — capability microkernel for ESP32-S3
# Usage:
#   make flash          build + flash (auto-detects port)
#   make flash PORT=…   flash to a specific port
#   make monitor        open serial monitor
#   make run            flash then monitor
#   make build          build only (no flash)
#   make clean

SHELL := /bin/bash

TINYGO        ?= tinygo
TINYGO_TARGET ?= esp32s3-generic
USE_IDF       ?= 1          # link against ESP-IDF FreeRTOS (required for flash domain loading)
BAUD          ?= 115200

KERNEL_DIR  := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))kernel
DOMAINS_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))domains
OUT_DIR     := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))build/out

# Auto-detect the first connected ESP32 USB-serial port.
# Override with: make flash PORT=/dev/cu.usbmodem1301
ifeq ($(shell uname),Darwin)
_DETECTED_PORT := $(firstword $(wildcard /dev/cu.usbmodem*))
else
_DETECTED_PORT := $(firstword $(wildcard /dev/ttyUSB* /dev/ttyACM*))
endif
PORT ?= $(_DETECTED_PORT)

ifeq ($(USE_IDF),1)
EXTRA_TAGS := -tags=idf
else
EXTRA_TAGS :=
endif

# ── prereq checks ────────────────────────────────────────────────────────────

.PHONY: _check_tinygo _check_port

_check_tinygo:
	@command -v $(TINYGO) >/dev/null 2>&1 || \
		{ echo "error: tinygo not found (brew install tinygo)"; exit 1; }

_check_port:
	@[ -n "$(PORT)" ] || \
		{ echo "error: no ESP32 USB port found — plug in the board or set PORT=…"; exit 1; }
	@[ -e "$(PORT)" ] || \
		{ echo "error: port $(PORT) does not exist"; exit 1; }

# ── primary targets ───────────────────────────────────────────────────────────

.PHONY: all build build-domains led wifi logger \
        flash flash-domains flash-all flash-led flash-wifi flash-logger \
        monitor run clean help

all: build

build: _check_tinygo
	@echo "Building Canal kernel (target=$(TINYGO_TARGET))..."
	cd $(KERNEL_DIR) && $(TINYGO) build \
		-target=$(TINYGO_TARGET) \
		-opt=2 \
		$(EXTRA_TAGS) \
		-o ../build/out/kernel.elf \
		./cmd/esp32s3
	@echo "Built: build/out/kernel.elf"

build-domains: led wifi logger

led: _check_tinygo
	@echo "Building LED domain..."
	@mkdir -p $(OUT_DIR)
	cd $(DOMAINS_DIR)/led-blinker && $(TINYGO) build \
		-target=$(TINYGO_TARGET) \
		-opt=2 \
		$(EXTRA_TAGS) \
		-o $(OUT_DIR)/led.elf \
		.
	@echo "Built: build/out/led.elf"

wifi: _check_tinygo
	@echo "Building WiFi domain..."
	@mkdir -p $(OUT_DIR)
	cd $(DOMAINS_DIR)/wifi && $(TINYGO) build \
		-target=$(TINYGO_TARGET) \
		-opt=2 \
		$(EXTRA_TAGS) \
		-o $(OUT_DIR)/wifi.elf \
		./cmd/esp32s3
	@echo "Built: build/out/wifi.elf"

logger: _check_tinygo
	@echo "Building Logger domain..."
	@mkdir -p $(OUT_DIR)
	cd $(DOMAINS_DIR)/logger && $(TINYGO) build \
		-target=$(TINYGO_TARGET) \
		-opt=2 \
		$(EXTRA_TAGS) \
		-o $(OUT_DIR)/logger.elf \
		.
	@echo "Built: build/out/logger.elf"

flash: _check_tinygo _check_port
	@echo "Flashing to $(PORT)..."
	cd $(KERNEL_DIR) && $(TINYGO) flash \
		-target=$(TINYGO_TARGET) \
		-opt=2 \
		$(EXTRA_TAGS) \
		-port=$(PORT) \
		./cmd/esp32s3

flash-domains: _check_port led wifi logger
	@echo "Flashing all domain binaries to $(PORT)..."
	esptool.py --chip esp32s3 --port $(PORT) --baud 921600 write_flash \
		0x100000 $(OUT_DIR)/led.elf \
		0x180000 $(OUT_DIR)/wifi.elf \
		0x200000 $(OUT_DIR)/logger.elf
	@echo "Domains flashed"

# Flash kernel + domains in sequence.
flash-all: _check_tinygo _check_port
	@$(MAKE) --no-print-directory flash PORT=$(PORT)
	@echo "Waiting for device to re-enumerate..."
	@sleep 3
	@NEW_PORT=$$(ls /dev/cu.usbmodem* /dev/ttyUSB* /dev/ttyACM* 2>/dev/null | head -1); \
	$(MAKE) --no-print-directory flash-domains PORT=$$NEW_PORT

# Re-flash a single domain without touching the kernel.
flash-led: _check_port led
	@echo "Flashing LED domain to $(PORT) at 0x100000..."
	esptool.py --chip esp32s3 --port $(PORT) --baud 921600 write_flash \
		0x100000 $(OUT_DIR)/led.elf
	@echo "LED domain flashed"

flash-wifi: _check_port wifi
	@echo "Flashing WiFi domain to $(PORT) at 0x180000..."
	esptool.py --chip esp32s3 --port $(PORT) --baud 921600 write_flash \
		0x180000 $(OUT_DIR)/wifi.elf
	@echo "WiFi domain flashed"

flash-logger: _check_port logger
	@echo "Flashing Logger domain to $(PORT) at 0x200000..."
	esptool.py --chip esp32s3 --port $(PORT) --baud 921600 write_flash \
		0x200000 $(OUT_DIR)/logger.elf
	@echo "Logger domain flashed"

monitor: _check_port
	@echo "Monitoring $(PORT) at $(BAUD) baud  (Ctrl-] to quit)"
	python3 -m serial.tools.miniterm --dtr 0 --rts 0 $(PORT) $(BAUD)

# flash then immediately monitor (the board re-enumerates, so wait briefly)
run: _check_tinygo _check_port
	@$(MAKE) --no-print-directory flash PORT=$(PORT)
	@echo "Waiting for device to re-enumerate..."
	@sleep 2
	@NEW_PORT=$$(ls /dev/cu.usbmodem* /dev/ttyUSB* /dev/ttyACM* 2>/dev/null | head -1); \
	echo "Monitoring $$NEW_PORT at $(BAUD) baud  (Ctrl-] to quit)"; \
	python3 -m serial.tools.miniterm --dtr 0 --rts 0 $$NEW_PORT $(BAUD)

clean:
	rm -rf build/out
	@echo "Cleaned build/out"

help:
	@echo ""
	@echo "Canal build system"
	@echo ""
	@echo "  make build            compile kernel only"
	@echo "  make build-domains    compile all domain binaries (led, wifi, logger)"
	@echo "  make led/wifi/logger  compile a single domain"
	@echo ""
	@echo "  make flash            compile + flash kernel (auto-detects port)"
	@echo "  make flash-domains    flash all domains to their partitions"
	@echo "  make flash-all        flash kernel then all domains"
	@echo "  make flash-led        rebuild + flash LED domain only"
	@echo "  make flash-wifi       rebuild + flash WiFi domain only"
	@echo "  make flash-logger     rebuild + flash Logger domain only"
	@echo ""
	@echo "  make monitor          open serial monitor"
	@echo "  make run              flash kernel + monitor"
	@echo "  make clean            remove build artifacts"
	@echo ""
	@echo "  scripts/flash.sh [PORT] [--domains-only] [--led-only]"
	@echo ""
	@echo "Variables (override on command line):"
	@echo "  PORT=$(PORT)"
	@echo "  TINYGO_TARGET=$(TINYGO_TARGET)"
	@echo "  USE_IDF=$(USE_IDF)   (default 1: link real FreeRTOS via ESP-IDF for flash domain loading)"
	@echo "                      (set to 0 to disable IDF — goroutine fallbacks only, no flash loading)"
	@echo "  BAUD=$(BAUD)"
	@echo ""
