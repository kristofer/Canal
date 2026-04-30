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
USE_IDF       ?= 0          # set to 1 to link against ESP-IDF FreeRTOS
BAUD          ?= 115200

KERNEL_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))kernel

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

.PHONY: all build flash monitor run clean help

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

flash: _check_tinygo _check_port
	@echo "Flashing to $(PORT)..."
	cd $(KERNEL_DIR) && $(TINYGO) flash \
		-target=$(TINYGO_TARGET) \
		-opt=2 \
		$(EXTRA_TAGS) \
		-port=$(PORT) \
		./cmd/esp32s3

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
	@echo "  make build          compile kernel (no flash)"
	@echo "  make flash          compile + flash  (auto-detects port)"
	@echo "  make monitor        open serial monitor"
	@echo "  make run            flash + monitor"
	@echo "  make clean          remove build artifacts"
	@echo ""
	@echo "Variables (override on command line):"
	@echo "  PORT=$(PORT)"
	@echo "  TINYGO_TARGET=$(TINYGO_TARGET)"
	@echo "  USE_IDF=$(USE_IDF)   (1 = link real FreeRTOS via ESP-IDF)"
	@echo "  BAUD=$(BAUD)"
	@echo ""
