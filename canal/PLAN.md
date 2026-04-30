# Canal OS — Roadmap to Shell + SDCard + TCP Server on ESP32-S3 N16R8

_Last updated: 2026-04-30_

---

## Where We Are

The kernel substrate is solid. What works:

| Component | Status | Notes |
|---|---|---|
| Boot sequence (`boot_esp32s3.go`) | ✅ Working | Loads domains, falls back to goroutine |
| Domain table (`domain.go`) | ✅ Working | Alloc, lifecycle, spinlock |
| ELF loader (`loader.go`) | ✅ Written | IDF-only; `??` in git (not yet committed) |
| `SpawnDomainFromFlash()` | ✅ Written | Calls `LoadDomain()` → `xTaskCreate()` |
| Capability table (`captable.go`) | ✅ Working | Alloc, grant, revoke, send/recv |
| Syscall queue infrastructure | ✅ Working | Queue created; **handler task never spawned** |
| LED domain | ✅ Working | WS2812, goroutine fallback |
| WiFi domain | ⚠️ Stub | Only heartbeat ticker |
| Logger domain | ⚠️ Stub | Only heartbeat ticker |
| Shell domain | ❌ Missing | Not started |
| SDCard domain | ❌ Missing | Not started |
| TCP server domain | ❌ Missing | Not started |
| Per-domain heap isolation | ⚠️ Partial | Memory reserved, MPU not enforced |

The ELF loading path (`loader.go` + `SpawnDomainFromFlash()`) is written but **not yet proven end-to-end** — domains may not have their runtime initialized correctly when called as a raw FreeRTOS task entry.

---

## Key Architecture Decisions (Decide Before Coding)

### A. Domain Runtime Model

**The problem:** When the ELF loader calls `xTaskCreate(entryPoint, ...)`, it jumps directly to `domain_entry`. A standard TinyGo binary expects `_start` → `runtime.init` → `main`. Skipping this may leave the GC and scheduler uninitialized.

**Decision:** Domains share the kernel's TinyGo runtime. They are **not** standalone TinyGo binaries with their own scheduler/GC. Each domain is compiled with `-buildmode=c-shared` or a custom entry-point-only linker setup where:
- `.bss` zeroing is handled by the ELF loader (already done)
- The domain uses `go` goroutines and `time.Sleep` through the kernel's scheduler
- The kernel's GC owns all memory; per-domain heaps are just pre-allocated slices

This means a domain binary is a **TinyGo module** (position-independent, no own scheduler) rather than a full program. The entry `domain_entry(domainID, syscallQ, replyQ)` is the goroutine body running inside a FreeRTOS task.

_Alternative_: True process isolation with per-domain TinyGo schedulers. This is the long-term goal but requires each binary to carry ~460KB runtime overhead. With 16MB flash it's feasible, but the runtime init problem remains. Defer to a later sprint.

### B. Memory Layout for N16R8 (16MB flash, 8MB PSRAM)

```
Flash (16MB):
  0x000000 - 0x008000: Bootloader (32KB)
  0x008000 - 0x009000: Partition table (4KB)
  0x010000 - 0x100000: Kernel (960KB)
  0x100000 - 0x180000: LED domain (512KB)
  0x180000 - 0x280000: WiFi domain (1MB)
  0x280000 - 0x380000: Logger domain (1MB)
  0x380000 - 0x480000: SDCard domain (1MB)
  0x480000 - 0x580000: Shell domain (1MB)
  0x580000 - 0x680000: TCP server domain (1MB)
  0x680000 - 0xF80000: Data/FAT partition (9.5MB, for logs + config files)
  0xF80000 - 0x1000000: NVS config (512KB)

PSRAM (8MB, at 0x3D000000):
  0x3D000000 - 0x3D080000: WiFi domain heap (512KB)
  0x3D080000 - 0x3D100000: Logger ring buffer + heap (512KB)
  0x3D100000 - 0x3D200000: SDCard domain heap + file buffers (1MB)
  0x3D200000 - 0x3D300000: Shell domain heap (1MB)
  0x3D300000 - 0x3D400000: TCP server domain heap (1MB)
  0x3D400000 - 0x3D800000: General kernel allocations (4MB)
```

