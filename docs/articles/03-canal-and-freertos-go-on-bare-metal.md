# Article 3: Canal and FreeRTOS — Running Go on Bare Metal

> **Canal version**: 0.1.0-dev | **picoceci version**: 0.1.0-dev
> **Target audience**: Systems programmers and embedded developers who want to understand
> Canal's internals.
> **Prerequisites**: [Article 1](./01-esp32s3-hardware-meets-software.md) and
> [Article 2](./02-picoceci-a-language-for-tiny-machines.md), or familiarity with RTOS
> concepts and Go/TinyGo.

---

## Introduction

Articles 1 and 2 introduced the ESP32-S3 hardware and the picoceci language from the
outside looking in. This article opens the hood. We will look at:

- Canal's design goals and why those goals led to the architecture you will see.
- How FreeRTOS is used as a low-level scheduling substrate—and why user domains never see
  it.
- How TinyGo's runtime fits inside an isolated domain.
- The kernel structures—capability table, domain table, syscall interface—that knit it all
  together.
- How inter-domain messages travel across domain boundaries.
- The build system that produces runnable binaries from TinyGo, clang, and ESP-IDF.

---

## 1. Canal's Design Goals

Three principles guide every engineering decision in Canal.

### 1.1 Capability-Based Security on Constrained Hardware

A *capability* in Canal is an unforgeable token that grants a domain the right to perform
a specific operation on a specific resource. You cannot access GPIO without holding a
`device:gpio` capability. You cannot read from the SD card without `fs:read`. You cannot
even communicate with the Wi-Fi service without `service:wifi`.

Capabilities are tracked in a kernel-managed table (`kernel/captable.go`). The table
stores, for each capability slot:

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `CapabilityID` | Slot index (uint32) |
| `Type` | uint8 | Channel, Memory, Device, IRQ, or Service |
| `Rights` | uint32 | Bitmask: Read, Write, Execute, Grant, Revoke |
| `Owner` | `DomainID` | Domain that created this capability |
| `Target` | `unsafe.Pointer` | FreeRTOS queue handle, memory address, etc. |
| `RefCount` | uint16 | Number of domains holding a reference |

The table has a fixed maximum of 256 entries—chosen to fit in on-chip SRAM with no
dynamic allocation. Capabilities are allocated with `CapAlloc`, validated with
`CapValidate`, and cleaned up with `CapRevoke`. A domain can only revoke capabilities it
owns; trying to revoke someone else's capability returns `ErrPermissionDenied`.

### 1.2 Crash Isolation

If a domain panics, exhausts its heap, or is killed by another domain, the damage is
contained:

- The domain's FreeRTOS task is deleted (`vTaskDelete`).
- All capabilities owned by that domain are revoked (`CapRevoke` is called for each entry
  in `domain.Caps`).
- The domain's slot in the domain table is marked `DomainStateInvalid`.
- Other domains continue running undisturbed.

None of this requires a full system reset. The kernel can optionally respawn the domain
from its flash partition, giving a self-healing behaviour that would be impossible with a
traditional single-address-space RTOS firmware.

### 1.3 Go-Native APIs

TinyGo compiles a substantial subset of Go for microcontrollers. Canal uses this to give
application code a familiar Go interface—channels, goroutines, `go` statements—even
though the implementation under the hood talks to FreeRTOS queues and Canal capabilities.

---

## 2. FreeRTOS as the Scheduling Substrate

