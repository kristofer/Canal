# Article 4: picoceci on Canal — Programming the Microkernel

> **Canal version**: 0.1.0-dev | **picoceci version**: 0.1.0-dev
> **Target audience**: Developers who have read Articles 2 and 3 and want to use picoceci
> for real tasks on Canal.
> **Prerequisites**: [Articles 1–3](../EDUCATIONAL_PLAN.md).

---

## Introduction

Articles 2 and 3 built two mental models: picoceci the language, and Canal the
microkernel. This article brings them together. picoceci runs *as a Canal domain*—it is
not a privileged interpreter but an ordinary user program subject to the same capability
rules as any other code on the system. What makes picoceci special is that its runtime
and REPL make those capability rules feel natural and interactive rather than burdensome.

By the end of this article you will be able to:

- Understand how the picoceci domain is structured and what resources it uses.
- Know which capabilities a picoceci script can request and what each one permits.
- Write picoceci scripts that open channels to system service domains.
- Hot-patch running code without a reboot.
- Work through three complete hardware examples.
- Explain what picoceci cannot do and why that is a feature.

---

## 1. The picoceci Domain

### 1.1 Domain Manifest: Capabilities at Startup

The picoceci domain is a regular Canal domain compiled as a separate TinyGo binary. It is
spawned by the Canal kernel at boot using `SpawnDomainFromFlash`, which loads the ELF
from the picoceci flash partition and creates a FreeRTOS task at the ELF entry point.

Before the interpreter enters its REPL loop, the domain requests the capabilities it needs
for the interactive session. A stripped-down view of the startup sequence
(`domains/picoceci/main.go`):

```go
func main() {
    time.Sleep(2 * time.Second)  // Wait for USB CDC to stabilise
    println("[picoceci] Starting v" + version + " (Canal domain)")

    // Set up module resolver (wires to Canal FS capability once sdcard domain is ready)
    resolver := module.NewResolver(func(path string) ([]byte, error) {
        return nil, errNoFS
    })
    module.RegisterBuiltins(resolver)
    loader := module.NewLoader(resolver)

    println("[picoceci] Ready.")
    runREPL(loader)
}
```

Under the hood, before any user script can evaluate `GPIO pin: 2 direction: #output`, the picoceci runtime
calls `runtime.RequestCap("device:gpio", RightRead|RightWrite)`. The kernel looks up the
capability in its registry, creates a `CapTypeChannel` slot backed by a FreeRTOS queue,
and returns a `CapabilityID`. That ID is stored in the domain's `Caps` array. No user
script can forge or bypass this ID—it only exists if the kernel granted it.

### 1.2 Memory Budget

The picoceci domain is built with `-gc=leaking` (`Makefile` target `picoceci`), which
disables the GC and uses a simple bump allocator. This is appropriate because:

- Each REPL expression creates a short-lived value graph that can be discarded after
  `vm.Run` returns.
- Without GC bookkeeping overhead, the interpreter fits in the `HeapMedium` (32 KB)
  budget comfortably.
- The VM is re-created from scratch for each expression (`bytecode.NewVM()` inside
  `runREPL`), so every allocation made during evaluation is implicitly freed when the
  bump pointer is reset.

Memory breakdown for a typical interactive session:

| Component | Approximate size |
|-----------|-----------------|
| Interpreter binary (code) | ~150 KB in flash |
| REPL loop stack | 4 KB |
| Bytecode compiler + VM | 8 KB |
| Per-expression value heap | up to 16 KB |
| FreeRTOS task overhead | 4 KB |

### 1.3 Lifecycle: Boot → REPL → Error Recovery

```
Canal kernel boots
      │
      ▼
picoceci domain spawned (SpawnDomainFromFlash)
      │
      ▼
main(): sleep 2s, request capabilities
      │
      ▼
runREPL(loader)
      │
      ▼ loop:
  1. print "> "
  2. console.ReadLine() — blocks until USB serial input
  3. lexer.NewString(line)
  4. parser.New(l).ParseProgram()     ← parse error → print + continue
  5. bytecode.NewCompilerWithLoader(loader).Compile(...)
                                      ← compile error → print + continue
  6. bytecode.NewVM().Run(chunk)      ← runtime error → print + continue
  7. print result
  8. goto 1
```

Critically, a runtime error in step 6 does **not** crash the domain. The VM returns an
`error` value; `runREPL` prints it and loops back to step 1. The domain keeps running.
The Canal kernel only gets involved if the domain itself panics (out-of-memory, hardware
fault)—in which case `DomainKill` cleans up the slot and the other domains are unaffected.

---

## 2. Capabilities picoceci Can Hold

picoceci scripts interact with hardware and network services through a set of named
capabilities. Each capability type corresponds to a `CapTypeChannel` capability backed by
a FreeRTOS queue connected to the appropriate system service domain.

