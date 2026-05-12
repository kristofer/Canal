# WiFi Picoceci FS Bridge Implementation Plan

## Current State

- WiFi picoceci REPL now has a minimal local capability shim:
  - `Canal capability: #fsRead`
  - `Canal capability: #fsWrite`
  - `fs list:`
  - `fs readFile:`
  - `fs writeFile:data:`
  - `fs exists:`
- The resolver in `domains/wifi/cmd/esp32s3/interpreter.go` now calls `readModuleFromFS(...)`.
- `readModuleFromFS(...)` is currently backed by a local in-memory FS shim in `domains/wifi/cmd/esp32s3/canal_globals.go`.

## Goal

Replace the local in-memory shim with real `service:fs` operations so WiFi picoceci reads and writes through the SD card domain using Canal capabilities.

## Phase 1: Domain-Local Capability Shim (Kernel Syscall Bridge)

1. Add a tiny WiFi-domain syscall shim package or file:
   - Parse `domain_entry` task parameter as kernel `DomainParams`.
   - Capture `DomainID`, `SyscallQ`, and `ReplyQ`.
2. Mirror minimal syscall structs/constants locally in WiFi domain:
   - `SyscallRequest`, `SyscallResponse`
   - `SysCapRequest`, `SysCapSend`, `SysCapRecv`
   - `ErrNone`, `RightRead`, `RightWrite`
3. Bind to queue symbols directly in WiFi domain:
   - `xQueueGenericSend`
   - `xQueueReceive`
   - `xQueueGenericCreate`
   - `vQueueDelete`
4. Implement local capability helpers:
   - `capRequest(name, rights) -> capID`
   - `capSend(capID, ptr, len)`
   - `capRecv(capID, ptr, len)`

Verification:

- A smoke function in WiFi domain can request `service:fs` and returns success/failure deterministically.

## Phase 2: Shared FS Protocol Layer for WiFi Domain

1. Extract/copy protocol structs/constants from `stdlib/fs/common.go` to a WiFi-local protocol file:
   - message envelope, op codes, payload structs, limits.
2. Keep this protocol layer free of `runtime` and `kernel` package imports.
3. Implement request helper:
   - create temporary reply queue
   - send FS message through `capSend`
   - wait for response via reply queue with timeout

Verification:

- Unit-style in-domain checks for encode/decode and response parsing.

## Phase 3: Replace Local FS Shim with Real `service:fs` Calls

1. In `canal_globals.go`, replace local file operations:
   - `list:` -> `OpList`
   - `readFile:` -> `OpOpen + OpRead + OpClose`
   - `writeFile:data:` -> `OpOpen + OpWrite + OpSync + OpClose`
   - `exists:` -> `OpStat`
2. Keep method selectors and object model unchanged so picoceci scripts continue to work.
3. Make `readModuleFromFS(path)` call real FS read path.

Verification:

- In WiFi REPL:
  - `fs := Canal capability: #fsRead.`
  - `fs list: '/'.`
  - `fs readFile: '/path'.`
  - write/read round-trip with `#fsWrite`.

## Phase 4: Kernel and SD Service Alignment

1. Confirm `service:fs` request path in kernel capability lookup is fully wired for requestors.
2. Keep SD domain boot order before WiFi (`sdcard`, then `wifi`).
3. Validate SD domain ACL grants for WiFi domain IDs.
4. Add deterministic logging for capability request failures.

Verification:

- Boot logs show SD service ready before WiFi REPL accepts clients.
- WiFi REPL FS operations succeed without fallback shim.

## Phase 5: Cleanup and Hardening

1. Remove local in-memory FS fallback from WiFi shim.
2. Add clear error mapping for:
   - permission denied
   - not found
   - timeout
   - service unavailable
3. Add small picoceci integration scripts under `domains/sdcard/` for regression checks.

Verification:

- Build passes (`make build`, `make wifi`).
- Manual REPL script passes read/list/write scenarios on real SD card.