Canal does *not* write its own scheduler. Instead it layers on top of
[FreeRTOS](https://www.freertos.org/), a battle-tested RTOS with over 30 years of
production history.

### 2.1 Tasks, Queues, and Semaphores

The FreeRTOS primitives Canal uses most heavily are:

**Tasks** (`xTaskCreate`, `vTaskDelete`). Each domain runs as one or more FreeRTOS tasks.
The Canal kernel itself runs as a high-priority task that services syscall requests:

```go
// kernel/syscall.go
result := xTaskCreate(
    *(*unsafe.Pointer)(unsafe.Pointer(&fn)),
    cstring("syscall"),
    4096,           // stack depth (words)
    nil,
    3,              // priority — higher than any domain task
    &syscallTaskHandle,
)
```

**Queues** (`xQueueCreate`, `xQueueSend`, `xQueueReceive`). Every inter-domain message
goes through a FreeRTOS queue. Each domain gets two queues at spawn time: a
`SyscallQ` (the domain sends requests into this queue; the kernel consumes from it) and a
`ReplyQ` (the kernel sends responses; the domain consumes from it).

```go
// kernel/domain.go
syscallQ := xQueueCreate(8, uint32(unsafe.Sizeof(SyscallRequest{})))
replyQ   := xQueueCreate(8, uint32(unsafe.Sizeof(SyscallResponse{})))
```

This is the core IPC protocol: a domain submits a 32-byte `SyscallRequest` to
`kernelSyscallQ`, blocks on its own `ReplyQ`, and wakes when the kernel posts a
`SyscallResponse`.

### 2.2 How FreeRTOS Tasks Map to Canal Domains

One Canal domain = one FreeRTOS task (plus any goroutines spawned inside that domain,
which are scheduled cooperatively by TinyGo's own goroutine scheduler within the task).

```
┌───────────────────────────────────────────────────┐
│  FreeRTOS scheduler                               │
│                                                   │
│  Task: "syscall" (priority 3, always running)    │
│  Task: "wifi"    (priority 2, domain entry)       │
│  Task: "tls"     (priority 2, domain entry)       │
│  Task: "logger"  (priority 1, domain entry)       │
│  Task: "idle"    (priority 0, FreeRTOS idle)      │
└───────────────────────────────────────────────────┘
         │                  │
     TinyGo goroutines   TinyGo goroutines
     inside wifi domain  inside tls domain
```

### 2.3 Priority Levels and Real-Time Considerations

Canal uses `configMAX_PRIORITIES = 5`. The assignments are:

| Priority | Occupant |
|----------|---------|
| 4 (highest) | Reserved for future hard-real-time domains |
| 3 | Canal syscall handler |
| 2 | System service domains (Wi-Fi, TLS, SD card) |
| 1 | User domains (picoceci, HTTP server, logger) |
| 0 | FreeRTOS idle task |

A domain that blocks on a channel send/receive yields the CPU immediately. FreeRTOS
picks the next highest-priority runnable task. Because the syscall handler is priority 3,
any domain that invokes a syscall wakes the handler promptly without being preempted by a
lower-priority domain.

### 2.4 Why the FreeRTOS API Surface Is Hidden from User Domains

User domains written against Canal's Go API cannot call `xTaskCreate`, `xQueueSend`, or
any other FreeRTOS function directly. There are two reasons:

1. **Security**: `xTaskCreate` takes a raw function pointer and a stack size. A domain
   that can call it could spawn a task that runs in the kernel's address space or with
   the kernel's capabilities. Even if the MMU would catch the resulting memory fault, the
   attack surface grows dramatically.

2. **Portability**: Canal also targets ARM Cortex-M (where the MPU replaces the MMU) and
   will add RISC-V support. Abstracting FreeRTOS away lets the same user-domain code run
   unchanged on every backend.

---

## 3. TinyGo Runtime Inside a Domain

Each Canal domain compiled with TinyGo embeds a small runtime (`runtime/` package) that
provides goroutine scheduling, garbage collection, and channel semantics—all isolated
within the domain's heap.

### 3.1 Goroutine Scheduler

TinyGo's goroutine scheduler uses cooperative multitasking within a single FreeRTOS task.
Goroutines yield at channel operations, `time.Sleep`, and system calls. There is no
preemption between goroutines *within* a domain, but the FreeRTOS preemptive scheduler
can preempt the entire FreeRTOS task at tick boundaries, switching between domains.

### 3.2 Garbage Collector: Conservative Mark-Sweep, Per-Domain

The GC in `runtime/gc.go` runs entirely within one domain's heap. Its structure:

```
┌──────────────────────────────────────────────┐
│  gcHeap (per-domain global)                  │
│                                              │
│  base     uintptr  ← start of PSRAM region  │
│  size     uint32   ← HeapMedium (32 KB)     │
│  current  uintptr  ← bump pointer           │
│  blocks   [256]gcBlock                       │
│  markBits [256]uint64                        │
└──────────────────────────────────────────────┘
```

Allocation is a **bump pointer**—`heap.current` advances by the requested (aligned) size.
When the bump pointer hits the end of the heap, `gcCollect()` runs:

1. **Mark phase**: Scan the current goroutine stack and all goroutine stacks, treating
   every word that falls within the heap range as a potential pointer. Mark the block
   containing each such pointer.
2. **Sweep phase**: Walk the `blocks` table; any block not marked is freed. If all blocks
   are freed, the bump pointer resets to `heap.base`.

Because this GC is per-domain:

- A GC pause in Domain A has zero effect on Domain B. Domain B's FreeRTOS task keeps
  running with no pause.
- Heap corruption in a buggy domain cannot spread to another domain's memory.
- After a domain is killed, its entire PSRAM region is simply unmapped—no GC cycle needed
  to recover its memory.

### 3.3 Channel Implementation Mapped to FreeRTOS Queues

The `hchan` struct in `runtime/chan.go` is the Go channel type at runtime:

```go
type hchan struct {
    queueHandle unsafe.Pointer   // FreeRTOS queue
    elemsize    uintptr
    elemtype    unsafe.Pointer   // Type info
    closed      uint32
    capID       kernel.CapabilityID  // Non-zero ⟹ cross-domain channel
}
```

For a **local channel** (both sender and receiver in the same domain), `chanSend` and
`chanRecv` call `xQueueSend`/`xQueueReceive` directly on the embedded FreeRTOS queue.
Blocking is free—the goroutine yields until the FreeRTOS queue is non-empty.

For a **cross-domain channel** (`capID != 0`), the send/receive becomes a capability
syscall. The kernel validates the capability, copies the message through the associated
FreeRTOS queue, and returns.

### 3.4 Memory Layout Per Domain

```
PSRAM region for Domain N
┌─────────────────────────────────┐ ← HeapStart (domain.HeapStart)
│  TinyGo goroutine stacks        │
├─────────────────────────────────┤
│  TinyGo heap (bump allocator)   │
│  gcHeap.base = HeapStart        │
│  gcHeap.current (grows →)       │
├─────────────────────────────────┤
│  Static globals                 │
└─────────────────────────────────┘ ← HeapStart + HeapSize
```

Stacks and heap share the same PSRAM allocation. The domain declares its total budget
at spawn time using the `HeapTiny/Small/Medium` constants (or a custom value); the kernel
calls `make([]byte, heapSize)` and stores the slice in `domain.Heap` to keep it alive in
the GC.

---

## 4. The Kernel Substrate

### 4.1 Capability Table: Structure, Lookup, Revocation

The global `capTable [256]Capability` lives in kernel DRAM and is protected by a
spinlock. Operations:

| Function | What it does |
|----------|-------------|
| `CapAlloc(owner, type, target, rights)` | Finds a free slot, fills it, returns `CapabilityID` |
| `CapValidate(capID, requiredRights)` | Checks the slot is valid and the rights bitmask is satisfied |
| `CapGrant(capID, granter, grantee)` | Adds the capability to the grantee's `domain.Caps` array; increments `RefCount` |
| `CapRevoke(capID, revoker)` | Decrements `RefCount`; if zero, marks slot `CapTypeInvalid` |
| `CapSend` / `CapRecv` | Validated message passing through the associated FreeRTOS queue |

Every domain can hold up to 16 capabilities simultaneously (the `Caps [16]CapabilityID`
field in the `Domain` struct). Exceeding this limit returns `ErrCapTableFull`.

### 4.2 Domain Lifecycle: Create → Load → Start → Crash → Restart

```
DomainSpawn (Go entry) ─────────────────────────────────────────────▶ DomainStateRunning
    or
SpawnDomainFromFlash (ELF entry) ───────────────────────────────────▶ DomainStateRunning
                                                                             │
                                                          normal operation   │
                                                                             ▼
                                                               DomainStateSuspended
                                                             (vTaskSuspend called)
                                                                             │
                                                                             ▼
                         DomainKill ──── capabilities revoked ──▶ DomainStateInvalid
                                                                  FreeRTOS task deleted
                                                                             │
                                                              (optional)    ▼
                                                          SpawnDomainFromFlash again
```

There are two spawn paths:

- **`DomainSpawn`** takes a Go function pointer (`entry func()`) and launches it as a
  goroutine. Used for domains compiled into the kernel binary.
- **`SpawnDomainFromFlash`** reads an ELF binary from a flash partition
  (`domainPartitions` map), copies PT_LOAD segments into RAM, and creates a FreeRTOS task
  at the ELF entry point. Used for separately compiled domain images.

### 4.3 Syscall Interface: How a Domain Makes a Kernel Request

The syscall protocol is a synchronous request/response over two FreeRTOS queues:

```
Domain N                              Kernel (syscall task)
─────────────────────────────────────────────────────────────────
1. Fill SyscallRequest{Op, DomainID, CapID, Arg0..3, DataPtr, DataLen}
2. xQueueSend(kernelSyscallQ, &req, portMAX_DELAY)
                                      3. xQueueReceive(kernelSyscallQ, &req, ...)
                                      4. switch req.Op { ... }
                                      5. xQueueSend(domain.ReplyQ, &resp, ...)
6. xQueueReceive(domain.ReplyQ, &resp, portMAX_DELAY)
7. Check resp.Error
```

The nine syscall opcodes (defined in `kernel/types.go`) are:

| Opcode | Purpose |
|--------|---------|
| `SysCapRequest` | Ask the kernel for a named capability |
| `SysCapGrant` | Delegate a capability to another domain |
| `SysCapRevoke` | Take back a capability |
| `SysCapSend` | Send a message via capability channel |
| `SysCapRecv` | Receive a message via capability channel |
| `SysMemAlloc` | Allocate memory from kernel GC heap for this domain |
| `SysDomainSpawn` | Spawn a child domain |
| `SysDomainKill` | Kill a domain |
| `SysDebugPrint` | Write to kernel debug console |

### 4.4 MMU / MPU Configuration Per Domain

Each domain has an `MPUConfig` struct with three region descriptors: code, data, and heap.
On the ESP32-S3 these map to MMU page-table entries controlled by the kernel. The
`HeapStart` field in `Domain` records the physical address of the domain's heap slice so
the kernel can set up the correct page-table entry when the domain is scheduled.

On ARM Cortex-M targets the same `MPUConfig` struct drives the hardware MPU. The
abstraction is the same; only the register-level setup differs.

---

## 5. Inter-Domain Communication

### 5.1 Capability-Mediated Channels

A cross-domain channel in Canal is not just a FIFO—it is a *capability-guarded* FIFO.
Before any message can be sent, `CapValidate` checks that:

1. The capability slot is valid (`CapTypeInvalid` fails immediately).
2. The caller holds the required rights (`RightWrite` for send, `RightRead` for receive).

Only then does the actual `xQueueSend`/`xQueueReceive` call proceed. This means a domain
cannot "guess" a channel handle and send noise to a service—the capability check happens
inside the kernel, which the domain cannot bypass.

### 5.2 Message Passing Protocol and Zero-Copy Optimizations

In the current implementation, messages are copied into the FreeRTOS queue (queue items
are fixed-size structs stored by value). For small messages (≤ 64 bytes, matching the
cache line) this copy is cheap—typically three to five store instructions on the LX7 core.

For larger messages (TLS records, HTTP payloads) a zero-copy path is possible: instead of
copying the payload, the sender places it in a memory region both domains can read, and
only passes a *pointer + length* through the queue. This requires the kernel to map a
shared-memory capability between the two domains (type `CapTypeMemory` with both `RightRead`
and `RightWrite`). Without that capability check, one domain could pass a pointer into
another domain's private heap—a classic cross-domain pointer-aliasing vulnerability. Canal
prevents this by requiring an explicit memory-sharing capability grant before any pointer
can be treated as valid by the receiving domain.

### 5.3 Example: Wi-Fi → TLS → HTTP Domain Message Flow

The following ASCII sequence diagram shows how an HTTP request travels through three
domains. Each `─►` is a channel send; the label names the capability exercised.

```
HTTP domain          TLS domain           Wi-Fi domain       Kernel
    │                    │                     │                │
    │──[service:tls]──►  │                     │                │
    │  WriteRequest       │                     │                │
    │  {plaintext HTTP}   │                     │                │
    │                    │  OpHandshake /       │                │
    │                    │  OpWrite (encrypt)   │                │
    │                    │──[service:wifi]──►   │                │
    │                    │  SocketSendRequest   │                │
    │                    │  {ciphertext}        │                │
    │                    │                     │  send TCP      │
    │                    │                     │  segment       │
    │                    │                     │  (hardware)    │
    │                    │                     │                │
    │  ◄──────────────── │ ◄─────────────────  │                │
    │  WriteResponse      │  SocketSendResponse │                │
    │  {bytes_written}    │  {bytes_sent}       │                │
```

Each `─►` crosses a domain boundary via a capability syscall. The HTTP domain holds
`service:tls` but NOT `service:wifi`—it cannot reach the network directly. The TLS domain
holds `service:wifi` but stores no application data—it only encrypts/decrypts buffers.
This minimal-privilege architecture means a compromise of the HTTP domain cannot exfiltrate
TLS private keys (which never leave the TLS domain) and cannot talk to the network without
going through the TLS layer.

---

## 6. Build System and Toolchain

### 6.1 How TinyGo, clang, and ESP-IDF Combine

Canal's kernel and domains are written in Go and compiled with TinyGo, which uses LLVM
(via `clang`) to produce Xtensa machine code. The ESP-IDF FreeRTOS library is linked in
as a pre-built C archive (`libfreertos.a`), bridged to Go via TinyGo's `//export` pragma
and `//go:linkname` directives.

```
Canal Go source
      │
      ▼
   TinyGo (0.31+)
      │   LLVM / clang
      ▼
  canal.o  (Xtensa ELF)
      │
      ├─── libfreertos.a   (ESP-IDF FreeRTOS)
      ├─── libesp_wifi.a   (Espressif Wi-Fi stack)
      └─── libnewlib.a     (C runtime)
      │
      ▼
  kernel.elf  →  esptool.py  →  ESP32-S3 flash
```

Setting `USE_IDF=1` (the default) adds `-tags=idf` and links the real ESP-IDF FreeRTOS.
Setting `USE_IDF=0` substitutes goroutine-based stub implementations so the kernel can be
compiled and partially tested on a host machine without hardware.

### 6.2 Makefile Targets

The `canal/Makefile` provides these primary targets:

| Target | What it does |
|--------|-------------|
| `make build` | Compile kernel ELF only (`build/out/kernel.elf`) |
| `make build-domains` | Compile LED, Wi-Fi, and Logger domain ELFs |
| `make flash` | `tinygo flash` kernel to auto-detected USB port |
| `make flash-domains` | Flash all domain ELFs to their flash partitions with `esptool.py` |
| `make flash-all` | Kernel flash followed by domain flash (full install) |
| `make flash-led/wifi/logger` | Rebuild and flash a single domain only |
| `make picoceci` | Compile the picoceci domain (`-gc=leaking` for minimal code size) |
| `make picoceci-flash` | Flash picoceci to the board |
| `make monitor` | Open serial monitor at 115200 baud |
| `make run` | `flash-all` then `monitor` in sequence |
| `make clean` | Delete `build/out/` |

### 6.3 Flashing and Debugging Workflow

**First-time setup**:

```bash
cd Canal/canal
chmod +x scripts/setup.sh
./scripts/setup.sh esp32s3      # downloads FatFS, mbedTLS
make flash-all                  # kernel + LED + Wi-Fi + Logger
make monitor                    # open serial console
```

**Iterating on a single domain** (no kernel re-flash needed):

```bash
# After editing domains/wifi/…
make flash-wifi                 # re-flash just the Wi-Fi partition
make monitor                    # observe output
```

**JTAG debugging** (OpenOCD + GDB):

```bash
openocd -f board/esp32s3-builtin.cfg
# In a second terminal:
xtensa-esp32s3-elf-gdb build/out/kernel.elf
(gdb) target remote :3333
(gdb) monitor reset halt
(gdb) continue
```

---

## Summary

Canal is a thin orchestration layer over FreeRTOS: it adds capability-based access
control, per-domain heap isolation (backed by the ESP32-S3 MMU and PID controller), and a
clean Go API that hides the RTOS internals. Each domain runs as a FreeRTOS task with its
own TinyGo runtime instance—its own goroutine scheduler, its own GC, and its own section
of PSRAM. The kernel's syscall handler mediates every cross-domain message through
capability validation, ensuring no domain can exceed the permissions it was granted.

In [Article 4](./04-picoceci-on-canal-programming-the-microkernel.md) we focus on the
picoceci domain specifically: how the interpreter requests capabilities, how scripts issue
channel operations that become Canal syscalls, and how to debug a running system with the
capability inspector.

---

## Exercises

1. **FreeRTOS abstraction.** In Canal, FreeRTOS tasks are the underlying mechanism, but
   user domains never call FreeRTOS APIs directly. Explain why hiding the FreeRTOS surface
   is a deliberate design choice. What security property would be lost if user domains
   could call `xTaskCreate` themselves?

2. **Per-domain GC.** TinyGo's garbage collector runs per-domain rather than globally.
   Sketch (in prose or pseudocode) how a GC pause in Domain A could affect Domain B in a
   *shared-heap* model, and then explain why Canal's per-domain GC prevents this problem.

3. **Capability revocation in flight.** Describe what is stored in Canal's capability
   table for a domain that has been granted `service:wifi`. What happens—step by step—when
   that domain's capability is revoked while a Wi-Fi operation is in flight?

4. **Zero-copy safety.** The article mentions a zero-copy optimization for inter-domain
   messages. Explain what "zero-copy" means in this context, under what conditions Canal
   can safely use it, and why a naive implementation without capability checking would be a
   security hole.

5. **Message flow diagram.** Draw a sequence diagram (ASCII or prose) showing the full
   message flow described in the article: Wi-Fi domain → TLS domain → HTTP domain. For
   each arrow, note which capability is exercised and what data crosses the domain
   boundary.
