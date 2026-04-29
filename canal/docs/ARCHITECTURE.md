# Canal Architecture

## Overview

Canal is a capability-based microkernel operating system designed for embedded systems.

## Core Components

### Kernel Substrate
- Minimal trusted computing base
- Capability table management
- Domain lifecycle management
- MMU/MPU configuration
- Syscall interface

### Domains
Isolated execution environments with:
- Dedicated memory regions (MMU/MPU protected)
- Independent Go runtime and GC
- Set of capabilities
- One or more goroutines

### Capabilities
Unforgeable tokens granting specific access rights:
- Cannot be forged (kernel-managed)
- Cannot be guessed (large ID space)
- Explicitly transferred
- Can be revoked

### Channels
Go channels serve as universal IPC:
- Type-safe message passing
- Maps to FreeRTOS queues
- Cross-domain communication
- Blocking/non-blocking modes

## Memory Protection

### ARM Cortex-M (MPU)
8-16 configurable regions per domain:
- Region 0: Kernel (privileged)
- Region 1: Code (user RX)
- Region 2: Data (user RW, NX)
- Region 3: Heap (user RW, NX)

### ESP32-S3 (MMU)
Page-based (4KB pages):
- Virtual address spaces per domain
- Hardware page tables
- TLB for performance
- PID support

### RISC-V (PMP)
8-16 physical protection entries:
- Similar to MPU
- Naturally aligned regions
- Simpler than MMU

## Security Properties

### Hardware-Enforced
- Memory isolation
- Privilege separation
- Execute protection

### Kernel-Enforced
- Capability unforgability
- No ambient authority
- Explicit grant/revoke

### Design-Enforced
- Least privilege
- Crash isolation
- Resource cleanup

## Performance

| Operation | Cycles |
|-----------|--------|
| Context switch (ARM) | ~10 |
| Context switch (ESP32) | ~100 |
| Same-domain channel | ~50 |
| Cross-domain channel | ~300 |

## Future Work

- Multi-core support
- Real-time guarantees
- Formal verification
- Persistent capabilities
