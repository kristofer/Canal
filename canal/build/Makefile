# Canal ESP32-S3 build system (internal — prefer the top-level Makefile)
# Run from the repo root: cd .. && make flash

SHELL := /bin/bash

TINYGO        ?= tinygo
TINYGO_TARGET ?= esp32s3-generic
USE_IDF_FREERTOS ?= 0

ifeq ($(shell uname),Darwin)
PORT ?= $(firstword $(wildcard /dev/cu.usbmodem*))
else
PORT ?= $(firstword $(wildcard /dev/ttyUSB* /dev/ttyACM*))
endif

BAUD ?= 115200
OUT_DIR := out

ifeq ($(USE_IDF_FREERTOS),1)
EXTRA_TAGS  := -tags=idf
IDF_PATH    := $(HOME)/esp/esp-idf
IDF_EXPORT  := source $(IDF_PATH)/export.sh >/dev/null 2>&1 &&
else
EXTRA_TAGS  :=
IDF_EXPORT  :=
endif

all: kernel

$(OUT_DIR):
	mkdir -p $(OUT_DIR)

kernel: $(OUT_DIR)
ifeq ($(USE_IDF_FREERTOS),1)
	@echo "Building kernel via ESP-IDF..."
	cd idf-app && $(IDF_EXPORT) idf.py build
	cp -f idf-app/build/canal_idf_bridge.elf $(OUT_DIR)/kernel.elf
	cp -f idf-app/build/canal_idf_bridge.bin $(OUT_DIR)/kernel.bin
else
	@echo "Building kernel (baremetal TinyGo)..."
	cd ../kernel && $(TINYGO) build \
		-target=$(TINYGO_TARGET) \
		-opt=2 \
		$(EXTRA_TAGS) \
		-o ../build/$(OUT_DIR)/kernel.elf \
		./cmd/esp32s3
endif

flash: kernel
	@[ -n "$(PORT)" ] || { echo "error: no port found"; exit 1; }
ifeq ($(USE_IDF_FREERTOS),1)
	cd idf-app && $(IDF_EXPORT) idf.py -p $(PORT) flash
else
	@echo "Flashing to $(PORT)..."
	cd ../kernel && $(TINYGO) flash \
		-target=$(TINYGO_TARGET) \
		$(EXTRA_TAGS) \
		-port=$(PORT) \
		./cmd/esp32s3
endif

monitor:
	@[ -n "$(PORT)" ] || { echo "error: no port found"; exit 1; }
ifeq ($(USE_IDF_FREERTOS),1)
	cd idf-app && $(IDF_EXPORT) idf.py -p $(PORT) monitor
else
	python3 -m serial.tools.miniterm --dtr 0 --rts 0 $(PORT) $(BAUD)
endif

clean:
	rm -rf $(OUT_DIR)

.PHONY: all kernel flash monitor clean
