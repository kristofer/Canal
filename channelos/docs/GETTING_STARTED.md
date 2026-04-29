# Getting Started

## Prerequisites

1. **TinyGo 0.31.0+**
```bash
   wget https://github.com/tinygo-org/tinygo/releases/download/v0.31.0/tinygo_0.31.0_amd64.deb
   sudo dpkg -i tinygo_0.31.0_amd64.deb
```

2. **ESP-IDF** (for ESP32 targets)
```bash
   git clone --recursive https://github.com/espressif/esp-idf.git ~/esp/esp-idf
   cd ~/esp/esp-idf
   ./install.sh esp32s3
   . ./export.sh
```

3. **Python tools**
```bash
   pip install esptool pyserial
```

## Quick Start

```bash
# Clone
git clone https://github.com/yourusername/channelos.git
cd channelos

# Setup
chmod +x scripts/setup.sh
./scripts/setup.sh esp32s3

# Build
make TARGET=esp32s3

# Flash
make flash PORT=/dev/ttyUSB0

# Monitor
make monitor PORT=/dev/ttyUSB0
```

## Creating a Domain

1. Create directory: `mkdir domains/mydomain`
2. Create `main.go`:
```go
   package main

   func main() {
       println("My domain!")
   }
```
3. Add to Makefile
4. Build and flash

## Next Steps

- Read [ARCHITECTURE.md](ARCHITECTURE.md)
- Explore [examples/](../examples/)
- Review [CAPABILITIES.md](CAPABILITIES.md)
