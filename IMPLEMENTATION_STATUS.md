# TCP Network Interpreter - Implementation Status

## ✅ Validated Working State (2026-05-08)

The WiFi domain is now booting reliably from flash, bringing up TCP, and running picoceci over a network client. Multi-line paste mode is confirmed working in live runs.

### Confirmed Working

- WiFi domain loads from flash and starts successfully
- WiFi connects and obtains DHCP address
- TCP listener accepts client connections
- picoceci REPL runs over TCP
- Pasted multi-line programs execute successfully

### Fixes That Unblocked Stability

- Transcript sink wiring for picoceci VM:
  - WiFi interpreter uses `bytecode.NewVMWithTranscript(console)` so Transcript output is routed to the TCP client writer.
- WiFi domain XIP mapping correction:
  - `WIFI_IROM_ORIGIN` adjusted to `0x42185000` in `canal/Makefile` so linked virtual offsets and flash file offsets map correctly at runtime.
- Kernel boot stack alignment:
  - Default kernel domain stack reduced to `led,wifi` in `kernel/boot_esp32s3.go` to avoid noisy attempts to load domains not in default flash workflow.
- Early startup hardening for TinyGo runtime:
  - WiFi domain now bootstraps a minimal DRAM heap at entry before early startup/logging paths.
  - Later allocation path can move to PSRAM.
- IP wait-loop short-circuit:
  - Kernel now retains STA netif pointer and exports it to domains.
  - WiFi domain polls `esp_netif_get_ip_info` and exits wait loop as soon as IP is assigned.

### Integration Notes

- `go.mod` keeps a local replace for picoceci (`=> ../../picoceci`), so runtime behavior follows the local picoceci checkout.
- Kernel and domain symbol sharing still relies on `--just-symbols` and force-retained IDF symbols in `build/idf-app/CMakeLists.txt`.

## ✅ What's Been Completed

### 1. Build System Integration