### `service:wifi` — Network Connections

Grants the ability to initiate and tear down TCP/UDP connections and perform DNS lookups.
The Wi-Fi domain (`domains/wifi/`) exposes operations via `WiFiMessage` structs:

- `OpConnect` — associate with an access point
- `OpDisconnect` — disconnect cleanly
- `OpCreateSocket` — open a TCP or UDP socket
- `OpSocketSend` / `OpSocketRecv` — transfer data
- `OpGetIP` — query the assigned IP address

picoceci scripts *never* touch the Wi-Fi hardware registers. The capability channel
carries only `ConnectRequest` / `ConnectResponse` messages; the Wi-Fi domain enforces all
hardware-level constraints.

### `service:tls` — Encrypted Channels

Grants the ability to establish TLS sessions and encrypt/decrypt data. The TLS domain
(`domains/tls/`) holds the private key material. A picoceci script can call `OpHandshake`
and `OpWrite`/`OpRead` but cannot request `OpLoadPrivateKey`—that opcode is only
accessible to the kernel during TLS domain initialisation. A compromise of the picoceci
domain cannot exfiltrate TLS private keys.

### `device:gpio` — General-Purpose I/O Pins

Grants the ability to configure a GPIO pin as input or output and read or write its logic
level. The capability channel carries small 32-byte messages: `{pin: uint8, value: bool}`
for writes, `{pin: uint8}` / `{value: bool}` for reads. GPIO register accesses happen
inside the GPIO service domain, not in the picoceci domain.

### `device:uart` — Serial Peripherals

Grants read/write access to a UART peripheral (distinct from the USB CDC console). Used
for talking to sensors, GPS modules, or other boards over serial.

### `fs:read` / `fs:write` — SD Card Files

Grants read-only or read-write access to the SD card filesystem managed by the SDCard
domain. File paths are restricted to the domain's allowed prefix; the SDCard domain
enforces path validation.

### Custom Capabilities

Any Canal domain can register a capability by allocating a `CapTypeService` entry and
advertising its name. A picoceci script requests the capability by name just like any
built-in one:

```picoceci
| kv r |
kv := Canal capability: #keyvalue.
kv send: (object { op := 'set'. key := 'count'. value := 42 }).
r := kv receive: 64.
Console println: r ok printString.
"true"
```

---

## 3. Capability-Based I/O in picoceci Programs

### 3.1 Acquiring a Canal Capability

`Canal capability: aSymbol` is the picoceci expression for requesting a Canal capability.
Under the hood it calls `runtime.RequestCap`; it returns a capability object that wraps
a `CapabilityID`:

```picoceci
| wifi |
wifi := Canal capability: #wifi.
```

If the capability is not registered (the Wi-Fi domain is not running, or the picoceci
domain was not granted it at startup), `Canal capability:` returns an error object that
the REPL prints immediately.

### 3.2 Sending Typed Messages

picoceci sends structured messages to capability objects using `send:` with an object
literal. The runtime serializes the object before handing it to `kernel.CapSend`:

```picoceci
wifi send: (object {
    op := 'connect'.
    ssid := 'MyNetwork'.
    password := 'secret'.
    timeout := 10000
  }).
```

On the receiving end (inside the Wi-Fi domain), the bytes are deserialized back into a
`ConnectRequest` struct and processed.

### 3.3 Receiving Responses and Handling Errors

`cap receive: n` blocks the goroutine-like task until the service domain posts a reply
(reading up to `n` bytes). The result is a picoceci object:

```picoceci
| result |
result := wifi receive: 256.
(result ok)
    ifTrue:  [ Console println: 'Connected, IP: ', result ip printString ]
    ifFalse: [ Console println: 'Failed: ', result error printString.
               wifi close ].
```

`wifi close` marks the capability closed on the picoceci side and decrements the
capability's `RefCount`. If `RefCount` reaches zero, the kernel frees the capability slot.

### 3.4 Concurrency: Running Multiple Tasks in picoceci

`Task spawn: aBlock` creates a lightweight FreeRTOS task running the block concurrently:

```picoceci
| blinkTask |
blinkTask := Task spawn: [
    | led |
    led := GPIO pin: 2 direction: #output.
    [ true ] whileTrue: [
        led high.
        Task delay: 500.
        led low.
        Task delay: 500
    ]
  ].
blinkTask name: 'blinker'.
```

While `blinkTask` runs in the background, the REPL remains responsive. Both tasks run
inside the single picoceci FreeRTOS task via cooperative scheduling—a `Task delay:` or
capability receive yields the goroutine, allowing the other to run.

---

## 4. Live Coding on a Running System

### 4.1 Defining New Behaviour in the REPL

Because every REPL expression is compiled and run independently, a block or object
defined in one expression is immediately available in the next:

