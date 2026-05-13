# Typed Channel Plan (Replacing Capability + Heavy Syscall Paths)

## Problem

The current capability/syscall model is heavier than needed for the short-term Canal goal:
run multiple domain tasks and let picoceci communicate with services quickly and reliably.
We currently maintain overlapping paths (capability table, syscall request routing, and
domain-local shims), which slows iteration.

## Proposal Summary

Move to **typed picoceci channels** plus a **small io_uring-style queue pair**:

1. **Typed channels as the public model** (what picoceci sees)
2. **Shared submission/completion queue mechanics** (how requests are transported)
3. **Static boot wiring policy** (which domain can open which service channel)

This keeps queue IPC and domain isolation while removing most capability formalism.

## Core Design

### 1) Typed picoceci channels

- Replace capability acquisition APIs with typed channel opens:
  - `Canal openChannel: #fs.`
  - `Canal openChannel: #wifi.`
- Note: the `openChannel:` examples above use picoceci selector syntax.
- Each channel has a declared schema/version and fixed message object types:
  - `FSRequest` / `FSResponse`
  - `WiFiRequest` / `WiFiResponse`
  - `TLSRequest` / `TLSResponse`
- picoceci-facing calls remain object-centric (`send:`/`receive:`), but runtime validates
  the message type before enqueue.

### 2) Simplified io_uring-style transport

Per requesting domain:

- `SQ` (submission queue): request descriptors written by caller runtime
- `CQ` (completion queue): completion descriptors written by service runtime

Descriptor shape (conceptual):

- `op` (typed operation enum)
- `service` (fs/wifi/tls/etc.)
- `reqPtr`, `reqLen`
- `cookie` (caller correlation ID)
- completion: `status`, `respPtr`, `respLen`, `cookie`

This gives async-friendly behavior with fixed queue semantics and avoids expanding
syscall op surface area.

### 3) Kernel role after change

- Keep kernel as queue/router/bootstrap authority.
- Remove capability grant/revoke and capability-right rights negotiation from hot path.
- Keep only minimal control operations needed for:
  - domain startup,
  - channel registry lookup,
  - queue wiring and health checks.

## Migration Phases

### Phase 1 — Define typed channel contracts

1. Add service channel registry entries (`fs`, `wifi`, `tls`) with schema IDs.
2. Define shared request/response structs and operation enums.
3. Add runtime validation helpers for typed send/recv boundaries.

Exit criteria:

- One source of truth exists for channel type definitions.
- Existing stdlib packages can compile against the new type contracts.

### Phase 2 — Introduce SQ/CQ transport alongside current flow

1. Implement per-domain SQ/CQ creation and descriptor helpers.
2. Add service-side completion publishing.
3. Bridge current blocking APIs to SQ submit + CQ wait.

Exit criteria:

- FS and WiFi operations can run through SQ/CQ with equivalent behavior.
- Existing callers can still use current blocking APIs.

### Phase 3 — Switch picoceci and stdlib to typed channels

1. Replace capability-centric calls in picoceci paths with `openChannel`.
2. Route `stdlib/fs`, `stdlib/wifi`, `stdlib/tls` through typed channel wrappers.
3. Remove WiFi domain local capability shim usage.

Exit criteria:

- picoceci REPL I/O path no longer needs capability-request semantics.
- `readModuleFromFS(...)` uses typed FS channel path.

### Phase 4 — Shrink syscall/capability surface

1. Remove unused capability grant/revoke request handling.
2. Keep only minimal kernel channel routing/control syscalls.
3. Add deterministic errors for channel-open/channel-send/channel-recv failures.

Exit criteria:

- Capability table is no longer part of the normal service I/O path.
- Syscall layer is reduced to bootstrap/control rather than service transport.

### Phase 5 — Validate fast path for picoceci over WiFi

1. Ensure provider-before-consumer boot order (`sdcard`, `wifi`, then consumers).
2. Validate TCP REPL with module reads/writes over typed FS channel.
3. Capture one end-to-end smoke script for regression.

Exit criteria:

- picoceci over WiFi works with typed channels + SQ/CQ path.
- No domain-local capability shim is required.

## Risk Controls

- Keep wire payload formats stable where possible during migration.
- Gate removal behind compatibility wrappers until end-to-end smoke passes.
- Validate each phase with `make build`, targeted domain builds, and runtime smoke checks
  (boot sequence, service registration, and basic channel open/send/recv behavior).