- **Modified**: [Makefile](canal/Makefile#L14-L20) - Added `WIFI_SSID`, `WIFI_PASSWORD`, `WIFI_PORT` variables
- **Modified**: [Makefile](canal/Makefile#L177-L185) - WiFi domain build passes credentials via `-X` ldflags
- **Modified**: [build/idf-app/CMakeLists.txt](canal/build/idf-app/CMakeLists.txt#L10-L24) - Force-linked WiFi and LwIP symbols

### 2. Kernel WiFi/Network Support

- **Modified**: [kernel/freertos_esp32s3_idf.go](canal/kernel/freertos_esp32s3_idf.go#L124-L174) - Added WiFi and LwIP extern declarations
- ESP-IDF WiFi functions: `esp_wifi_init`, `esp_wifi_set_mode`, `esp_wifi_set_config`, `esp_wifi_start`, `esp_wifi_connect`
- LwIP socket functions: `lwip_socket`, `lwip_bind`, `lwip_listen`, `lwip_accept`, `lwip_recv`, `lwip_send`, `lwip_close`, `lwip_setsockopt`, `lwip_fcntl`
- USB Serial JTAG: `usb_serial_jtag_write_bytes`, `usb_serial_jtag_wait_tx_done`

### 3. WiFi Domain Implementation

- **Created**: [domains/wifi/cmd/esp32s3/esp_idf.go](canal/domains/wifi/cmd/esp32s3/esp_idf.go) - ESP-IDF API bindings
- **Created**: [domains/wifi/cmd/esp32s3/network.go](canal/domains/wifi/cmd/esp32s3/network.go) - WiFi connection and TCP server logic
- **Created**: [domains/wifi/cmd/esp32s3/interpreter.go](canal/domains/wifi/cmd/esp32s3/interpreter.go) - picoceci REPL implementation
- **Created**: [domains/wifi/cmd/esp32s3/console.go](canal/domains/wifi/cmd/esp32s3/console.go) - Network console abstraction
- **Modified**: [domains/wifi/cmd/esp32s3/main.go](canal/domains/wifi/cmd/esp32s3/main.go) - Main orchestration

### 4. Architecture

```
┌─────────────────────────────────────────┐
│   Kernel (canal_idf_bridge.elf)        │
│   - ESP-IDF WiFi stack                  │
│   - LwIP TCP/IP stack                   │
│   - Symbol exports to domains           │
└─────────────────────────────────────────┘
                  │
                  │ --just-symbols link
                  ↓
┌─────────────────────────────────────────┐
│   WiFi Domain (wifi.elf)                │
│   ┌───────────────────────────────────┐ │
│   │ main.go - Orchestration           │ │
│   │  ├─ connectToWiFi()               │ │
│   │  ├─ createTCPServer()             │ │
│   │  └─ acceptTCPClient() loop        │ │
│   ├───────────────────────────────────┤ │
│   │ network.go - WiFi & TCP           │ │
│   │  ├─ WiFi connection               │ │
│   │  ├─ TCP server socket             │ │
│   │  └─ Client accept                 │ │
│   ├───────────────────────────────────┤ │
│   │ console.go - Network I/O          │ │
│   │  ├─ tcpConsole struct             │ │
│   │  ├─ ReadLine() over TCP           │ │
│   │  └─ Write() over TCP              │ │
│   ├───────────────────────────────────┤ │
│   │ interpreter.go - picoceci REPL    │ │
│   │  ├─ runREPL()                     │ │
│   │  ├─ evalREPLSource()              │ │
│   │  └─ Paste mode support            │ │
│   └───────────────────────────────────┘ │
└─────────────────────────────────────────┘
                  │
                  │ TCP/IP
                  ↓
┌─────────────────────────────────────────┐
│   Network Client (nc (netcat))            │
│   - Interactive picoceci REPL           │
│   - Full line editing                   │
│   - Multi-line paste mode               │
└─────────────────────────────────────────┘
```

## ✅ Current Status: WORKING ON HARDWARE

The end-to-end TCP interpreter path is now working on-device.

### What's Working

- ✅ Kernel initializes ESP-IDF WiFi stack at boot (see [kernel/freertos_esp32s3_idf.go](canal/kernel/freertos_esp32s3_idf.go#L177-L208))
- ✅ Network interface (netif) created via `esp_netif_init()`
- ✅ Event loop initialized via `esp_event_loop_create_default()`
- ✅ WiFi station interface created via `esp_netif_create_default_wifi_sta()`
- ✅ All symbols properly force-linked in CMakeLists.txt
- ✅ WiFi domain can call WiFi/socket functions

### Remaining Follow-ups

- Add event-driven connect completion (instead of periodic polling) if desired
- Add early boot log ring-buffer path for allocation-free diagnostics
- Add regression test checklist for domain XIP link-origin consistency

## 🔧 Next Steps

### Option 1: Move to Event-Driven IP Ready

Replace the polling short-circuit with an explicit event-group signal from `IP_EVENT_STA_GOT_IP` for faster and cleaner connect readiness.

### Option 2: Full Implementation

1. Add proper WiFi event handling
2. Implement DHCP client
3. Add mDNS for hostname discovery
4. Add connection retry logic
5. Add proper error handling throughout

### Option 3: Use Existing ESP-IDF Example

Study ESP-IDF's [station example](https://github.com/espressif/esp-idf/tree/master/examples/wifi/getting_started/station) and port initialization sequence to kernel.

## 📝 Build & Test Commands

```bash
# Clean build kernel with WiFi support
rm -rf build/idf-app/build && make build

# Build and flash WiFi domain
make flash-wifi WIFI_SSID=YourNetwork WIFI_PASSWORD=YourPassword PORT=/dev/cu.usbmodem11201

# Monitor serial output
make monitor PORT=/dev/cu.usbmodem11201

# Once working, connect from another terminal
nc <device-ip> 2323
```

## 📦 Files Modified/Created

### Modified

- `canal/Makefile` - Build system integration
- `canal/build/idf-app/CMakeLists.txt` - Symbol exports
- `canal/kernel/freertos_esp32s3_idf.go` - API exports
- `canal/domains/wifi/cmd/esp32s3/main.go` - Orchestration

### Created

- `canal/domains/wifi/cmd/esp32s3/esp_idf.go` - ESP-IDF bindings (144 lines)
- `canal/domains/wifi/cmd/esp32s3/network.go` - Networking layer (134 lines)
- `canal/domains/wifi/cmd/esp32s3/console.go` - TCP console (133 lines)
- `canal/domains/wifi/cmd/esp32s3/interpreter.go` - REPL logic (119 lines)
- `canal/NETWORK_INTERPRETER.md` - User documentation
- `canal/IMPLEMENTATION_STATUS.md` - This file

## 🎯 What Works Now

- ✅ Build system accepts WiFi credentials
- ✅ Kernel exports WiFi/network symbols
- ✅ Domain links against kernel symbols
- ✅ WiFi domain boots from flash and runs reliably
- ✅ TCP picoceci interpreter works over network
- ✅ Multi-line paste workflow is operational

## 🔨 What Still Needs Validation

- ❓ Long-duration soak test for WiFi reconnect behavior
- ❓ Concurrent logging load while running large pasted programs
- ❓ Graceful client disconnect/reconnect under repeated cycles

## 💡 Key Insights

1. **Symbol Export Pattern**: Use CMakeLists.txt `-Wl,-u,<symbol>` to force ESP-IDF symbols into kernel
2. **Domain Linking**: Use `--just-symbols` to share kernel symbols with domains
3. **Credential Passing**: Use TinyGo `-X` ldflags to embed build-time configuration
4. **Console Abstraction**: Interface-based design allows USB vs TCP I/O switching

This implementation provides a solid foundation for network-accessible embedded interpreters on ESP32-S3!
