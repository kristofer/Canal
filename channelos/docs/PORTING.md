# Porting Guide

To add support for new hardware:

## 1. Create Target Directory

```bash
mkdir -p build/targets/myboard
```

## 2. Create config.mk

```makefile
# build/targets/myboard/config.mk

ARCH := arm
TINYGO_TARGET := myboard

FLASH_COMMAND := flash-tool --board myboard $(OUT_DIR)/kernel.bin
MONITOR_COMMAND := serial-monitor /dev/ttyUSB0 115200
```

## 3. Implement HAL

Create `kernel/hal/myarch/protection.go`:

```go
package myarch

import "kernel/hal"

type myProtection struct {
    // Architecture-specific fields
}

func newMemoryProtection() hal.MemoryProtection {
    return &myProtection{}
}

func (m *myProtection) Init() error {
    // Initialize hardware
}

// Implement other hal.MemoryProtection methods
```

## 4. Create Boot Code

`kernel/boot_myboard.go`:

```go
//go:build tinygo && myboard

package kernel

func main() {
    // Initialize
    // Load domains
    // Start scheduler
}
```

## 5. Test

```bash
make TARGET=myboard
make flash PORT=/dev/ttyUSB0
```

## 6. Submit PR

Include:
- HAL implementation
- Boot code
- Build configuration
- Documentation
