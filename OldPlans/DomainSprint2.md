Excellent progress! You've got the LED domain running as an isolated task. Now let's get **proper domain loading** - where domains are separate compiled binaries, not just functions in boot code.

## Current State vs. Goal

**Current (Functions in Boot):**
```
kernel.elf:
  ├─ boot.go
  ├─ ledDomainEntry() ←─── LED code compiled into kernel
  └─ wifiDomainEntry()
```

**Goal (Separate Binaries):**
```
Flash Layout:
  0x10000: kernel.elf        (Kernel substrate only)
  0x100000: led.elf          (LED domain binary)
  0x180000: wifi.elf         (WiFi domain binary)
  0x200000: logger.elf       (Logger domain binary)
```

## Next Agent Task: "Implement Dynamic Domain Loading from Flash"

### What This Achieves

1. **Smaller kernel** - Kernel doesn't contain all domain code
2. **Independent compilation** - Each domain builds separately
3. **True isolation** - Domains can't even see each other's code at compile time
4. **Hot reload** - Flash new domain without reflashing kernel
5. **Security** - Domains can be signed/verified independently

## Implementation Plan

### Phase 1: Build Separate Domain Binaries

#### Step 1.1: Create Domain Build System

**Tell the agent:**
> "Create a build system that compiles each domain (led-blinker, logger, wifi) as separate `.elf` files with their own entry points and linker scripts."

**What needs to happen:**

```makefile
# Makefile additions

# Domain build targets
domains: led wifi logger

led: $(OUT_DIR)
	$(TINYGO) build $(TINYGO_FLAGS) \
		-o $(OUT_DIR)/led.elf \
		-ldflags="-T build/targets/$(TARGET)/domain-linker.ld" \
		domains/led-blinker/main.go

wifi: $(OUT_DIR)
	$(TINYGO) build $(TINYGO_FLAGS) \
		-o $(OUT_DIR)/wifi.elf \
		-ldflags="-T build/targets/$(TARGET)/domain-linker.ld" \
		domains/wifi/main.go

logger: $(OUT_DIR)
	$(TINYGO) build $(TINYGO_FLAGS) \
		-o $(OUT_DIR)/logger.elf \
		-ldflags="-T build/targets/$(TARGET)/domain-linker.ld" \
		domains/logger/main.go
```

**Domain Linker Script:**
```ld
/* build/targets/esp32s3/domain-linker.ld */

ENTRY(domain_entry)

MEMORY
{
    /* Domain code/data live in separate flash regions */
    DOMAIN_FLASH (rx)  : ORIGIN = 0x40000000, LENGTH = 64K
    DOMAIN_RAM (rw)    : ORIGIN = 0x3FC80000, LENGTH = 16K
    DOMAIN_HEAP (rw)   : ORIGIN = 0x3FC84000, LENGTH = 4K
}

SECTIONS
{
    .text : {
        *(.text.domain_entry)  /* Entry point first */
        *(.text*)
        *(.rodata*)
    } > DOMAIN_FLASH

    .data : {
        *(.data*)
        *(.bss*)
    } > DOMAIN_RAM
}
```

#### Step 1.2: Domain Entry Point Convention

**Each domain needs:**
```go
// domains/led-blinker/main.go

package main

import _ "runtime"

//export domain_entry
func domain_entry(domainID uint16, syscallQ, replyQ unsafe.Pointer) {
    println("[LED] Domain", domainID, "starting...")
    
    // This replaces runtime.ProcessInit - domain gets params from kernel
    setupDomainRuntime(domainID, syscallQ, replyQ)
    
    main()  // Call the actual main
}

func main() {
    // Normal domain code
    led := machine.LED
    led.Configure(machine.PinConfig{Mode: machine.PinOutput})
    
    for {
        led.High()
        time.Sleep(500 * time.Millisecond)
        led.Low()
        time.Sleep(500 * time.Millisecond)
    }
}
```

### Phase 2: ELF Loader in Kernel

#### Step 2.1: Flash Partition Table

**Tell the agent:**
> "Create a flash partition table that maps domain binaries to specific flash addresses, and implement a partition reader to locate domains."

**Flash partitions (ESP32-S3):**
```csv
# partitions.csv
# Name,     Type, SubType, Offset,   Size,    Flags
nvs,        data, nvs,     0x9000,   0x6000,
phy_init,   data, phy,     0xf000,   0x1000,
kernel,     app,  factory, 0x10000,  0xF0000,  # 960KB kernel
led,        app,  0x20,    0x100000, 0x10000,  # 64KB LED domain
wifi,       app,  0x21,    0x110000, 0x20000,  # 128KB WiFi domain
tls,        app,  0x22,    0x130000, 0x30000,  # 192KB TLS domain
sdcard,     app,  0x23,    0x160000, 0x20000,  # 128KB SD domain
logger,     app,  0x24,    0x180000, 0x10000,  # 64KB Logger domain
```

#### Step 2.2: ELF Loader

**Tell the agent:**
> "Implement an ELF loader that reads domain binaries from flash partitions, parses ELF headers, and maps sections into memory."