**XIP for domain code**: Keep domain `.text` sections in flash (executed via ICache at `0x42000000 + offset`). The ELF loader only copies `.data` and zeros `.bss`. This eliminates the need to fit code into scarce IRAM and keeps domain images in flash. The linker script places `.text` in the ICache-mapped window and `.data`/`.bss` in PSRAM.

### C. WiFi Credentials

Store SSID/password in the NVS partition. First boot: provision via UART (serial `wifi-set SSID PASSWORD\n`). Subsequent boots: read from NVS. The kernel handles NVS reads at boot and passes credentials to the WiFi domain via a startup capability.

### D. IPC Message Protocol

Fixed-size FreeRTOS queue messages (already in place with `SyscallRequest`/`SyscallResponse`). For bulk data (shell I/O, file reads), use a shared PSRAM buffer capability with the message carrying a pointer + length. This avoids copying large data through 60-byte queue messages.

```go
// Extend SyscallRequest with a bulk-data variant:
type DataMessage struct {
    Op     uint32
    CapID  CapabilityID
    Ptr    uint32  // PSRAM address of bulk data
    Len    uint32
    _      [48]byte
}
```

---

## Phase 0: Solidify ELF Loading (1–2 days)

**Goal:** Prove that a domain binary loads from flash and runs correctly.

### 0.1 Fix the Syscall Handler Task

`InitSyscall()` creates the request queue but never spawns the handler. Every domain syscall blocks forever.

```go
// kernel/syscall.go — in InitSyscall(), after queue creation:
var taskHandle TaskHandle_t
xTaskCreate(
    syscallHandlerTrampoline,  // C trampoline → SyscallHandler()
    cstring("syscall"),
    4096,
    nil,
    3,   // Higher priority than domains
    &taskHandle,
)
```

### 0.2 Verify LED ELF Loading

Flash the LED domain ELF and confirm it loads without falling back to the goroutine path. The serial boot log should show `[flash] led id: 1`, not `[goroutine] led`.

If it fails, the most likely causes:
1. **Missing domain_entry export**: Confirm `//export domain_entry` is in the LED domain source and the linker script's `ENTRY(domain_entry)` matches.
2. **Wrong virtual addresses**: The ELF's PT_LOAD VAddr must point to writable DRAM/PSRAM, not flash. Check with `xtensa-esp32s3-elf-readelf -l build/out/led.elf`.
3. **Runtime not initialized**: If LED runs but immediately crashes, it may be calling GC/goroutine APIs before runtime init. For Phase 0, keep LED domain free of goroutines and GC in `domain_entry` — use only raw loops and machine API.

### 0.3 Update the Partition Table for N16R8

Replace `canal/build/targets/esp32s3/partitions.csv`:

```csv
# Canal OS — ESP32-S3 N16R8 (16MB flash)
# Name,     Type, SubType, Offset,   Size,    Flags
nvs,         data, nvs,     0x9000,   0x6000,
phy_init,    data, phy,     0xf000,   0x1000,
kernel,      app,  factory, 0x10000,  0xF0000,
led,         app,  0x20,    0x100000, 0x80000,
wifi,        app,  0x21,    0x180000, 0x100000,
logger,      app,  0x22,    0x280000, 0x100000,
sdcard,      app,  0x23,    0x380000, 0x100000,
shell,       app,  0x24,    0x480000, 0x100000,
tcp,         app,  0x25,    0x580000, 0x100000,
data,        data, fat,     0x680000, 0x980000,
config,      data, nvs,     0xF80000, 0x80000,
```

Update `domainPartitions` in `domain.go` and `config.mk` to match.

### 0.4 Enable PSRAM

The N16R8's 8MB PSRAM is not enabled by default. In `sdkconfig` (IDF menuconfig):

```
CONFIG_SPIRAM=y
CONFIG_SPIRAM_MODE_OCT=y
CONFIG_SPIRAM_SPEED_80M=y
CONFIG_SPIRAM_USE_CAPS_ALLOC=y
CONFIG_SPIRAM_MALLOC_ALWAYSINTERNAL=16384
```

Once enabled, IDF's heap allocator will automatically use PSRAM for large allocations (> 16KB by default). Domain heaps allocated with `make([]byte, heapSize)` will land in PSRAM for `heapSize >= 16KB`.

### 0.5 Commit Untracked Files

The following new files should be committed:
- `canal/kernel/loader.go`
- `canal/kernel/loader_stub.go`
- `canal/build/targets/esp32s3/domain-linker.ld`

