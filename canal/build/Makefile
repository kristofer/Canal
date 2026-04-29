# Canal ESP32-S3 Build System

SHELL := /bin/bash

TINYGO := tinygo
TARGET := esp32s3
TINYGO_TARGET ?= esp32s3-generic
IDF_PATH := $(HOME)/esp/esp-idf
OUT_DIR := out
PORT ?= /dev/cu.usbmodem1101
BAUD ?= 921600
USE_IDF_FREERTOS ?= 0

ifeq ($(USE_IDF_FREERTOS),1)
EXTRA_TAGS := -tags=idf
else
EXTRA_TAGS :=
endif

IDF_EXPORT := source $(IDF_PATH)/export.sh >/dev/null 2>&1

# Kernel
KERNEL_SRC := kernel/boot_esp32s3.go \
              kernel/captable.go \
              kernel/domain.go \
              kernel/syscall.go \
              kernel/hal/xtensa/mmu.go

# Domains
WIFI_SRC := domains/wifi/main.go \
            domains/wifi/driver.go \
            domains/wifi/protocol.go

HTTP_SRC := domains/http-server/main.go

LED_SRC := domains/led-blinker/main.go

all: kernel

$(OUT_DIR):
	mkdir -p $(OUT_DIR)

# kernel:
# 	@echo "Building kernel..."
# 	$(TINYGO) build \
# 		-target=$(TARGET) \
# 		-opt=2 \
# 		-o build/kernel.elf \
# 		$(KERNEL_SRC)

kernel: $(OUT_DIR)

ifeq ($(USE_IDF_FREERTOS),1)
	@echo "Building kernel via ESP-IDF component path..."
	cd idf-app && $(IDF_EXPORT) && idf.py build
	cp -f idf-app/build/canal_idf_bridge.elf $(OUT_DIR)/kernel.elf
	cp -f idf-app/build/canal_idf_bridge.bin $(OUT_DIR)/kernel.bin
else
	@echo "Building kernel..."
	cd ../kernel && $(IDF_EXPORT) && $(TINYGO) build \
		-target=$(TINYGO_TARGET) \
		-opt=2 \
		$(EXTRA_TAGS) \
		-o ../build/$(OUT_DIR)/kernel.elf \
		./cmd/esp32s3
	cd ../kernel && $(IDF_EXPORT) && $(TINYGO) build \
		-target=$(TINYGO_TARGET) \
		-opt=2 \
		$(EXTRA_TAGS) \
		-o ../build/$(OUT_DIR)/kernel.bin \
		./cmd/esp32s3
endif

wifi: $(OUT_DIR)
	@echo "Building WiFi domain..."
	cd ../domains/wifi && $(IDF_EXPORT) && $(TINYGO) build \
		-target=$(TINYGO_TARGET) \
		-opt=2 \
		-o ../../build/$(OUT_DIR)/wifi.elf \
		./cmd/esp32s3

http: $(OUT_DIR)
	@echo "Building HTTP server domain..."
	cd ../domains/http-server && $(IDF_EXPORT) && $(TINYGO) build \
		-target=$(TINYGO_TARGET) \
		-opt=2 \
		-o ../../build/$(OUT_DIR)/http-server.elf \
		.

led: $(OUT_DIR)
	@echo "Building LED domain..."
	cd ../domains/led-blinker && $(IDF_EXPORT) && $(TINYGO) build \
		-target=$(TINYGO_TARGET) \
		-opt=2 \
		-o ../../build/$(OUT_DIR)/led-blinker.elf \
		.

flash: kernel

ifeq ($(USE_IDF_FREERTOS),1)
	@echo "Flashing kernel via ESP-IDF..."
	cd idf-app && $(IDF_EXPORT) && idf.py -p $(PORT) flash
else
	@echo "Flashing kernel to ESP32-S3 with TinyGo..."
	cd ../kernel && $(IDF_EXPORT) && $(TINYGO) flash \
		-target=$(TINYGO_TARGET) \
		$(EXTRA_TAGS) \
		-port=$(PORT) \
		./cmd/esp32s3
endif

monitor:

ifeq ($(USE_IDF_FREERTOS),1)
	cd idf-app && $(IDF_EXPORT) && idf.py -p $(PORT) monitor
else
	$(IDF_EXPORT) && $(TINYGO) monitor -target=$(TINYGO_TARGET) -port=$(PORT)
endif

clean:
	rm -rf $(OUT_DIR)

.PHONY: all kernel wifi http led flash monitor clean