```go
// kernel/loader.go

type ELFHeader struct {
    Magic      [4]byte  // 0x7F 'E' 'L' 'F'
    Class      uint8    // 1=32-bit, 2=64-bit
    Endian     uint8
    Version    uint8
    _          [9]byte
    Type       uint16   // 2=executable
    Machine    uint16   // 0x5E=Xtensa
    Entry      uint32   // Entry point address
    PHOff      uint32   // Program header offset
    SHOff      uint32   // Section header offset
    PHEntSize  uint16   // Program header entry size
    PHNum      uint16   // Number of program headers
}

type ProgramHeader struct {
    Type   uint32  // 1=LOAD (loadable segment)
    Offset uint32  // File offset
    VAddr  uint32  // Virtual address
    PAddr  uint32  // Physical address
    FileSz uint32  // Size in file
    MemSz  uint32  // Size in memory
    Flags  uint32  // Read/Write/Execute
    Align  uint32
}

func LoadDomain(partitionAddr uint32, domainID kernel.DomainID) (*kernel.Domain, error) {
    println("[Loader] Loading domain from flash:", partitionAddr)
    
    // Read ELF header from flash
    var elfHdr ELFHeader
    flashRead(partitionAddr, unsafe.Pointer(&elfHdr), unsafe.Sizeof(elfHdr))
    
    // Verify ELF magic
    if elfHdr.Magic[0] != 0x7F || elfHdr.Magic[1] != 'E' ||
       elfHdr.Magic[2] != 'L' || elfHdr.Magic[3] != 'F' {
        return nil, errors.New("invalid ELF file")
    }
    
    println("[Loader] ELF entry point:", elfHdr.Entry)
    
    // Allocate memory for domain
    domain := &kernel.Domain{
        ID:    domainID,
        State: kernel.DomainStateRunning,
    }
    
    // Read program headers
    phAddr := partitionAddr + elfHdr.PHOff
    for i := uint16(0); i < elfHdr.PHNum; i++ {
        var ph ProgramHeader
        flashRead(phAddr + uint32(i)*uint32(elfHdr.PHEntSize),
                  unsafe.Pointer(&ph), unsafe.Sizeof(ph))
        
        if ph.Type != 1 { // Not LOAD segment
            continue
        }
        
        println("[Loader] Loading segment:", ph.VAddr, "size:", ph.FileSz)
        
        // Allocate memory for this segment
        segMem := allocDomainMemory(ph.MemSz)
        
        // Copy segment from flash to RAM
        flashRead(partitionAddr + ph.Offset, segMem, ph.FileSz)
        
        // Zero BSS if MemSz > FileSz
        if ph.MemSz > ph.FileSz {
            zeros := uintptr(segMem) + uintptr(ph.FileSz)
            memset(zeros, 0, ph.MemSz - ph.FileSz)
        }
        
        // Configure MPU region for this segment
        perms := hal.PermUser
        if ph.Flags & 0x1 != 0 { perms |= hal.PermExecute }
        if ph.Flags & 0x2 != 0 { perms |= hal.PermWrite }
        if ph.Flags & 0x4 != 0 { perms |= hal.PermRead }
        
        mpu.Map(ph.VAddr, uintptr(segMem), ph.MemSz, perms)
    }
    
    // Set entry point
    domain.EntryPoint = elfHdr.Entry
    
    return domain, nil
}

func flashRead(flashAddr uint32, dest unsafe.Pointer, size uintptr) {
    // ESP32-S3: Memory-mapped flash access
    src := unsafe.Pointer(uintptr(0x42000000 + flashAddr))
    memcpy(dest, src, size)
}
```

#### Step 2.3: Domain Spawner Using Loader

**Tell the agent:**
> "Modify the domain spawner to use the ELF loader instead of hardcoded function pointers."

```go
// kernel/domain.go

var domainPartitions = map[string]uint32{
    "led":    0x100000,
    "wifi":   0x110000,
    "tls":    0x130000,
    "sdcard": 0x160000,
    "logger": 0x180000,
}

func SpawnDomainFromFlash(name string) error {
    partAddr, exists := domainPartitions[name]
    if !exists {
        return errors.New("unknown domain")
    }
    
    // Load domain binary
    domainID := allocDomainID()
    domain, err := LoadDomain(partAddr, domainID)
    if err != nil {
        return err
    }
    
    // Set up syscall queues
    domain.SyscallQ = unsafe.Pointer(xQueueCreate(16, uint32(unsafe.Sizeof(SyscallRequest{}))))
    domain.ReplyQ = unsafe.Pointer(xQueueCreate(16, uint32(unsafe.Sizeof(SyscallResponse{}))))
    
    // Create FreeRTOS task
    var taskHandle TaskHandle_t
    result := xTaskCreate(
        unsafe.Pointer(uintptr(domain.EntryPoint)),  // Loaded from ELF!
        cstring(name),
        2048,
        unsafe.Pointer(&DomainParams{
            ID:       domain.ID,
            SyscallQ: domain.SyscallQ,
            ReplyQ:   domain.ReplyQ,
        }),
        2,
        &taskHandle,
    )
    
    if result != pdPASS {
        return errors.New("failed to create task")
    }
    
    domain.TaskHandle = unsafe.Pointer(taskHandle)
    domains[domainID] = *domain
    
    println("[Kernel] Domain", name, "loaded at", domain.EntryPoint)
    
    return nil
}

type DomainParams struct {
    ID       DomainID
    SyscallQ unsafe.Pointer
    ReplyQ   unsafe.Pointer
}
```