```picoceci
| greet |
greet := [ :name | 'Hello, ', name, '!' ].
greet value: 'Canal'.
"Hello, Canal!"
greet value: 'picoceci'.
"Hello, picoceci!"
```

The block is stored in the REPL's global namespace and persists across REPL expressions
for the lifetime of the session. Object templates (defined with `object`) are also
globally available from the moment they are evaluated.

### 4.2 Hot-Patching a Running Program

Reassigning a variable that a running task reads on each iteration takes effect
immediately on the next iteration — no reboot, no domain restart required:

```picoceci
| blinkRate led blinkTask |
blinkRate := 500.              "original: 500 ms"
led := GPIO pin: 2 direction: #output.
blinkTask := Task spawn: [
    [ true ] whileTrue: [
        led toggle.
        Task delay: blinkRate  "reads blinkRate on each iteration"
    ]
  ].

blinkRate := 100.              "hot-patch: task picks this up on next iteration"
```

Because the block captures `blinkRate` by reference, the running task sees the updated
value the next time it evaluates `Task delay: blinkRate`. This is fundamentally different
from recompiling C firmware: no flash erase, no domain restart required.

> **Important caveat**: Hot-patching a block that is in the *middle* of execution
> (currently on the call stack) will not affect the current invocation—only future calls.
> This is expected behaviour, not a bug.

### 4.3 Debugging with the Capability Inspector

Canal exposes a `SysDebugPrint` syscall that picoceci scripts can use to print kernel
state. A built-in REPL command `!caps` lists the current domain's capability slots:

```
> !caps
Domain 5 (picoceci) — 3 capabilities:
  [0] CapID=12  type=Channel  rights=RW  → service:wifi
  [1] CapID=15  type=Channel  rights=RW  → device:gpio
  [2] CapID=19  type=Channel  rights=R   → fs:read
```

This lets you verify at a glance which capabilities are active, whether a capability has
been revoked (it disappears from the list), and what rights each one carries.

---

## 5. Worked Examples

The following examples are illustrative; code samples show the picoceci idioms and Canal
capability interactions. Listings marked *(illustrative)* have not been run on hardware.

### 5.1 Blink an LED and Log the State to a File

```picoceci
"Requires: device:gpio, fs:write"

| led log |
led := GPIO pin: 2 direction: #output.
log := Canal capability: #fsWrite.

10 timesRepeat: [
    led high.
    log send: (object { path := '/log.txt'. data := 'LED ON' }).
    Task delay: 500.
    led low.
    log send: (object { path := '/log.txt'. data := 'LED OFF' }).
    Task delay: 500
].
```

Each `send:` is a capability syscall. The kernel validates the GPIO write right before
setting the pin; the SDCard domain validates the write path before appending to the file.
Neither operation is possible without the corresponding capability.

### 5.2 Fetch a URL over HTTPS and Display the Response

*(illustrative — requires `#wifi` and `#tls` capabilities)*

```picoceci
"Step 1: Connect to Wi-Fi"
| wifi connResult |
wifi := Canal capability: #wifi.
wifi send: (object { op := 'connect'. ssid := 'MyNetwork'. password := 'secret'. timeout := 10000 }).
connResult := wifi receive: 256.
(connResult ok) ifFalse: [ Console println: 'WiFi failed: ', connResult error printString ].

"Step 2: Open a TCP socket (via Wi-Fi capability)"
wifi send: (object { op := 'create_socket'. protocol := 'tcp' }).
| sockResult sockId |
sockResult := wifi receive: 64.
sockId := sockResult socketId.

"Step 3: TLS handshake (via TLS capability)"
| tls ctxResult ctxId hsResult |
tls := Canal capability: #tls.
tls send: (object { op := 'create_context'. role := 'client'. verifyPeer := true }).
ctxResult := tls receive: 64.
ctxId := ctxResult contextId.

tls send: (object { op := 'handshake'. contextId := ctxId. socketId := sockId.
                    host := 'example.com'. port := 443 }).
hsResult := tls receive: 64.
(hsResult ok) ifFalse: [ Console println: 'TLS failed' ].

"Step 4: Send HTTP GET request"
tls send: (object { op := 'write'. contextId := ctxId.
                    data := 'GET / HTTP/1.1', String nl, 'Host: example.com', String nl, String nl }).
tls receive: 32.   "discard write ack"

"Step 5: Receive HTTP response"
| resp |
tls send: (object { op := 'read'. contextId := ctxId. maxLen := 1024 }).
resp := tls receive: 1024.
Console println: resp data printString.
```

The domain boundary crossings in this example:

1. picoceci → Wi-Fi capability (`#wifi`): connect, socket create
2. picoceci → TLS capability (`#tls`): handshake, encrypt, decrypt
3. TLS domain → Wi-Fi domain (`#wifi`): socket send/receive (internal, not visible
   to the picoceci script)

