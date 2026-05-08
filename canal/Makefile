# Canal — capability microkernel for ESP32-S3
# Usage:
#   make flash          build + flash kernel only (auto-detects port)
#   make flash PORT=…   flash kernel to a specific port
#   make flash-domains  build + flash all domain binaries
#   make flash-all      flash kernel then all domains (full install)
#   make monitor        open serial monitor
#   make run            flash kernel + domains, then monitor (full workflow)
#   make build          build only (no flash)
#   make clean

SHELL := /bin/bash

TINYGO        ?= tinygo
TINYGO_TARGET ?= esp32s3-generic
USE_IDF       ?= 1          # link against ESP-IDF FreeRTOS (required for flash domain loading)
BAUD          ?= 115200
DOMAIN_STACK  ?= led wifi
ALL_DOMAINS   ?= led wifi logger picoceci sdcard tls
ERASE_UNUSED  ?= 1
WIFI_SSID     ?=
WIFI_PASSWORD ?=
WIFI_PORT     ?= 2323

KERNEL_DIR  := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))kernel
DOMAINS_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))domains
OUT_DIR     := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))build/out
BUILD_DIR   := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))build
DOMAIN_LINKER := $(BUILD_DIR)/targets/esp32s3/domain-linker.ld

# Domain RAM windows are static arrays declared in bridge_main.c.
# Their addresses are read from the kernel ELF after build with nm.
# DOMAIN_RAM_SIZE must match DOMAIN_RAM_SLOT_SIZE in bridge_main.c.
DOMAIN_RAM_SIZE ?= 0x4000
NM ?= $(firstword $(wildcard /Users/kristofer/.espressif/tools/xtensa-esp-elf/*/xtensa-esp-elf/bin/xtensa-esp32s3-elf-nm) xtensa-esp32s3-elf-nm)
KERNEL_ELF := $(BUILD_DIR)/idf-app/build/canal_idf_bridge.elf
TINYGO_ROOT ?= $(shell tinygo env TINYGOROOT 2>/dev/null)

# Extract a symbol address from the kernel ELF. Usage: $(call nm_addr,SYMBOL)
define nm_addr
$(shell $(NM) $(KERNEL_ELF) 2>/dev/null | awk '/$(1)$$/{print "0x"$$1; exit}')
endef

LED_RAM_ORIGIN     = $(call nm_addr,domain_ram_led)
WIFI_RAM_ORIGIN    = $(call nm_addr,domain_ram_wifi)
LOGGER_RAM_ORIGIN  = $(call nm_addr,domain_ram_logger)
PICOCECI_RAM_ORIGIN= $(call nm_addr,domain_ram_picoceci)
TLS_RAM_ORIGIN     = $(call nm_addr,domain_ram_tls)

# Linker flags injected into every domain build to resolve FreeRTOS/ESP-IDF
# symbols (vTaskDelay, xQueueReceive, etc.).
#
# --just-symbols tells lld to read the symbol table from the kernel ELF and
# treat every symbol as an absolute definition at its linked virtual address.
# No sections are copied; domain code simply calls into the kernel's live
# FreeRTOS/ESP-IDF functions at runtime (kernel and domains share one flat
# virtual address space on ESP32-S3).
#
# This is intentionally open-ended: adding new FreeRTOS/ESP-IDF //export
# declarations to any domain package will resolve automatically without any
# extra Makefile changes.
#
# --just-symbols is an external linker (lld) flag; it must be passed via
# TinyGo's -extldflags so TinyGo forwards it to lld rather than trying to
# interpret it as a TinyGo link flag (which would produce "flag not defined").
#
# Expands to nothing when the kernel ELF has not been built yet.
#
# --allow-multiple-definition is required because canal_idf_bridge.elf contains
# both IDF/newlib symbols (malloc, __errno_location, …) and unrenamed TinyGo
# target-support symbols (handleInterrupt, _vector_table, call_start_cpu0, …).
# When a domain is compiled with TinyGo it re-emits those same symbols; telling
# lld to allow duplicates and keep the first definition (the domain's own object
# files, which appear before --just-symbols in the link order) is the correct
# resolution.  FreeRTOS/IDF-only symbols (vTaskDelay, xQueueSend, …) are not
# defined in domain code, so they still resolve to the kernel's live addresses.
DOMAIN_LDFLAGS     = $(if $(wildcard $(KERNEL_ELF)),-ldflags="-extldflags='--just-symbols=$(KERNEL_ELF) --allow-multiple-definition'")

# Domains are linked into fixed flash windows derived from their partition offsets.
# This is required because TinyGo/xtensa output is not position-independent; the
# loader must later map flash at the same linked virtual addresses.
LED_DROM_ORIGIN      = 0x3C101000
LED_IROM_ORIGIN      = 0x42104000
WIFI_DROM_ORIGIN     = 0x3C181000
WIFI_IROM_ORIGIN     = 0x42185000
LOGGER_DROM_ORIGIN   = 0x3C201000
LOGGER_IROM_ORIGIN   = 0x42204000
PICOCECI_DROM_ORIGIN = 0x3C281000
PICOCECI_IROM_ORIGIN = 0x42285000
TLS_DROM_ORIGIN      = 0x3C380000
TLS_IROM_ORIGIN      = 0x42380000

# Generate a per-domain linker script + target JSON, then call tinygo build.
# $(1)=domain name, $(2)=DRAM origin hex, $(3)=DRAM length hex
# $(4)=DROM origin hex, $(5)=IROM origin hex
define build_domain_elf
	$(eval _ld := $(OUT_DIR)/$(1)-domain.ld)
	$(eval _json := $(OUT_DIR)/$(1)-target.json)
	@mkdir -p $(OUT_DIR)
	sed -e 's/DRAM (rw) : ORIGIN = 0x3FC88000, LENGTH = 416K/DRAM (rw) : ORIGIN = $(2), LENGTH = $(3)/' \
		-e 's/DROM (r)  : ORIGIN = 0x3C000000, LENGTH = 32M/DROM (r)  : ORIGIN = $(4), LENGTH = 32M/' \
		-e 's/IROM (rx) : ORIGIN = 0x42000000, LENGTH = 32M/IROM (rx) : ORIGIN = $(5), LENGTH = 32M/' \
		-e 's/ENTRY(call_start_cpu0)/ENTRY(domain_entry)/' \
		$(TINYGO_ROOT)/targets/esp32s3.ld > $(_ld)
	printf '{"inherits":["esp32s3-generic"],"linkerscript":"%s"}\n' "$(_ld)" > $(_json)
endef

# Auto-detect the first connected ESP32 USB-serial port.
# Override with: make flash PORT=/dev/cu.usbmodem1301
ifeq ($(shell uname),Darwin)
_DETECTED_PORT := $(firstword $(wildcard /dev/cu.usbmodem*))
else
_DETECTED_PORT := $(firstword $(wildcard /dev/ttyUSB* /dev/ttyACM*))
endif
PORT ?= $(_DETECTED_PORT)

ifeq ($(strip $(USE_IDF)),1)
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

.PHONY: all build build-domains led wifi logger tls \
        flash flash-domains flash-all flash-led flash-wifi flash-logger flash-tls flash-picoceci \
	flash-core-domains monitor run clean help

all: build

build: _check_tinygo
	@if [ "$(strip $(USE_IDF))" = "1" ]; then \
		echo "Building Canal kernel via ESP-IDF bridge..."; \
		$(MAKE) --no-print-directory -C $(BUILD_DIR) USE_IDF_FREERTOS=1 kernel; \
	else \
		echo "Building Canal kernel (target=$(TINYGO_TARGET))..."; \
		cd $(KERNEL_DIR) && $(TINYGO) build \
			-target=$(TINYGO_TARGET) \
			-opt=2 \
			$(EXTRA_TAGS) \
			-o ../build/out/kernel.elf \
			./cmd/esp32s3; \
	fi
	@echo "Built: build/out/kernel.elf"

build-domains: $(DOMAIN_STACK)

led: _check_tinygo
	@echo "Building LED domain..."
	$(call build_domain_elf,led,$(LED_RAM_ORIGIN),$(DOMAIN_RAM_SIZE),$(LED_DROM_ORIGIN),$(LED_IROM_ORIGIN))
	cd $(DOMAINS_DIR)/led-blinker && $(TINYGO) build \
		-target=$(OUT_DIR)/led-target.json \
		-opt=2 \
		$(EXTRA_TAGS) \
		$(DOMAIN_LDFLAGS) \
		-o $(OUT_DIR)/led.elf \
		.
	@echo "Built: build/out/led.elf"

wifi: _check_tinygo
	@echo "Building WiFi domain..."
	@if [ -z "$(WIFI_SSID)" ]; then \
		echo "Warning: WIFI_SSID not set. Set with: make wifi WIFI_SSID=your_ssid WIFI_PASSWORD=your_password"; \
	fi
	$(call build_domain_elf,wifi,$(WIFI_RAM_ORIGIN),$(DOMAIN_RAM_SIZE),$(WIFI_DROM_ORIGIN),$(WIFI_IROM_ORIGIN))
	cd $(DOMAINS_DIR)/wifi && $(TINYGO) build \
		-target=$(OUT_DIR)/wifi-target.json \
		-opt=2 \
		-gc=leaking \
		$(EXTRA_TAGS) \
		-ldflags="$(if $(wildcard $(KERNEL_ELF)),-extldflags='--just-symbols=$(KERNEL_ELF) --allow-multiple-definition' )-X main.wifiSSID=$(WIFI_SSID) -X main.wifiPassword=$(WIFI_PASSWORD) -X main.tcpPort=$(WIFI_PORT)" \
		-o $(OUT_DIR)/wifi.elf \
		./cmd/esp32s3
	@echo "Built: build/out/wifi.elf"

logger: _check_tinygo
	@echo "Building Logger domain..."
	$(call build_domain_elf,logger,$(LOGGER_RAM_ORIGIN),$(DOMAIN_RAM_SIZE),$(LOGGER_DROM_ORIGIN),$(LOGGER_IROM_ORIGIN))
	cd $(DOMAINS_DIR)/logger && $(TINYGO) build \
		-target=$(OUT_DIR)/logger-target.json \
		-opt=2 \
		$(EXTRA_TAGS) \
		$(DOMAIN_LDFLAGS) \
		-o $(OUT_DIR)/logger.elf \
		.
	@echo "Built: build/out/logger.elf"

tls: _check_tinygo
	@echo "Building TLS domain..."
	$(call build_domain_elf,tls,$(TLS_RAM_ORIGIN),$(DOMAIN_RAM_SIZE),$(TLS_DROM_ORIGIN),$(TLS_IROM_ORIGIN))
	cd $(DOMAINS_DIR)/tls && $(TINYGO) build \
		-target=$(OUT_DIR)/tls-target.json \
		-opt=2 \
		$(EXTRA_TAGS) \
		$(DOMAIN_LDFLAGS) \
		-o $(OUT_DIR)/tls.elf \
		.
	@echo "Built: build/out/tls.elf"

flash: _check_tinygo _check_port
	@if [ "$(strip $(USE_IDF))" = "1" ]; then \
		echo "Flashing kernel via ESP-IDF bridge to $(PORT)..."; \
		$(MAKE) --no-print-directory -C $(BUILD_DIR) USE_IDF_FREERTOS=1 flash PORT=$(PORT); \
	else \
		echo "Flashing to $(PORT)..."; \
		cd $(KERNEL_DIR) && $(TINYGO) flash \
			-target=$(TINYGO_TARGET) \
			-opt=2 \
			$(EXTRA_TAGS) \
			-port=$(PORT) \
			./cmd/esp32s3; \
	fi

flash-domains: _check_tinygo _check_port
	@echo "Building and flashing domains ($(DOMAIN_STACK)) to $(PORT)..."
	@if [ "$(ERASE_UNUSED)" = "1" ]; then \
		echo "Erasing unused domain partitions (ALL_DOMAINS=$(ALL_DOMAINS))..."; \
		for domain in $(ALL_DOMAINS); do \
			if [[ " $(DOMAIN_STACK) " == *" $$domain "* ]]; then \
				continue; \
			fi; \
			case "$$domain" in \
				led) addr=$(LED_ADDR) ;; \
				wifi) addr=$(WIFI_ADDR) ;; \
				logger) addr=$(LOGGER_ADDR) ;; \
				picoceci) addr=$(PICOCECI_ADDR) ;; \
				sdcard) addr=$(SDCARD_ADDR) ;; \
				tls) addr=$(TLS_ADDR) ;; \
				*) continue ;; \
			esac; \
			echo "  - erasing $$domain at $$addr size $(DOMAIN_PARTITION_SIZE)"; \
			esptool.py --chip esp32s3 --port $(PORT) erase_region $$addr $(DOMAIN_PARTITION_SIZE) || exit $$?; \
		done; \
	fi
	@for domain in $(DOMAIN_STACK); do \
		$(MAKE) --no-print-directory flash-$$domain PORT=$(PORT) || exit $$?; \
	done
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

flash-tls: _check_port tls
	@echo "Flashing TLS domain to $(PORT) at $(TLS_ADDR)..."
	esptool.py --chip esp32s3 --port $(PORT) --baud 921600 write_flash \
		$(TLS_ADDR) $(OUT_DIR)/tls.elf
	@echo "TLS domain flashed"

# Build and flash the main demo domains in sequence.
flash-core-domains: _check_tinygo _check_port
	@$(MAKE) --no-print-directory flash-domains PORT=$(PORT) DOMAIN_STACK="$(DOMAIN_STACK)"

monitor: _check_port
	@if [ "$(strip $(USE_IDF))" = "1" ]; then \
		echo "Monitoring via ESP-IDF bridge on $(PORT)..."; \
		$(MAKE) --no-print-directory -C $(BUILD_DIR) USE_IDF_FREERTOS=1 monitor PORT=$(PORT); \
	else \
		echo "Monitoring $(PORT) at $(BAUD) baud  (Ctrl-] to quit)"; \
		python3 -m serial.tools.miniterm --dtr 0 --rts 0 $(PORT) $(BAUD); \
	fi

# Flash kernel + domains, then immediately monitor.
# NOTE: tinygo flash (used by the 'flash' target) erases the entire chip, so
# domains must always be re-flashed after the kernel.  'run' does both steps
# in sequence so the device is in a consistent state before the monitor opens.
run: _check_tinygo _check_port
	@$(MAKE) --no-print-directory flash-all PORT=$(PORT)
	@echo "Waiting for device to re-enumerate..."
	@sleep 2
	@NEW_PORT=$$(ls /dev/cu.usbmodem* /dev/ttyUSB* /dev/ttyACM* 2>/dev/null | head -1); \
	echo "Monitoring $$NEW_PORT at $(BAUD) baud  (Ctrl-] to quit)"; \
	python3 -m serial.tools.miniterm --dtr 0 --rts 0 $$NEW_PORT $(BAUD)

clean:
	rm -rf build/out
	@echo "Cleaned build/out"

# ── picoceci domain ──────────────────────────────────────────────────────────

PICOCECI_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))domains/picoceci
DOMAIN_PARTITION_SIZE ?= 0x80000
LED_ADDR ?= 0x100000
WIFI_ADDR ?= 0x180000
LOGGER_ADDR ?= 0x200000
PICOCECI_ADDR ?= 0x280000
SDCARD_ADDR ?= 0x300000
TLS_ADDR ?= 0x380000

.PHONY: picoceci picoceci-flash picoceci-run

picoceci: _check_tinygo
	@echo "Building picoceci domain (target=$(TINYGO_TARGET))..."
	$(call build_domain_elf,picoceci,$(PICOCECI_RAM_ORIGIN),$(DOMAIN_RAM_SIZE),$(PICOCECI_DROM_ORIGIN),$(PICOCECI_IROM_ORIGIN))
	$(TINYGO) build \
		-target=$(OUT_DIR)/picoceci-target.json \
		-opt=2 \
		-gc=leaking \
		$(DOMAIN_LDFLAGS) \
		-o build/out/picoceci.elf \
		$(PICOCECI_DIR)
	@echo "Built: build/out/picoceci.elf"

picoceci-flash: _check_port picoceci
	@echo "Flashing picoceci domain to $(PORT) at $(PICOCECI_ADDR)..."
	esptool.py --chip esp32s3 --port $(PORT) --baud 921600 write_flash \
		$(PICOCECI_ADDR) $(OUT_DIR)/picoceci.elf
	@echo "picoceci domain flashed"

flash-picoceci: picoceci-flash

picoceci-run: _check_tinygo _check_port
	@$(MAKE) --no-print-directory picoceci-flash PORT=$(PORT)
	@echo "Waiting for device to re-enumerate..."
	@sleep 2
	@NEW_PORT=$$(ls /dev/cu.usbmodem* /dev/ttyUSB* /dev/ttyACM* 2>/dev/null | head -1); \
	echo "Monitoring $$NEW_PORT at $(BAUD) baud  (Ctrl-] to quit)"; \
	python3 -m serial.tools.miniterm --dtr 0 --rts 0 $$NEW_PORT $(BAUD)

help:
	@echo ""
	@echo "Canal build system"
	@echo ""
	@echo "Kernel targets:"
	@echo "  make build            compile kernel only"
	@echo "  make build-domains    compile all domains listed in DOMAIN_STACK"
	@echo "  make led/wifi/logger/tls compile a single domain"
	@echo ""
	@echo "  make flash            compile + flash kernel (auto-detects port)"
	@echo "  make flash-domains    build + flash domain stack in DOMAIN_STACK"
	@echo "                      (kernel must be built first -- run make flash before)"
	@echo "  make flash-all        flash kernel then all domains"
	@echo "  make flash-led        rebuild + flash LED domain only"
	@echo "  make flash-wifi       rebuild + flash WiFi domain only"
	@echo "  make flash-logger     rebuild + flash Logger domain only"
	@echo "  make flash-tls        rebuild + flash TLS domain only"
	@echo "  make flash-picoceci   rebuild + flash picoceci domain only"
	@echo "  make flash-core-domains alias for flash-domains"
	@echo ""
	@echo "  make monitor          open serial monitor"
	@echo "  make run              flash kernel + domains + monitor (full workflow)"
	@echo "  make clean            remove build artifacts"
	@echo ""
	@echo "Picoceci domain targets:"
	@echo "  make picoceci       build picoceci domain"
	@echo "  make picoceci-flash flash picoceci to device"
	@echo "  make picoceci-run   flash + monitor picoceci"
	@echo ""
	@echo "  scripts/flash.sh [PORT] [--domains-only] [--led-only]"
	@echo ""
	@echo "Variables (override on command line):"
	@echo "  PORT=$(PORT)"
	@echo "  TINYGO_TARGET=$(TINYGO_TARGET)"
	@echo "  USE_IDF=$(USE_IDF)   (default 1: link real FreeRTOS via ESP-IDF for flash domain loading)"
	@echo "                      (set to 0 to disable IDF — goroutine fallbacks only, no flash loading)"
	@echo "  BAUD=$(BAUD)"
	@echo "  PICOCECI_ADDR=$(PICOCECI_ADDR)"
	@echo "  TLS_ADDR=$(TLS_ADDR)"
	@echo "  DOMAIN_STACK=$(DOMAIN_STACK)"
	@echo ""
