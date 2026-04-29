# ESP32-S3 Target Configuration

ARCH := xtensa
TINYGO_TARGET := esp32s3

# Flash addresses
KERNEL_ADDR := 0x10000
WIFI_ADDR := 0x100000
TLS_ADDR := 0x180000
SDCARD_ADDR := 0x200000
HTTP_ADDR := 0x280000
LOGGER_ADDR := 0x300000

FLASH_COMMAND := esptool.py \
	--chip esp32s3 \
	--port $(PORT) \
	--baud 921600 \
	write_flash \
	$(KERNEL_ADDR) $(OUT_DIR)/kernel.bin