---

## Phase 1: Syscall + IPC Infrastructure (2 days)

**Goal:** Domains can request kernel services and communicate with each other.

### 1.1 Syscall Handler Task (prerequisite, see 0.1)

Domains call the kernel by sending a `SyscallRequest` to their domain's `SyscallQ` and blocking on `ReplyQ`. The syscall handler task drains `SyscallQ` across all domains.

**Fix needed**: The current handler scans a single global queue. Change to iterate `domainTable` and drain each domain's queue. Or use a single kernel-wide inbox with `DomainID` in the request header — simpler.

### 1.2 Service Capability Registration

Replace the hardcoded `map[string]CapabilityID` in `syscall.go` with a dynamic registry:

```go
// kernel/registry.go
type ServiceRegistry struct {
    mu      uint32  // spinlock
    entries [64]serviceEntry
    count   int
}

type serviceEntry struct {
    name  [32]byte
    capID CapabilityID
    owner DomainID
}

func (r *ServiceRegistry) Register(name string, capID CapabilityID, owner DomainID) error
func (r *ServiceRegistry) Lookup(name string) (CapabilityID, bool)
```

Domains call `SyscallRegisterService(name, capID)` after they've initialized and are ready to accept requests. Other domains call `SyscallCapRequest(name)` to get a capability to that service.

### 1.3 Cross-Domain Channels

Introduce a `CapTypeChannel` workflow for bulk I/O:

```
Domain A                   Kernel                  Domain B
  │ CapRequest("logger")     │                        │
  │──────────────────────────▶│                        │
  │◀─ cap: ch_write ──────────│                        │
  │                           │                        │
  │ CapSend(ch_write, msg)    │ CapRecv(ch_read, msg)  │
  │──────────────────────────▶│───────────────────────▶│
```

Each channel is a pair of FreeRTOS queues. `CapSend` and `CapRecv` already work in `captable.go`; this sprint validates them with two real domains.

---

## Phase 2: WiFi Domain (3 days)

**Goal:** WiFi domain connects to an AP and exposes a TCP socket capability.

### 2.1 IDF WiFi Bindings

The WiFi domain cannot use the TinyGo `net` package — it needs direct IDF calls. Write a thin C wrapper:

```c
// domains/wifi/esp_wifi_glue.c
#include "esp_wifi.h"
#include "esp_event.h"
#include "nvs_flash.h"

void canal_wifi_init(void) {
    nvs_flash_init();
    esp_netif_init();
    esp_event_loop_create_default();
    esp_netif_create_default_wifi_sta();
    wifi_init_config_t cfg = WIFI_INIT_CONFIG_DEFAULT();
    esp_wifi_init(&cfg);
}

int canal_wifi_connect(const char *ssid, const char *pass) {
    wifi_config_t wc = {};
    strncpy((char*)wc.sta.ssid, ssid, 32);
    strncpy((char*)wc.sta.password, pass, 64);
    esp_wifi_set_mode(WIFI_MODE_STA);
    esp_wifi_set_config(WIFI_IF_STA, &wc);
    esp_wifi_start();
    return esp_wifi_connect();
}

void canal_wifi_get_ip(char *buf, int len) {
    esp_netif_ip_info_t info;
    esp_netif_get_ip_info(esp_netif_get_handle_from_ifkey("WIFI_STA_DEF"), &info);
    snprintf(buf, len, IPSTR, IP2STR(&info.ip));
}
```

Declare in Go via `//go:linkname` or `import "C"`.

### 2.2 WiFi Domain Entry

```go
// domains/wifi/main.go

//export domain_entry
func domain_entry(params *DomainParams) {
    println("[WiFi] starting")
    
    canalWifiInit()
    
    // Load credentials from NVS (passed in startup cap or hard-coded for dev)
    ssid, pass := loadWifiCreds()
    
    if err := canalWifiConnect(ssid, pass); err != nil {
        println("[WiFi] connect failed:", err)
        return
    }
    println("[WiFi] connected, IP:", canalWifiGetIP())
    
    // Register "wifi" service capability
    SyscallRegisterService("wifi.socket.create", socketCreateCap)
    
    // Event loop: reconnect on disconnect
    for {
        vTaskDelay(5000)
        if !wifiConnected() {
            canalWifiConnect(ssid, pass)
        }
    }
}
```