The picoceci domain never touches raw TCP bytes or TLS record parsing—both are handled by
their respective domains.

### 5.3 React to a Sensor Reading and Publish an MQTT Message

*(illustrative — requires `#uart` and `#wifi` capabilities)*

```picoceci
| uart wifi |
uart := UART new: 0 baud: 9600.
wifi  := Canal capability: #wifi.

"Check Wi-Fi status"
wifi send: (object { op := 'status' }).
| status |
status := wifi receive: 64.
(status connected) ifFalse: [ Console println: 'not connected' ].

| publishMqtt |
publishMqtt := [ :topic :payload |
    "MQTT publish over TCP (simplified — assumes broker at 192.168.1.100:1883)"
    wifi send: (object { op := 'create_socket'. protocol := 'tcp' }).
    wifi receive: 64.
    "… MQTT CONNECT packet, PUBLISH packet (omitted for brevity) …"
    Console println: 'Published: ', payload printString, ' to ', topic printString
  ].

"Poll sensor every 10 seconds"
[ true ] whileTrue: [
    | reading |
    reading := uart readLine.
    reading isNil ifFalse: [
        publishMqtt value: 'sensors/temperature' value: reading
    ].
    Task delay: 10000
].
```

---

## 6. Limitations and Safety Model

### 6.1 What picoceci Cannot Do

**No raw memory access.** There are no pointer types in picoceci. You cannot call
`unsafe.Pointer`, access arbitrary addresses, or forge a `CapabilityID` integer. This is
enforced both by the language (the type system has no pointer type) and by the hardware
(the MMU will fault on any access outside the domain's pages).

**No capability escalation.** A picoceci script can only hold capabilities the kernel
grants. It cannot `CapGrant` itself a capability it does not already own, and it cannot
bypass the `CapValidate` check in the kernel.

**No direct hardware access.** There are no register-read/write builtins in picoceci.
Every hardware operation goes through a capability channel to the appropriate service
domain.

**No kernel introspection.** `!caps` reads the domain's *own* capability list, not the
full kernel capability table. A script cannot enumerate other domains' capabilities.

### 6.2 How a Crashed Script Is Sandboxed

If a picoceci script enters an infinite loop, allocates until the heap is full, or
triggers a panic:

1. The TinyGo runtime inside the picoceci FreeRTOS task catches the panic (or the heap
   allocator returns nil and `runtimeAlloc` panics).
2. `runREPL` catches the error at the `vm.Run` call site and prints it.
3. The REPL returns to the prompt.

If the panic is unrecoverable (stack overflow, hardware memory fault):

1. The ESP32-S3 MMU raises a fault interrupt.
2. Canal's fault handler calls `DomainKill` on the offending domain.
3. All capabilities held by the picoceci domain are revoked.
4. The kernel optionally re-spawns the domain from its flash partition.
5. **All other domains continue running.** The Wi-Fi connection is not dropped. The
   logger keeps logging. The hardware state is intact.

---

## Summary

picoceci sits at the top of Canal's privilege stack. It is an ordinary user domain—subject
to the same capability enforcement as every other code on the system—but its interactive
REPL and hot-patching capability make it uniquely productive for rapid hardware
experimentation. Scripts talk to hardware through named capability channels, and every
channel operation is a kernel-validated syscall that cannot exceed the domain's declared
rights.

In [Article 5](./05-build-a-programming-learning-environment.md) we shift from architecture
to practice: a complete guide to setting up a Canal + picoceci learning environment for
a classroom, workshop, or solo project.

---

## Exercises

1. **Capability manifest.** A picoceci script wants to read a GPIO pin, write the result
   to a file on the SD card, and publish the value over MQTT. List the capabilities the
   domain manifest must declare, and explain what would happen at runtime if any one of
   those capabilities were missing.

2. **Capability IPC mapping.** Write a short picoceci pseudo-program that acquires the
   `#wifi` Canal capability, sends a connect request, waits for the acknowledgement, and
   closes the capability on error. Add inline comments explaining how each capability
   operation maps to the underlying Canal IPC mechanism described in Article 3.

3. **Hot-patch hazards.** The article describes "hot-patching a running program without
   rebooting." Identify two situations where hot-patching would be unsafe and explain what
   guards Canal puts in place to prevent those situations (or what the programmer must do
   manually).

4. **Capability vs. UNIX permissions.** Compare the capability model for picoceci scripts
   to the UNIX permission model (read/write/execute bits). Give one example of a security
   guarantee the Canal capability model provides that UNIX file permissions cannot express.

5. **HTTPS trace.** Trace through the "Fetch a URL over HTTPS" worked example step by
   step, identifying every domain boundary crossed and every capability exercised. What is
   the minimum set of capabilities a picoceci domain needs to complete this task?
