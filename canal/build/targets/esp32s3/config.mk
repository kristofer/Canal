# ESP32-S3 Target Configuration

ARCH := xtensa
TINYGO_TARGET := esp32s3

# Flash partition addresses (must match partitions.csv).
# Domain partitions are 512KB each to fit full TinyGo runtime binaries.
KERNEL_ADDR  := 0x10000
LED_ADDR     := 0x100000
WIFI_ADDR    := 0x180000
LOGGER_ADDR  := 0x200000
TLS_ADDR     := 0x280000
SDCARD_ADDR  := 0x300000

FLASH_COMMAND := esptool \
	--chip esp32s3 \
	--port $(PORT) \
	--baud 921600 \
	write_flash \
	$(KERNEL_ADDR)  $(OUT_DIR)/kernel.bin \
	$(LED_ADDR)     $(OUT_DIR)/led.elf \
	$(WIFI_ADDR)    $(OUT_DIR)/wifi.elf \
	$(LOGGER_ADDR)  $(OUT_DIR)/logger.elf