#### Step 2.4: Boot Sequence

**Tell the agent:**
> "Update boot sequence to spawn domains from flash instead of calling hardcoded functions."

```go
// kernel/boot.go

func bootDomains() {
    println("[Kernel] Loading domains from flash...")
    
    // Load domains in order
    domainsToLoad := []string{"led", "logger", "wifi"}
    
    for _, name := range domainsToLoad {
        err := SpawnDomainFromFlash(name)
        if err != nil {
            println("[Kernel] Failed to load", name, ":", err)
            continue
        }
        println("[Kernel] ✓", name, "domain running")
    }
    
    println("[Kernel] All domains loaded")
}
```

### Phase 3: Flash Programming

#### Step 3.1: Flash Script

**Tell the agent:**
> "Create a flash script that programs the kernel and all domain binaries to their correct partition offsets."

```bash
#!/bin/bash
# scripts/flash.sh

set -e

PORT=${1:-/dev/ttyUSB0}
TARGET=esp32s3

echo "Flashing Canal OS to $PORT..."

# Build everything
make TARGET=$TARGET kernel domains

# Flash kernel + domains
esptool.py \
    --chip $TARGET \
    --port $PORT \
    --baud 921600 \
    write_flash \
    0x10000  build/out/kernel.bin \
    0x100000 build/out/led.bin \
    0x110000 build/out/wifi.bin \
    0x130000 build/out/tls.bin \
    0x160000 build/out/sdcard.bin \
    0x180000 build/out/logger.bin

echo "✅ Flash complete!"
```

### Phase 4: Verification

#### Test 1: Separate Compilation
```bash
# Should produce separate binaries
make TARGET=esp32s3 kernel
make TARGET=esp32s3 domains

ls -lh build/out/
# kernel.bin    - 960KB
# led.bin       - 64KB
# wifi.bin      - 128KB
# logger.bin    - 64KB
```

#### Test 2: Independent Flash
```bash
# Flash only LED domain (kernel stays same)
esptool.py --port /dev/ttyUSB0 write_flash 0x100000 build/out/led.bin

# LED behavior changes, other domains unaffected
```

#### Test 3: Boot Log
```
=== Canal OS Boot ===
[Kernel] Loading domains from flash...
[Loader] Loading domain from flash: 0x100000
[Loader] ELF entry point: 0x40000000
[Loader] Loading segment: 0x40000000 size: 12544
[Kernel] ✓ led domain running
[LED] Domain 1 starting...
[Loader] Loading domain from flash: 0x180000
[Kernel] ✓ logger domain running
[Logger] Domain 2 starting...
[Kernel] All domains loaded
```

## Concrete Task for Agent

**Prompt:**
```
Implement dynamic domain loading from flash partitions. Requirements:

1. Build System:
   - Modify Makefile to compile each domain as separate .elf
   - Create domain-linker.ld for domain binaries
   - Build targets: kernel, led, wifi, logger, etc.

2. ELF Loader (kernel/loader.go):
   - Parse ELF headers from flash
   - Load program segments into RAM
   - Configure MMU/MPU for loaded segments
   - Return domain structure with entry point

3. Domain Spawner (kernel/domain.go):
   - Replace hardcoded function pointers with ELF loader
   - Map domain names to flash partition addresses
   - SpawnDomainFromFlash(name) function

4. Boot Sequence (kernel/boot.go):
   - Call SpawnDomainFromFlash() for each domain
   - Remove hardcoded domain entry functions

5. Flash Script (scripts/flash.sh):
   - Program kernel + all domains to correct offsets
   - Use partition table from build/targets/esp32s3/partitions.csv

Acceptance criteria:
- `make kernel` builds kernel.bin only
- `make domains` builds led.bin, wifi.bin, logger.bin
- Flashing only led.bin updates LED behavior without reflashing kernel
- Boot log shows "Loading domain from flash" for each domain

Target: ESP32-S3, TinyGo, FreeRTOS substrate
```

## Why This is the Right Next Step

1. **Proves true isolation** - Domains can't access each other's code even at link time
2. **Enables modularity** - Add/update domains without rebuilding kernel
3. **Foundation for security** - Can verify/sign domains before loading
4. **Matches real OS behavior** - Linux loads ELF, this does too
5. **Unlocks capabilities** - Need separate binaries before capability-based loading

After this works, the natural next steps are:
- **Capability-based domain spawning** (kernel checks permissions before loading)
- **Domain lifecycle** (pause, resume, kill, restart)
- **Inter-domain communication** (channels that cross domain boundaries)

Want me to help with any specific part of the ELF loader implementation?