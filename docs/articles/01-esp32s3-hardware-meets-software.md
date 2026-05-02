# Article 1: The ESP32-S3 System: Hardware Meets Software

> **Canal version**: 0.1.0-dev | **picoceci version**: 0.1.0-dev
> **Target audience**: Developers new to embedded systems or the ESP32 family.
> **Prerequisites**: Basic programming familiarity; no embedded-systems experience required.

---

## Introduction

Modern microcontrollers are no longer the simple, single-purpose chips of the 1980s.
The Espressif ESP32-S3 packs dual 240 MHz cores, megabytes of RAM, Wi-Fi, Bluetooth, USB,
and a rich peripheral set into a package that costs a few dollars and draws only milliwatts
of power. Yet turning that raw silicon into a *safe*, *structured* computing platform—one
where multiple programs can run side-by-side without stepping on each other—requires
deliberate design choices at every layer of the stack.

That is exactly what **Canal** does. Canal is a capability-based microkernel for embedded
systems, written in [TinyGo](https://tinygo.org/). It uses the ESP32-S3's hardware security
features to isolate programs into independent *domains*, each of which can only access
resources it has been explicitly granted.

This article builds the mental model you need before diving into Canal's architecture or
its companion language, picoceci. By the end you will understand:

- What the ESP32-S3 chip provides at the hardware level.
- How Canal's software layers map onto that hardware.
- Why this particular chip is a good fit for a capability-based microkernel.

---

## 1. ESP32-S3 at a Glance

The ESP32-S3 is Espressif's second-generation dual-core Wi-Fi + Bluetooth SoC (System on
Chip). The highlights relevant to Canal are:

| Feature | Detail |
|---------|--------|
| CPU cores | Dual-core Xtensa LX7 at up to 240 MHz |
| On-chip SRAM | 512 KB total (split between data and instruction RAM) |
| External PSRAM | Up to 8 MB over Octal SPI (OPI) or Quad SPI |
| Flash storage | Up to 16 MB (external, over SPI) |
| Wireless | Wi-Fi 4 (802.11 b/g/n) + Bluetooth 5.0 / BLE |
| Peripherals | SPI, I²C, I²S, UART, USB OTG, ADC, DAC, touch sensors, PWM |
| Security | Secure Boot v2, Flash Encryption, Digital Signature peripheral |
| Memory protection | MMU with page-based addressing + hardware PID controller |

The dual LX7 cores run the same ISA (Instruction Set Architecture) as the earlier
ESP32, but with several extensions: wider SIMD instructions, a deeper pipeline, and,
most importantly for Canal, a revamped Memory Management Unit.

---

## 2. Memory Architecture

Understanding the ESP32-S3's memory map is essential to understanding Canal, because
Canal uses *hardware* memory boundaries to enforce the isolation between domains.

### 2.1 Internal SRAM

The 512 KB on-chip SRAM is split into two regions:

```
┌──────────────────────────────┐  0x3FC88000
│  DRAM (Data RAM) — ~300 KB  │  read/write data, stack, kernel heap
├──────────────────────────────┤  0x40370000
│  IRAM (Instruction RAM)      │  ~200 KB, fast path for interrupt handlers
└──────────────────────────────┘
```

- **DRAM** is where variables, stacks, and dynamic allocations live.
- **IRAM** is mapped for code execution and is close to the CPU's instruction cache, so
  it is used for latency-sensitive routines (interrupt service routines, FreeRTOS tick
  handler, etc.).

Canal's kernel data structures—the capability table, domain table, and syscall queues—live
in DRAM. User-domain heaps are deliberately placed in PSRAM (see below) so that the
comparatively precious on-chip SRAM is reserved for the kernel and system services.

### 2.2 PSRAM

The 8 MB PSRAM is accessed over a dedicated high-speed bus (OPI or QSPI). It is slower
than on-chip SRAM (roughly 10× more latency) but large enough to host several independent
domain heaps. Canal maps each domain's heap into a different region of PSRAM and
configures the MMU so that only the owning domain's hardware PID tag can access that
region.

Typical domain heap sizes (defined in `kernel/domain.go`):

| Constant | Size | Use case |
|----------|------|----------|
| `HeapTiny` | 2 KB | LED blinker, simple sensor domains |
| `HeapSmall` | 8 KB | Typical service domain |
| `HeapMedium` | 32 KB | Wi-Fi, HTTP, complex domains |

### 2.3 Flash Memory and Partition Tables

The SPI Flash holds the bootloader, the Canal kernel, and a partition for each domain.
Each domain is stored in its own 512 KB flash partition, allowing `SpawnDomainFromFlash`
to load only the code it needs at runtime. The partition map (in
`build/targets/esp32s3/partitions.csv`) looks roughly like this:

