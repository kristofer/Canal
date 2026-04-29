# Raspberry Pi Pico Configuration

ARCH := arm
TINYGO_TARGET := pico

FLASH_COMMAND := picotool load $(OUT_DIR)/kernel.uf2