### 2.3 TCP Socket Capability

The WiFi domain exposes one capability: `wifi.socket.tcp.listen`. Other domains (TCP server) request this cap and call it to get a listening socket file descriptor.

```go
// Syscall message to WiFi domain:
// Op: OpSocketListen, Data: port uint16
// Reply: fd int32 (or -1 on error)
```

---

## Phase 3: TCP Server Domain (2 days)

**Goal:** Listen on TCP port, accept connections, pipe each to a Shell domain instance.

### 3.1 Socket Accept Loop

```go
//export domain_entry
func domain_entry(params *DomainParams) {
    // Get socket listen capability from WiFi domain
    listenCap := SyscallCapRequest("wifi.socket.tcp.listen")
    fd := socketListen(listenCap, 23)  // port 23 = telnet-style

    for {
        connFd := socketAccept(fd)
        if connFd < 0 { continue }
        
        // Create rx/tx channel capabilities for this connection
        rxCap := CapAlloc(domainID, CapTypeChannel, RightRead|RightWrite)
        txCap := CapAlloc(domainID, CapTypeChannel, RightRead|RightWrite)
        
        // Spawn a shell domain instance, giving it the I/O caps
        SyscallDomainSpawn("shell", rxCap, txCap)
        
        // Bridge TCP socket to channel pair
        go bridgeConn(connFd, rxCap, txCap)
    }
}
```

### 3.2 Shell Lifecycle

Each TCP connection spawns one shell domain. When the connection closes, the shell domain is killed. The TCP server tracks connection→domain mappings.

Support up to 4 simultaneous connections (limited by PSRAM heap budget).

---

## Phase 4: Shell Domain (3 days)

**Goal:** Interactive command interpreter, works over any I/O channel (TCP or UART).

### 4.1 I/O Abstraction

The shell receives two capability IDs: one for reading input, one for writing output. This makes it connection-agnostic.

```go
type ShellIO struct {
    rxCap CapabilityID
    txCap CapabilityID
}

func (io *ShellIO) ReadLine() string { /* CapRecv on rxCap, accumulate until \n */ }
func (io *ShellIO) Write(s string)   { /* CapSend on txCap */ }
func (io *ShellIO) Printf(f string, args ...any) { io.Write(sprintf(f, args...)) }
```

### 4.2 Command Set (v1)

| Command | Description |
|---|---|
| `help` | List commands |
| `ps` | List running domains (via kernel CapRequest) |
| `caps` | List capabilities owned by caller |
| `log [n]` | Print last n log lines from Logger |
| `ls [path]` | List directory (via SDCard cap) |
| `cat <file>` | Print file contents |
| `rm <file>` | Delete file |
| `uptime` | Boot time in seconds |
| `free` | Free heap and PSRAM |
| `reboot` | Restart ESP32-S3 (via kernel syscall) |
| `wifi-status` | Print IP, RSSI, SSID |
| `wifi-set <ssid> <pass>` | Store WiFi credentials to NVS |

### 4.3 Shell Domain Entry

```go
//export domain_entry
func domain_entry(params *ShellDomainParams) {
    io := &ShellIO{rxCap: params.RxCap, txCap: params.TxCap}
    io.Write("\r\nCanal OS shell\r\n$ ")
    
    for {
        line := io.ReadLine()
        if line == "" { continue }
        
        args := splitArgs(line)
        if len(args) == 0 { continue }
        
        cmd, ok := commands[args[0]]
        if !ok {
            io.Printf("unknown command: %s\r\n", args[0])
        } else {
            cmd(io, args[1:])
        }
        io.Write("$ ")
    }
}
```

---

## Phase 5: SDCard Domain (3 days)

**Goal:** FAT32 filesystem on SPI SD card, accessible via capability API.

### 5.1 Hardware Wiring (ESP32-S3 N16R8)

Suggested SPI pins (adjust for your board):

| Signal | GPIO |
|---|---|
| MOSI | GPIO 11 |
| MISO | GPIO 13 |
| SCK  | GPIO 12 |
| CS   | GPIO 10 |

These use the FSPI (SPI2) peripheral. Verify against your board's pinout — N16R8 modules vary.

### 5.2 IDF SDSPI Bindings