```
nvs,       data, nvs,       0x009000, 0x006000
phy_init,  data, phy,       0x00F000, 0x001000
factory,   app,  factory,   0x010000, 0x0F0000   # Canal kernel
led,       app,  ota_0,     0x100000, 0x080000   # LED blinker domain
wifi,      app,  ota_1,     0x180000, 0x080000   # Wi-Fi domain
logger,    app,  ota_2,     0x200000, 0x080000   # Logger domain
tls,       app,  ota_3,     0x280000, 0x080000   # TLS domain
sdcard,    app,  ota_4,     0x300000, 0x080000   # SD-card domain
```

When Canal boots, `SpawnDomainFromFlash` reads the ELF binary from the appropriate flash
offset, copies PT_LOAD segments into PSRAM, and hands the entry point to a freshly
created FreeRTOS task.

---

## 3. Hardware Security Features

Canal's safety story would be hard to tell without the ESP32-S3's hardware assist. Three
mechanisms matter most.

### 3.1 The Memory Management Unit (MMU)

The ESP32-S3 MMU translates virtual addresses to physical addresses using a page table
that the kernel controls. Every memory access goes through this translation. If a virtual
address does not map to a valid physical page—or if the access violates the permissions
set for that page—the MMU raises a fault and the kernel handles it.

In Canal, each domain has a set of MMU pages assigned only to it. When the kernel switches
to a different domain it also switches to that domain's page table. Any attempt by the
running domain to read or write memory outside its own pages produces a hardware fault
*before* the data is touched—the bug is caught by hardware, not by a software check that
could itself be bypassed.

```
Domain A pages:   │XXXXXXXX│        │        │        │
Domain B pages:   │        │YYYYYYYY│        │        │
Domain C pages:   │        │        │ZZZZZZZZ│        │
Kernel pages:     │        │        │        │KKKKKKKK│

Domain A running → MMU allows X, rejects Y, Z, K
```

### 3.2 The PID Controller

On top of the MMU, the ESP32-S3 has a hardware PID (Process ID) controller. Each memory
bus transaction carries the current PID tag of the executing context. The PID controller
can be configured to allow or deny access to a physical memory range based on that tag,
providing a second layer of protection independent of the virtual address translation.

