# Capability Removal Plan (Channel-First Canal)

## Problem

Capabilities currently add complexity without strong enforcement value in the present
Canal architecture. The implementation also has split paths (kernel capability table,
runtime capability API, and domain-local shims), which slows down getting a reliable
picoceci-on-WiFi workflow.

## Current Gaps to Resolve First

1. `runtime/cap.go` still has partial/stub behavior (`marshal`/`unmarshal` TODO path),
   so only some capability flows are practical.
2. `kernel/syscall.go` capability lookup is still simplified and not service-complete.
3. WiFi has a domain-local capability shim (`domains/wifi/cmd/esp32s3/capability_shim.go`),
   which duplicates syscall contracts and creates another maintenance surface.
4. picoceci FS access is still described and structured around capability acquisition.

## Target Architecture (Post-Removal)

- Keep domain isolation and queue-based IPC.
- Replace user-facing capability acquisition with explicit service channels:
  - `service.Open("fs")`, `service.Open("wifi")`, etc.
- Enforce access by static wiring + boot policy (which domains get which service handles),
  not by dynamic grant/revoke semantics.
- Keep message protocols unchanged where possible to minimize risk.

## Phased Execution Plan

### Phase 1 — Introduce Service Handle API (No Behavior Break)

1. Add a channel-first runtime API that opens named services directly.
2. Internally map existing stdlib clients (`stdlib/fs`, `stdlib/wifi`, `stdlib/tls`) to
   the new API.
3. Keep compatibility wrappers for existing capability calls so old code still builds.

Exit criteria:

- Existing demos build and run unchanged.
- `stdlib/*` no longer depend on new capability requests for core operations.

### Phase 2 — Replace Domain-Local Capability Shims

1. Remove WiFi domain capability shim usage and switch to service handle API.
2. Keep WiFi picoceci selectors (`fs list:`, `fs readFile:`, etc.) unchanged.
3. Route `readModuleFromFS(...)` through the same service-handle-backed FS path.

Exit criteria:

- No domain-local capability syscall mirror remains for WiFi.
- FS operations in WiFi picoceci path use one shared service access path.

### Phase 3 — Kernel Simplification

1. Replace dynamic capability request/grant/revoke flow with service registry + channel
   lookup only.
2. Remove capability-rights branching that is no longer used by callers.
3. Keep queue IPC send/recv implementation and domain reply queues intact.

Exit criteria:

- Kernel syscall surface is service/channel oriented.
- `CapGrant`/`CapRevoke` path is removed or fully deprecated behind compatibility gates.

### Phase 4 — Picoceci Fast Path Completion

1. Ensure boot order guarantees providers before consumers (`sdcard` before WiFi/picoceci
   consumers).
2. Make service-open failures deterministic and visible in console logs.
3. Validate the known-good flow: boot, WiFi connect, TCP REPL, module import from FS.

Exit criteria:

- Running picoceci over WiFi no longer depends on capability-specific setup.
- End-to-end script load + file read/write works on device.

### Phase 5 — Cleanup + Documentation

1. Remove remaining capability-centric language from runtime/stdlib/domain docs.
2. Keep one migration note for old scripts and APIs.
3. Update educational material to focus on task/channel model.

Exit criteria:

- Repository docs reflect channels/services as the primary model.
- Capability model is clearly marked removed/deprecated.

## Risk Controls

- Do not change message wire formats unless required.
- Keep backward-compatible wrappers until Phase 4 validation is complete.
- Validate on-device after each phase (`make build`, domain build targets, and REPL smoke
  checks).
