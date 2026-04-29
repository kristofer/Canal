# Canal

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![TinyGo](https://img.shields.io/badge/TinyGo-0.31+-blue.svg)](https://tinygo.org/)

A capability-based microkernel operating system for embedded systems, written in Go (TinyGo).

## 🚀 Features

- **Capability-based security**: Fine-grained access control through unforgeable tokens
- **Domain isolation**: Memory-protected execution environments (ARM MPU, ESP32 MMU, RISC-V PMP)
- **Go-native**: Write OS services and applications in Go
- **FreeRTOS integration**: Built on proven RTOS primitives
- **Channel-based IPC**: Go channels as universal communication
- **Multi-architecture**: ARM Cortex-M, ESP32 (Xtensa/RISC-V), RP2040

## 📋 Supported Hardware

| Board | Architecture | Memory | Status |
|-------|-------------|--------|--------|
| ESP32-S3 | Xtensa LX7 | 512KB RAM, 8MB Flash | ✅ Primary |
| ESP32-C6 | RISC-V | 512KB RAM, 4MB Flash | 🚧 In Progress |
| Raspberry Pi Pico | ARM Cortex-M0+ | 264KB RAM, 2MB Flash | ✅ Supported |
| STM32F4 | ARM Cortex-M4 | 192KB RAM, 1MB Flash | 🚧 Planned |

## 🏃 Quick Start

### Prerequisites

```bash
# Install TinyGo 0.31.0+
wget https://github.com/tinygo-org/tinygo/releases/download/v0.31.0/tinygo_0.31.0_amd64.deb
sudo dpkg -i tinygo_0.31.0_amd64.deb

# For ESP32: Install ESP-IDF
git clone --recursive https://github.com/espressif/esp-idf.git ~/esp/esp-idf
cd ~/esp/esp-idf
./install.sh esp32s3
. ./export.sh

# Install Python tools
pip install esptool pyserial
```

### Build & Flash

```bash
# Clone repository
git clone https://github.com/yourusername/canal.git
cd canal

# Setup (downloads FatFS, mbedTLS)
chmod +x scripts/setup.sh
./scripts/setup.sh esp32s3

# Build for ESP32-S3
make TARGET=esp32s3

# Flash to device
make flash PORT=/dev/ttyUSB0

# Monitor output
make monitor PORT=/dev/ttyUSB0
```

Expected output:
=== Canal ESP32-S3 ===
Boot time: 342 ms
MMU initialized
Capability table ready
Domain table ready
Syscall handler ready
Loading domains...
WiFi domain: 1
TLS domain: 2
SDCard domain: 3
HTTP domain: 4
=== Boot Complete ===

## 🏗️ Architecture
┌─────────────────────────────────────────┐
│         User Domains                    │
│  HTTP Server │ Logger │ Applications   │
└────────┬─────────┬──────────┬──────────┘
│         │          │
Capability Channels (IPC)
│         │          │
┌────────┴─────────┴──────────┴──────────┐
│      System Service Domains            │
│  WiFi │ TLS │ Filesystem │ GPIO       │
└────────┬─────────┬──────────┬──────────┘
│         │          │
┌────────┴─────────┴──────────┴──────────┐
│         Kernel Substrate               │
│  Capability Table │ Domain Manager     │
│  MMU/MPU Config  │ Syscall Handler    │
└────────┬─────────┴────────────────────┘
│
┌────────┴──────────────────────────────┐
│         Hardware Layer                │
│  WiFi │ Flash │ GPIO │ SPI │ I2C     │
└───────────────────────────────────────┘
## 📚 Documentation

- **[Architecture](docs/ARCHITECTURE.md)** - System design and components
- **[Getting Started](docs/GETTING_STARTED.md)** - Build and deployment guide
- **[Capabilities](docs/CAPABILITIES.md)** - Security model explained
- **[Porting Guide](docs/PORTING.md)** - Add new hardware support
- **[Examples](docs/EXAMPLES.md)** - Sample applications

## 💡 Example Code

### Simple HTTP Server with TLS

```go
package main

import (
    "stdlib/wifi"
    "stdlib/tls"
)

func main() {
    // Connect to WiFi (isolated in WiFi domain)
    wifi.Init()
    wifi.Connect("MyNetwork", "password", 30000)

    // Load certificate (keys never leave TLS domain)
    cert, _ := tls.LoadCertificate("server", certPEM)
    key, _ := tls.LoadPrivateKey("server", keyPEM, nil)

    // Listen on HTTPS
    listener, _ := tls.Listen(":443", cert, key)

    // Handle connections
    for {
        conn, _ := listener.Accept()
        go handleRequest(conn)
    }
}
```

### File System with Capabilities

```go
package main

import "stdlib/fs"

func main() {
    // Open file (SD card domain checks permissions)
    file, _ := fs.Open("/logs/system.log")
    defer file.Close()

    // Read/write operations
    data := make([]byte, 1024)
    n, _ := file.Read(data)

    // File operations isolated to filesystem domain
}
```

## 🔒 Security Model

Canal implements **defense-in-depth** security:

1. **Hardware Isolation**: MMU/MPU prevents cross-domain memory access
2. **Capability System**: Unforgeable tokens control all resource access
3. **Least Privilege**: Domains request only needed capabilities
4. **API Reduction**: No direct hardware access from user code
5. **Crash Isolation**: Domain failures don't propagate

### Security Example
# HTTP Server Domain compromised by buffer overflow:
✗ Cannot access WiFi hardware (no device capability)
✗ Cannot read TLS private keys (different domain)
✗ Cannot access other domains' memory (MMU protection)
✗ Cannot interfere with Logger domain (isolated)
✓ Can only send network requests (has service:wifi capability)
→ Attack surface drastically limited

## 🛠️ Project Structure
canal/
├── kernel/          # Microkernel substrate
│   ├── captable.go  # Capability management
│   ├── domain.go    # Domain lifecycle
│   ├── syscall.go   # System call handler
│   └── hal/         # Hardware abstraction layer
├── runtime/         # TinyGo runtime extensions
│   ├── chan.go      # Channel implementation
│   ├── gc.go        # Garbage collector
│   └── cap.go       # Capability API
├── stdlib/          # Standard library
│   ├── fs/          # Filesystem API
│   ├── net/         # Networking API
│   ├── wifi/        # WiFi API
│   └── tls/         # TLS API
├── domains/         # System service domains
│   ├── wifi/        # WiFi service
│   ├── tls/         # TLS/crypto service
│   ├── sdcard/      # Filesystem service
│   └── ...
└── examples/        # Example applications

# ## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Code style guidelines
- Testing requirements
- Pull request process
- Development workflow

## 📊 Performance

| Metric | Value |
|--------|-------|
| Context switch (ARM) | ~10 cycles |
| Context switch (ESP32) | ~100 cycles |
| Channel send (same-domain) | ~50 cycles |
| Channel send (cross-domain) | ~300 cycles |
| Memory per domain | 36KB minimum |
| Kernel overhead | 20KB |

## 🔍 Comparison

| Feature | Canal | Linux | FreeRTOS | seL4 |
|---------|-----------|-------|----------|------|
| Memory isolation | ✅ | ✅ | ❌ | ✅ |
| Capabilities | ✅ | ❌ | ❌ | ✅ |
| Go support | ✅ | ✅ | ❌ | ❌ |
| Embedded focus | ✅ | ❌ | ✅ | ✅ |
| Formal verification | ❌ | ❌ | ❌ | ✅ |

## 📝 Status

- ✅ Kernel substrate (v0.1.0)
- ✅ ARM MPU support
- ✅ ESP32-S3 MMU support
- ✅ WiFi domain
- ✅ TLS domain
- ✅ Filesystem domain
- 🚧 Multi-core support
- 🚧 RISC-V PMP support
- 📋 Formal verification (planned)

## 🙏 Acknowledgments

- Built with [TinyGo](https://tinygo.org/)
- Uses [FreeRTOS](https://www.freertos.org/)
- Inspired by [seL4](https://sel4.systems/), [Fuchsia](https://fuchsia.dev/)
- Capability concepts from [EROS](http://www.eros-os.org/), [Coyotos](http://www.coyotos.org/)

## 📄 License

MIT License - see [LICENSE](LICENSE) for details

## 📧 Contact

- **Issues**: [GitHub Issues](https://github.com/yourusername/canal/issues)
- **Discussions**: [GitHub Discussions](https://github.com/yourusername/canal/discussions)
- **Email**: canal-dev@example.com

---

**Note**: This is an early-stage project. APIs may change. Production use not recommended yet.

⭐ Star us on GitHub if you find this interesting!