Canal uses PID tags to enforce domain boundaries even in scenarios where the MMU
configuration might be incomplete (for example, during the brief moment when the kernel is
setting up a new domain's page table entries). A domain running with PID 3 literally
*cannot* read memory tagged as belonging to PID 7 at the bus level—not even if it somehow
learned the physical address.

### 3.3 Secure Boot and Flash Encryption (Optional)

For production deployments Canal can rely on the ESP32-S3's secure boot chain:

- **Secure Boot v2**: The ROM bootloader verifies a cryptographic signature on the Canal
  kernel image before executing it. A modified or tampered kernel will not boot.
- **Flash Encryption**: All data on the SPI Flash is encrypted with a device-unique AES
  key stored in eFuses. Even if someone desolders the flash chip and reads it with external
  hardware they see only ciphertext.

These features are optional during development (they make the write/flash cycle slower)
but strongly recommended for any device deployed in the field.

---

## 4. Why the ESP32-S3 for Canal

Several microcontrollers offer memory protection—ARM's Cortex-M series has the MPU,
RISC-V cores have PMP—but the ESP32-S3 is Canal's primary target for a combination of
reasons.

**Hardware isolation with MMU *and* PID.** Most Cortex-M parts have an MPU with 8–16
regions, which can be limiting when running many domains. The ESP32-S3's full MMU provides
page-granular protection for an arbitrary number of domains, and the PID controller adds a
second, independent enforcement layer.

**Dual cores.** With two LX7 cores, Canal can dedicate one core to the kernel and system
services while the second core runs user domains. This reduces the latency impact of
domain switches and makes real-time guarantees easier to meet.

**Memory capacity.** The combination of 512 KB internal SRAM and 8 MB PSRAM gives Canal
enough room to host the kernel, several system service domains, and multiple user domains
simultaneously—something that is very tight on a 256 KB Cortex-M0+ part.

**Affordable and accessible.** The ESP32-S3 is available in development boards for under
$5, has extensive community documentation, and is supported by the mature
[ESP-IDF](https://github.com/espressif/esp-idf) framework as well as
[TinyGo](https://tinygo.org/), the Go compiler Canal is built with.

**Ecosystem tooling.** `esptool.py`, OpenOCD-based JTAG debugging, and the IDF monitor
all work out of the box, meaning the build-flash-debug cycle that developers depend on
does not require custom tooling.

> **Note**: Canal also supports ARM Cortex-M (Raspberry Pi Pico) and is adding RISC-V
> support, so the concepts in this series apply broadly. The ESP32-S3 is the reference
> target where all features are implemented first.

---

## 5. The Software Stack: From Bootloader to User Domain

With the hardware picture in place, here is how the software layers stack on top of it:

```
┌───────────────────────────────────────────────────────┐
│                   User Domains                        │
│   picoceci  │  HTTP Server  │  Sensor Logger  │  ...  │
└──────────────────────┬────────────────────────────────┘
                       │  Canal IPC (capability channels)
┌──────────────────────▼────────────────────────────────┐
│               System Service Domains                  │
│     Wi-Fi    │    TLS    │   Filesystem   │   GPIO    │
└──────────────────────┬────────────────────────────────┘
                       │  Syscall interface
┌──────────────────────▼────────────────────────────────┐
│               Canal Kernel Substrate                  │
│   Capability Table  │  Domain Manager  │  MMU/PID    │
│   Syscall Handler   │  FreeRTOS tasks  │  Scheduler  │
└──────────────────────┬────────────────────────────────┘
                       │
┌──────────────────────▼────────────────────────────────┐
│                  Hardware Layer                       │
│   Xtensa LX7  │  Flash  │  PSRAM  │  Wi-Fi  │  GPIO  │
└───────────────────────────────────────────────────────┘
```

**Bootloader** (ROM + second-stage)
: The ROM bootloader in chip eFuses verifies the second-stage bootloader signature (if
Secure Boot is enabled) and loads it from flash. The second-stage bootloader initialises
the SPI flash clock, sets up the partition table, and hands off to the Canal kernel.

**Canal kernel**
: Initialises the MMU, capability table, domain table, and syscall handler. It then
spawns the first-tier system service domains (Wi-Fi, TLS, Filesystem, GPIO) from their
flash partitions and starts the FreeRTOS scheduler. Once the scheduler is running, the
kernel acts as a passive service: it handles syscalls from domains and enforces
capability-based access control.

**System service domains**
: Long-lived domains that wrap hardware peripherals or compound services. They are more
privileged than user domains in the sense that they hold capabilities to hardware
resources, but they are still isolated from each other and from the kernel by the MMU.
A bug in the Wi-Fi domain cannot corrupt the TLS domain's private key material.

**User domains**
: Short-lived or long-lived programs loaded on demand. They start with no capabilities
and must request what they need at spawn time. A user domain that crashes or misbehaves
is killed by the kernel without affecting anything else on the system.

### Boot Sequence in Practice

```
1. ROM bootloader runs (chip reset vector)
2. Second-stage bootloader: clock, flash, partition table
3. Canal kernel entry: MMU init → CapTable init → DomainTable init
4. Kernel spawns system service domains (Wi-Fi, TLS, SDCard, …)
5. FreeRTOS scheduler starts
6. System domains initialise hardware (Wi-Fi stack, TLS context, …)
7. User domain(s) loaded on request or at configured startup
8. Normal operation: domains communicate via capability channels
```

Expected console output after a successful boot:

```
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
```

---

## Summary

The ESP32-S3 gives Canal four critical hardware gifts: a full MMU for page-granular memory
isolation, a PID controller for bus-level domain tagging, dual cores for parallel
execution, and 8 MB of PSRAM for per-domain heaps. Canal's software stack maps cleanly
onto this hardware, with the kernel living in protected SRAM, service domains in PSRAM,
and user code in its own isolated slices.

In [Article 2](./02-picoceci-a-language-for-tiny-machines.md) we will look at picoceci—
the language designed to live inside a Canal domain and expose the hardware's power through
a safe, teachable programming model.

---

## Exercises

1. **Dual-core advantage.** The ESP32-S3 has two Xtensa LX7 cores. Name two advantages
   this gives Canal over a single-core microcontroller when running multiple isolated
   domains simultaneously.

2. **Memory diagram.** Draw a simple ASCII diagram that shows the four memory regions
   available on the ESP32-S3 (internal SRAM, IRAM, PSRAM, and flash) and note the typical
   size of each. Explain in one sentence why Canal maps user-domain heaps into PSRAM rather
   than internal SRAM.

3. **PID controller.** Explain the role of the PID controller in Canal's security model.
   How does hardware tagging of memory accesses prevent one domain from reading another
   domain's data, even if a bug in that domain constructs an arbitrary pointer?

4. **Boot sequence.** Trace the boot sequence from power-on to a running user domain by
   listing each layer in order (bootloader, Canal kernel, system domains, user domain) and
   writing one sentence describing what each layer sets up before passing control to the
   next.

5. **Memory discipline.** Suppose a student has an Arduino background and asks "why can't
   I just use `malloc` everywhere?" Write a two-paragraph explanation—suitable for that
   student—of why Canal enforces per-domain memory budgets and what could go wrong on a
   microcontroller without that discipline.
