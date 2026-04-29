# ChannelOS ESP32-S3 Build System

TINYGO := tinygo
TARGET := esp32s3
IDF_PATH := $(HOME)/esp/esp-idf

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

all: kernel wifi http led flash

kernel:
	@echo "Building kernel..."
	$(TINYGO) build \
		-target=$(TARGET) \
		-opt=2 \
		-o build/kernel.elf \
		$(KERNEL_SRC)

wifi:
	@echo "Building WiFi domain..."
	$(TINYGO) build \
		-target=$(TARGET) \
		-opt=2 \
		-o build/wifi.elf \
		$(WIFI_SRC)

http:
	@echo "Building HTTP server domain..."
	$(TINYGO) build \
		-target=$(TARGET) \
		-opt=2 \
		-o build/http-server.elf \
		$(HTTP_SRC)

led:
	@echo "Building LED domain..."
	$(TINYGO) build \
		-target=$(TARGET) \
		-opt=2 \
		-o build/led-blinker.elf \
		$(LED_SRC)

flash: all
	@echo "Flashing to ESP32-S3..."
	esptool.py \
		--chip esp32s3 \
		--port /dev/ttyUSB0 \
		--baud 921600 \
		write_flash \
		0x10000 build/kernel.bin \
		0x100000 build/wifi.bin \
		0x180000 build/http-server.bin \
		0x200000 build/led-blinker.bin

monitor:
	python -m serial.tools.miniterm /dev/ttyUSB0 115200

clean:
	rm -rf build/*.elf build/*.bin

.PHONY: all kernel wifi http led flash monitor clean
