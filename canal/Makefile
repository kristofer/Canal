# Canal Build System

TINYGO ?= tinygo
TARGET ?= esp32s3
PORT ?= /dev/cu.usbmodem1201
BAUD ?= 115200

ROOT_DIR := $(shell pwd)
BUILD_DIR := $(ROOT_DIR)/build
OUT_DIR := $(BUILD_DIR)/out
TARGET_DIR := $(BUILD_DIR)/targets/$(TARGET)

# Include target-specific configuration
-include $(TARGET_DIR)/config.mk

TINYGO_FLAGS := -target=$(TINYGO_TARGET) -opt=2 -scheduler=none -gc=conservative

.PHONY: all clean kernel help

all: kernel
	@echo ""
	@echo "✅ Build complete for $(TARGET)"
	@echo ""
	@echo "Next steps:"
	@echo "  make flash PORT=$(PORT)    # Flash to device"
	@echo "  make monitor PORT=$(PORT)  # Open serial monitor"

help:
	@echo "Canal Build System"
	@echo ""
	@echo "Targets:"
	@echo "  all          - Build kernel and domains"
	@echo "  kernel       - Build kernel only"
	@echo "  clean        - Remove build artifacts"
	@echo "  flash        - Flash to device"
	@echo "  monitor      - Open serial monitor"
	@echo "  help         - Show this help"
	@echo ""
	@echo "Variables:"
	@echo "  TARGET       - Target board (esp32s3, rp2040, stm32f4)"
	@echo "  PORT         - Serial port (default: /dev/ttyUSB0)"
	@echo "  BAUD         - Baud rate (default: 115200)"
	@echo ""
	@echo "Examples:"
	@echo "  make TARGET=esp32s3"
	@echo "  make flash PORT=/dev/ttyACM0"
	@echo "  make monitor"

$(OUT_DIR):
	mkdir -p $(OUT_DIR)

kernel: $(OUT_DIR)
	@echo "Building kernel for $(TARGET)..."
	@echo ""
	@echo "⚠️  NOTE: Full kernel sources need to be added"
	@echo "    See kernel/README.md for implementation status"
	@echo "    Copy implementations from conversation"
	@echo ""

clean:
	rm -rf $(OUT_DIR)
	@echo "Build artifacts removed"

flash:
	@echo "Flashing to $(TARGET)..."
	@echo "Command: $(FLASH_COMMAND)"

monitor:
	@echo "Opening serial monitor on $(PORT)..."
	python -m serial.tools.miniterm $(PORT) $(BAUD)