```c
// domains/sdcard/sdcard_glue.c
#include "driver/sdspi_host.h"
#include "esp_vfs_fat.h"

static sdmmc_card_t *s_card;

int canal_sdcard_mount(int mosi, int miso, int sck, int cs) {
    esp_vfs_fat_sdmmc_mount_config_t cfg = {
        .format_if_mount_failed = false,
        .max_files = 8,
        .allocation_unit_size = 4096,
    };
    sdmmc_host_t host = SDSPI_HOST_DEFAULT();
    sdspi_device_config_t slot = SDSPI_DEVICE_CONFIG_DEFAULT();
    slot.gpio_cs = cs;
    slot.host_id = host.slot;
    
    spi_bus_config_t buscfg = {
        .mosi_io_num = mosi, .miso_io_num = miso,
        .sclk_io_num = sck, .quadwp_io_num = -1, .quadhd_io_num = -1,
    };
    spi_bus_initialize(host.slot, &buscfg, SPI_DMA_CH_AUTO);
    
    return esp_vfs_fat_sdspi_mount("/sdcard", &host, &slot, &cfg, &s_card);
}

int canal_sdcard_open(const char *path, int flags) { return open(path, flags); }
int canal_sdcard_read(int fd, void *buf, int n)    { return read(fd, buf, n); }
int canal_sdcard_write(int fd, const void *buf, int n) { return write(fd, buf, n); }
int canal_sdcard_close(int fd)                     { return close(fd); }
```

### 5.3 SDCard Domain Capability API

```
SyscallCapRequest("sdcard.open")
  → sends: path (string), flags (read/write/create)
  → returns: CapTypeChannel (read = file data, write = file data)

SyscallCapRequest("sdcard.readdir")
  → sends: path (string)
  → returns: CapTypeChannel (each recv is one filename)
```

File handles are represented as capability IDs. Closing a file = revoking the capability.

### 5.4 SDCard Domain Entry

```go
//export domain_entry
func domain_entry(params *DomainParams) {
    err := canalSDCardMount(11, 13, 12, 10)  // MOSI, MISO, SCK, CS
    if err != 0 {
        println("[SDCard] mount failed:", err)
        return
    }
    println("[SDCard] mounted at /sdcard")
    
    SyscallRegisterService("sdcard.open", openServiceCap)
    SyscallRegisterService("sdcard.readdir", readdirServiceCap)
    
    // Serve open/readdir requests forever
    serveRequests()
}
```

---

## Phase 6: Logger Domain (Real) (1–2 days)

**Goal:** Central log aggregator, persists to SDCard FAT partition.

### 6.1 Log Message Format

```go
type LogEntry struct {
    Timestamp uint32    // milliseconds since boot
    DomainID  uint8
    Level     uint8     // 0=DEBUG 1=INFO 2=WARN 3=ERROR
    Len       uint16
    Msg       [56]byte  // fits in a single queue message
}
```

### 6.2 Logger Domain Entry

```go
//export domain_entry
func domain_entry(params *DomainParams) {
    ring := newRingBuffer(256)  // 256-entry in-PSRAM ring buffer
    
    SyscallRegisterService("logger.write", logWriteCap)
    SyscallRegisterService("logger.read", logReadCap)  // for shell `log` command
    
    // Open log file on SDCard (wait until SDCard domain is ready)
    var logFile CapabilityID
    for {
        var ok bool
        logFile, ok = SyscallCapRequest("sdcard.open")
        if ok { break }
        vTaskDelay(500)
    }
    openFile(logFile, "/sdcard/canal.log", FlagWrite|FlagAppend|FlagCreate)
    
    for {
        entry := CapRecv(logWriteCap)
        ring.Push(entry)
        writeToFile(logFile, entry)
        
        // Flush every 16 entries (not on every write — FAT is slow)
        if ring.count % 16 == 0 {
            flushFile(logFile)
        }
    }
}
```

---

## Phase 7: Integration (2 days)

**Goal:** End-to-end test: power on → shell available via TCP in < 10 seconds.

### Boot Sequence Order

```
1. Kernel boots
2. Logger domain spawns (no SDCard yet — logs to ring buffer only)
3. WiFi domain spawns → connects to AP → registers socket capability
4. SDCard domain spawns → mounts → registers file capabilities
5. Logger opens log file on SDCard
6. TCP server spawns → gets socket cap → listens on port 23
7. Shell-ready LED indicator (LED blinks green)
```

