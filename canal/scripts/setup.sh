#!/bin/bash
# Canal Setup Script

set -e

echo "╔════════════════════════════════════════╗"
echo "║     Canal Setup Script v1.0        ║"
echo "╚════════════════════════════════════════╝"
echo ""

# Check TinyGo
if ! command -v tinygo &> /dev/null; then
    echo "❌ TinyGo not found"
    echo ""
    echo "Install TinyGo 0.31.0+:"
    echo "  wget https://github.com/tinygo-org/tinygo/releases/download/v0.31.0/tinygo_0.31.0_amd64.deb"
    echo "  sudo dpkg -i tinygo_0.31.0_amd64.deb"
    exit 1
fi

echo "✅ TinyGo found: $(tinygo version)"

TARGET=${1:-esp32s3}

case $TARGET in
    esp32s3|esp32c6)
        echo ""
        echo "Setting up ESP32 environment..."

        if ! command -v esptool.py &> /dev/null; then
            echo "Installing esptool..."
            pip install esptool pyserial
        fi

        echo "✅ ESP32 tools ready"
        ;;

    rp2040)
        echo ""
        echo "✅ RP2040 ready (built-in TinyGo support)"
        ;;

    stm32f4)
        echo ""
        if ! command -v arm-none-eabi-gcc &> /dev/null; then
            echo "❌ ARM toolchain not found"
            echo "   Install: sudo apt install gcc-arm-none-eabi"
            exit 1
        fi
        echo "✅ STM32 toolchain ready"
        ;;

    *)
        echo "❌ Unknown target: $TARGET"
        echo "   Supported: esp32s3, esp32c6, rp2040, stm32f4"
        exit 1
        ;;
esac

# Download dependencies
echo ""
echo "Downloading third-party dependencies..."

mkdir -p third_party

# FatFS
if [ ! -d "third_party/fatfs" ]; then
    echo "  Downloading FatFS..."
    echo "  (Placeholder - add download command)"
    mkdir -p third_party/fatfs
    echo "  ✅ FatFS prepared"
fi

# mbedTLS
if [ ! -d "third_party/mbedtls" ]; then
    echo "  Downloading mbedTLS..."
    echo "  (Placeholder - add download command)"
    mkdir -p third_party/mbedtls
    echo "  ✅ mbedTLS prepared"
fi

echo ""
echo "╔════════════════════════════════════════╗"
echo "║        Setup Complete!                 ║"
echo "╚════════════════════════════════════════╝"
echo ""
echo "Next steps:"
echo "  1. make TARGET=$TARGET"
echo "  2. make flash PORT=/dev/ttyUSB0"
echo "  3. make monitor"
echo ""
