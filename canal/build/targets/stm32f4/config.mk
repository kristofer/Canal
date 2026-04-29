# STM32F4 Configuration

ARCH := arm
TINYGO_TARGET := stm32f4disco

FLASH_COMMAND := openocd -f board/stm32f4discovery.cfg -c "program $(OUT_DIR)/kernel.elf verify reset exit"