Dependency enforcement: use kernel `SyscallCapRequest` with retry loop (domains poll until the capability becomes available).

### Integration Test Checklist

- [ ] Connect via `telnet <esp32-ip> 23`, get shell prompt
- [ ] `ps` shows all running domains
- [ ] `log 20` shows last 20 log lines from boot
- [ ] `ls /sdcard` shows FAT filesystem contents
- [ ] `cat /sdcard/canal.log` shows log file
- [ ] WiFi reconnects after AP restart (< 30 seconds)
- [ ] Multiple simultaneous TCP connections (2–3)
- [ ] SD card removal handled gracefully (SDCard domain logs error, keeps ring buffer)
- [ ] System survives 24-hour run without crash or memory exhaustion

---

## Partition Table (Final, N16R8)

File: `canal/build/targets/esp32s3/partitions.csv`

```csv
# Canal OS — ESP32-S3 N16R8 (16MB flash)
# Domain ELF binaries include TinyGo runtime shim (~300KB each with shared runtime approach).
# Name,     Type, SubType, Offset,   Size,    Flags
nvs,         data, nvs,     0x9000,   0x6000,
phy_init,    data, phy,     0xf000,   0x1000,
kernel,      app,  factory, 0x10000,  0xF0000,
led,         app,  0x20,    0x100000, 0x80000,
wifi,        app,  0x21,    0x180000, 0x100000,
logger,      app,  0x22,    0x280000, 0x100000,
sdcard,      app,  0x23,    0x380000, 0x100000,
shell,       app,  0x24,    0x480000, 0x100000,
tcp,         app,  0x25,    0x580000, 0x100000,
data,        data, fat,     0x680000, 0x980000,
config,      data, nvs,     0xF80000, 0x80000,
```

Addresses in `domain.go`:

```go
var domainPartitions = map[string]uint32{
    "led":    0x100000,
    "wifi":   0x180000,
    "logger": 0x280000,
    "sdcard": 0x380000,
    "shell":  0x480000,
    "tcp":    0x580000,
}
```

---

## Sprint Summary

| Phase | Duration | Deliverable |
|---|---|---|
| 0: Solidify ELF Loading | 1–2 days | LED loads from flash, syscall task spawns, PSRAM enabled |
| 1: IPC Infrastructure | 2 days | Service registry, cross-domain channels work |
| 2: WiFi Domain | 3 days | Connects to AP, exposes TCP socket cap |
| 3: TCP Server | 2 days | Accepts connections, spawns shell per connection |
| 4: Shell Domain | 3 days | Interactive command set, I/O via capability channels |
| 5: SDCard Domain | 3 days | FAT32 on SPI SD card, file capability API |
| 6: Logger Domain | 1–2 days | Ring buffer + SDCard persistence + shell `log` command |
| 7: Integration | 2 days | End-to-end test, stability run |

**Total: ~18–19 days of focused work.**

---

## N16R8-Specific Notes

- **Octal PSRAM**: Must enable `CONFIG_SPIRAM_MODE_OCT` — the N16R8 uses octal (8-bit) PSRAM interface, not quad. Using quad mode silently fails or reads garbage.
- **Flash XIP**: The 16MB flash uses 4-line SPI at 80MHz. The ICache window at `0x42000000` covers all 16MB. No special config needed.
- **GPIO 48**: Reserved for WS2812 LED in the current kernel code. Verify it doesn't conflict with your SD card or other wiring.
- **SPI bus conflicts**: If using SPI for both SD card and other peripherals, use separate SPI bus instances (FSPI vs HSPI).
- **USB-JTAG port**: The N16R8's USB re-enumerates on reset (documented in memory). Use the existing `flash.sh` approach: open port before flashing.
- **PSRAM and WiFi**: Both WiFi and PSRAM use SPI. On N16R8, PSRAM is Octal SPI and WiFi uses a separate RF interface — no conflict. But the SPIRAM speed must be ≤ 80MHz; going higher causes instability.

---

## What to Start With

**Sprint 0, task 1**: Add the `xTaskCreate` call in `InitSyscall()` to spawn the syscall handler task. This is a 5-line fix that unblocks all IPC work. Then flash and verify the LED domain loads from flash (not the goroutine fallback). Once those two things work, the rest of this plan is straightforward execution.
