# Article 5: Build a Programming Learning Environment on Canal

> **Canal version**: 0.1.0-dev | **picoceci version**: 0.1.0-dev
> **Target audience**: Educators, workshop organisers, and makers; technical but not
> necessarily embedded-systems experts.
> **Prerequisites**: [Articles 1–4](../EDUCATIONAL_PLAN.md) or willingness to follow
> instructions without full background knowledge.

---

## Introduction

Microcontrollers are remarkable teaching tools. They provide immediate, physical feedback
from code—an LED blinks, a motor turns, a number changes on a screen—that abstract
programming exercises on a laptop simply cannot match. The challenge for educators has
always been safety: giving students enough access to do interesting things while protecting
the hardware from being bricked by a runaway program, and protecting the classroom network
from student experiments gone wrong.

Canal solves this at the operating system level. Because each picoceci script runs in an
isolated domain with only the capabilities it was explicitly given, a student program that
crashes, allocates unbounded memory, or tries to write to pins it should not touch is
stopped by the hardware before any damage occurs. The rest of the system keeps running.

This article is a complete, hands-on guide to building that environment:

- Hardware and software shopping lists.
- Step-by-step setup for the host machine and board.
- A seven-lesson curriculum outline.
- The safety model that makes it viable in a classroom.
- How to extend, troubleshoot, and package the environment.

---

## 1. Vision and Use Cases

### Classroom with 20 Boards

Each student gets an identical ESP32-S3 development board pre-flashed with the Canal
learning image. Students connect over USB, open a browser (if a web dashboard is
deployed) or a serial terminal, and write picoceci code that controls real hardware.
Because domains are isolated, one student's infinite loop does not affect the neighbours'
boards—not even on the same board if multiple sessions share it.

### Solo Learner at Home

A single board, a laptop, and a USB cable. Canal provides the same structured safety
net that the classroom uses, so a beginner can experiment aggressively without worrying
about bricking the hardware. The REPL gives instant feedback; the capability model acts
as a built-in guide to which resources are available.

### Hackathon Starter Kit

Teams receive a Canal image pre-configured with the capabilities their project domain
needs (Wi-Fi, GPIO, sensors). The picoceci REPL lets them prototype ideas within minutes
of receiving the board, and the isolation model means different parts of the project can
be developed and tested independently without interfering with each other.

---

## 2. Hardware Shopping List

| Item | Notes |
|------|-------|
| ESP32-S3 development board | Espressif ESP32-S3-DevKitC-1 recommended; any ESP32-S3 board with a native USB port works |
| USB-C or USB-Micro cable | Must be a *data* cable, not a charge-only cable |
| micro-SD card (optional) | Class 10, 4–32 GB; needed for `fs:read` / `fs:write` capabilities and program saving |
| LEDs + 330 Ω resistors | For GPIO exercises |
| Pushbuttons | For GPIO input exercises |
| Breadboard + jumper wires | Standard 400-tie breadboard |
| Temperature/humidity sensor | DHT22 or SHT30 for Lesson 4 (sensor reading) |

**Board selection note**: The ESP32-S3-DevKitC-1 has a built-in addressable RGB LED on
GPIO 48 and a regular LED on GPIO 2, which means Lessons 0 and 1 require zero external
components.

---

## 3. Setting Up the Host Machine

### 3.1 Install TinyGo

```bash
# macOS (Homebrew)
brew tap tinygo-org/tools
brew install tinygo

# Linux (Debian/Ubuntu)
wget https://github.com/tinygo-org/tinygo/releases/download/v0.31.0/tinygo_0.31.0_amd64.deb
sudo dpkg -i tinygo_0.31.0_amd64.deb

# Verify
tinygo version
# expected: tinygo version 0.31.0 …
```

### 3.2 Install ESP-IDF (Required for ESP32-S3 Flash Domain Loading)

```bash
git clone --recursive https://github.com/espressif/esp-idf.git ~/esp/esp-idf
cd ~/esp/esp-idf
./install.sh esp32s3
source ./export.sh      # add to your .bashrc / .zshrc
```

### 3.3 Install Python Tools

```bash
pip3 install esptool pyserial
```

### 3.4 Clone Canal and picoceci

```bash
git clone https://github.com/kristofer/Canal.git
cd Canal/canal
```

### 3.5 One-Command Setup Script

```bash
chmod +x scripts/setup.sh
./scripts/setup.sh esp32s3
```

The setup script downloads and places the FatFS and mbedTLS third-party libraries that
the SD-card and TLS domains depend on. It prints a summary of what it installed and
exits non-zero if anything fails.

---

## 4. Flashing the Learning Image

### 4.1 Build the Canal Learning Image

The learning image includes the Canal kernel, the picoceci domain, the Wi-Fi domain,
the GPIO service domain, the SDCard domain, and a set of example picoceci programs on the
SD card.

```bash
# Build kernel
make build

# Build all domain binaries
make build-domains
make picoceci          # builds picoceci domain with -gc=leaking

# Or build everything in one step
make all
```

### 4.2 Flash Kernel and All Domains

```bash
# Plug in the board, then:
make flash-all PORT=/dev/ttyUSB0          # Linux
make flash-all PORT=/dev/cu.usbmodem1301  # macOS
```

`flash-all` runs `tinygo flash` for the kernel (which erases the entire chip) and then
uses `esptool.py` to write each domain binary to its dedicated flash partition:

```
0x010000  Canal kernel
0x100000  LED blinker domain
0x180000  Wi-Fi domain
0x200000  Logger domain
0x280000  TLS domain
0x300000  SDCard domain
```

The picoceci domain is flashed to a separate partition (address defined in
`build/targets/esp32s3/partitions.csv`).

### 4.3 Verify the Board Powers On

```bash
make monitor PORT=/dev/ttyUSB0
```

Expected output:

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
[picoceci] Starting v0.1.0-dev (Canal domain)
[picoceci] Ready.

>
```

If the board does not respond, see Section 8 (Troubleshooting).

### 4.4 Connecting to the REPL for the First Time

Any serial terminal at 115200 baud works. Recommended options:

```bash
# pyserial miniterm (bundled with esptool)
python3 -m serial.tools.miniterm --dtr 0 --rts 0 /dev/ttyUSB0 115200

# macOS screen
screen /dev/cu.usbmodem1301 115200

# PuTTY (Windows) — Serial, COM port, 115200 baud, 8N1, no flow control
```

Type a simple expression to confirm the REPL is working:

```picoceci
> 1 + 1
=> 2
```

---

## 5. Curriculum Sketches

The following lessons progress from zero hardware knowledge to building a networked sensor
application. Each lesson fits in 30–45 minutes for a motivated beginner.

### Lesson 0: Hello, Hardware!

**Goal**: Print a message and blink the built-in LED.

```picoceci
> println("Hello from Canal!")
Hello from Canal!

> let led = gpio_open(2)      # built-in LED on GPIO 2
> gpio_write(led, true)       # LED on
> sleep_ms(1000)
> gpio_write(led, false)      # LED off
```

**Takeaway**: The REPL gives immediate feedback; hardware responds in real time.

---

### Lesson 1: Variables, Loops, and Conditionals

**Goal**: Understand picoceci's basic data types and control flow.

```picoceci
> let x = 10
> let y = 3
> x + y
=> 13
> x > y
=> true
> if x > y { println("x wins") } else { println("y wins") }
x wins

> let i = 0
> while i < 5 {
    println(i)
    let i = i + 1
  }
0
1
2
3
4
```

**Takeaway**: Variables are immutable bindings; `let` inside a block creates a new binding
that shadows the outer one.

---

### Lesson 2: Functions and Recursion

**Goal**: Define reusable functions and understand recursion.

```picoceci
> let square = fn(x) { x * x }
> square(5)
=> 25

> let sum_to = fn(n) {
    if n <= 0 { 0 } else { n + sum_to(n - 1) }
  }
> sum_to(5)
=> 15
```

**Hardware exercise**: Write a `blink_n` function that blinks the LED *n* times.

```picoceci
> let blink_n = fn(led, n) {
    if n > 0 {
      gpio_write(led, true)
      sleep_ms(200)
      gpio_write(led, false)
      sleep_ms(200)
      blink_n(led, n - 1)
    }
  }
> blink_n(gpio_open(2), 5)
```

**Takeaway**: Functions are values; recursion is the natural way to express repetition
with a stopping condition.

---

### Lesson 3: Channels and Concurrency — Two Tasks Talking

**Goal**: Understand picoceci channels and the `spawn` primitive.

```picoceci
# Create a local channel (not a Canal IPC channel — just in-domain)
> let ch = make_channel()

# Producer: send numbers 1..5 then close
> spawn fn() {
    let i = 1
    while i <= 5 {
      send(ch, i)
      let i = i + 1
    }
    close(ch)
  }

# Consumer: receive until channel is closed
> let v = recv(ch)
> while v != nil {
    println(v)
    let v = recv(ch)
  }
1
2
3
4
5
```

**Takeaway**: Channels decouple producers from consumers; `spawn` creates a concurrent
task; `recv` blocks until data is available.

---

### Lesson 4: Reading a Sensor and Reacting to Data

**Goal**: Read real hardware data and branch on it.

```picoceci
# Assumes a temperature sensor on UART (e.g. SHT30)
> let uart = open_channel("device:uart")
> send(uart, {op: "configure", baud: 9600, bits: 8, parity: "none"})
> recv(uart)    # wait for ack

> let read_temp = fn() {
    send(uart, {op: "read", max_len: 8})
    let r = recv(uart)
    r.value     # numeric temperature reading
  }

> let temp = read_temp()
> println("Temperature: " + temp + "°C")
Temperature: 23.4°C

> if temp > 30 { println("Too hot!") } else { println("Comfortable") }
Comfortable
```

**Takeaway**: `device:uart` capability gates UART access; the domain manifest controls
which pins and baud rates are permitted.

---

### Lesson 5: Wi-Fi and the Internet — Fetching Data from a Web API

**Goal**: Connect to Wi-Fi and make an HTTP(S) request.

```picoceci
# Connect to Wi-Fi
> let wifi = open_channel("service:wifi")
> send(wifi, {op: "connect", ssid: "SchoolNet", password: "learn2code", timeout: 15000})
> let r = recv(wifi)
> println(r.ok)
true

# Fetch current time from a public API (illustrative)
> let tls = open_channel("service:tls")
> # … TLS handshake, HTTP GET, parse JSON response (see Article 4 for full code) …
> println("UTC time: " + response.utc)
UTC time: 2026-05-01T12:00:00Z
```

**Takeaway**: Network access requires two capabilities (`service:wifi` and `service:tls`);
the picoceci domain never touches TCP bytes directly.

---

### Lesson 6: Saving and Loading Programs on the SD Card

**Goal**: Persist picoceci code across reboots.

```picoceci
# Write a program to the SD card
> let fs = open_channel("fs:write")
> send(fs, {
    path: "/programs/blink.pico",
    data: "let blink_n = fn(led, n) { if n > 0 { gpio_write(led, true) " +
          "sleep_ms(200) gpio_write(led, false) sleep_ms(200) blink_n(led, n-1) } }"
  })
> recv(fs)

# Load it back in a fresh session
> load("/programs/blink.pico")
[loaded blink.pico]
> blink_n(gpio_open(2), 3)
```

**Takeaway**: `fs:write` is separate from `fs:read`; a domain with only `fs:read` cannot
modify stored programs.

---

## 6. Safety Model for a Learning Environment

### 6.1 Why a Crashed Student Program Cannot Hurt Other Domains

When a picoceci script causes an unrecoverable error:

1. The TinyGo runtime inside the picoceci FreeRTOS task catches the panic.
2. If the panic is unrecoverable (memory fault), the ESP32-S3 MMU raises a hardware fault.
3. Canal's fault handler calls `DomainKill` on the picoceci domain.
4. All capabilities held by the picoceci domain are revoked.
5. The Canal kernel re-spawns the picoceci domain from its flash partition.
6. **The Wi-Fi domain, TLS domain, SDCard domain, and all other students' sessions are
   completely unaffected.**

The isolation guarantee comes from hardware (MMU + PID controller), not from software
checks that a clever program might bypass. A student writing `while true {}` will starve
the picoceci goroutine scheduler but will not starve other FreeRTOS tasks running at
higher priority.

### 6.2 How to Reset a Domain Without Rebooting the Board

From the REPL (if it is still responsive):

```picoceci
> !reset_domain picoceci
[kernel] Domain 5 killed and restarted
[picoceci] Starting v0.1.0-dev (Canal domain)
[picoceci] Ready.
>
```

From the host machine, using the Canal debug CLI:

```bash
canal-cli --port /dev/ttyUSB0 kill-domain picoceci
canal-cli --port /dev/ttyUSB0 spawn-domain picoceci
```

A full board reset is rarely necessary. Canal is designed so that individual domains can
be restarted in milliseconds while the rest of the system remains online.

### 6.3 Restricting Capabilities for a Classroom

For a controlled lesson, the learning image can be built with a **restricted capability
policy** that prevents picoceci from requesting capabilities the lesson does not need.

Edit `kernel/syscall.go`, `findCapabilityByName`:

```go
case "service:wifi":
    // Allow only during Lesson 5 and beyond — comment out to restrict
    queue := xQueueCreate(4, uint32(unsafe.Sizeof(wifi.WiFiMessage{})))
    capID := CapAlloc(requestor, CapTypeChannel, unsafe.Pointer(queue), rights)
    capRegistry[name] = capID
    return capID
```

Commenting out the `service:wifi` case means any `open_channel("service:wifi")` call
returns an error immediately, without changing anything else. Students in Lessons 0–4 are
not affected; the instructor enables it for Lesson 5 by re-flashing the kernel.

---

## 7. Extending the Environment

### 7.1 Adding a Custom Domain

A custom domain is a Canal domain that registers one or more named capabilities and
responds to messages from picoceci scripts. Example: a key-value store domain.

```go
// domains/keyvalue/main.go
package main

import (
    "runtime"
    "time"
)

var store = map[string]string{}

func main() {
    // Register capability
    cap, _ := runtime.RequestCap("custom:keyvalue", kernel.RightRead|kernel.RightWrite)

    for {
        var msg KVMessage
        runtime.CapRecv(cap, &msg)

        switch msg.Op {
        case "set":
            store[msg.Key] = msg.Value
            runtime.CapSend(cap, KVResponse{Ok: true})
        case "get":
            v, ok := store[msg.Key]
            runtime.CapSend(cap, KVResponse{Ok: ok, Value: v})
        }
    }
}
```

Flash the new domain to an unused partition, and picoceci scripts can immediately use it:

```picoceci
> let kv = open_channel("custom:keyvalue")
> send(kv, {op: "set", key: "score", value: "42"})
> recv(kv)
> send(kv, {op: "get", key: "score"})
> recv(kv)
=> {ok: true, value: "42"}
```

### 7.2 Writing a Simple Web Dashboard

The HTTP server domain (`domains/http-server/`) can serve a live status page showing
each domain's state, capability count, and heap usage. Because the HTTP domain has only
`service:wifi` access and reads domain state through kernel debug channels, it cannot
interfere with any other domain.

A minimal Makefile step to add the dashboard to the learning image:

```bash
make build-dashboard   # compiles domains/http-server with dashboard template
make flash-domains     # re-flashes domain partitions without touching the kernel
```

Students can then open `http://<board-ip>/` in a browser to see a live view of the
running system.

### 7.3 Packaging a Finished Project

Once a project is complete, package it as a flashable image:

```bash
# Create a release image: kernel + all domains merged into one binary
make release OUT=my_project.bin

# Students flash it with a single command
esptool.py --chip esp32s3 --port /dev/ttyUSB0 write_flash 0x0 my_project.bin
```

The release image includes the partition table, bootloader, kernel, and all domain
binaries at their correct offsets. Recipients need only `esptool.py`—no TinyGo, no
ESP-IDF required.

---

## 8. Troubleshooting Common Issues

### 8.1 Board Not Detected over USB

**Symptom**: No port appears in `/dev/ttyUSB*` or `/dev/cu.usbmodem*`.

**Steps**:
1. Try a different USB cable. Charge-only cables (no data wires) are common and will not
   work. Test the cable with another USB device that transfers data.
2. Check the USB port on the board. ESP32-S3 boards have two USB ports: one for
   flashing/serial (usually labeled UART0 or USB), and one for the ESP32-S3's native USB
   OTG. Use the one labeled for serial/UART for flashing.
3. On Linux, check permissions: `ls -l /dev/ttyUSB0`. If the group is `dialout` and you
   are not in it, run `sudo usermod -aG dialout $USER` and log out/in.
4. On macOS, install the CH340/CP2102 USB-serial driver if the board uses one of those
   chips (some cheaper boards do).

### 8.2 Flash Partition Errors

**Symptom**: `esptool.py write_flash` fails with "Invalid head of packet" or the board
reboots into bootloader mode repeatedly.

**Steps**:
1. Hold the BOOT button on the board while pressing RESET, then release RESET, then
   release BOOT. This forces the board into download mode.
2. Erase the entire flash before re-flashing: `esptool.py --chip esp32s3 --port ... erase_flash`
3. Re-run `make flash-all`.
4. If the error persists, verify that `partitions.csv` matches the flash size of your
   specific board (4 MB vs 8 MB vs 16 MB). Check the board datasheet.

### 8.3 REPL Not Responding

**Symptom**: You can open the serial port but nothing appears, or the prompt `>` does not
show.

**Steps**:
1. Wait 3–5 seconds after connecting. The picoceci domain waits 2 seconds for USB CDC to
   stabilise before printing anything.
2. Press Enter once. Some terminals need one keystroke to synchronise line endings.
3. Check the baud rate is exactly 115200, 8 data bits, no parity, 1 stop bit, no
   hardware flow control.
4. Check the Canal boot log: run `make monitor` to see whether the picoceci domain
   started successfully. If you see `[picoceci] Ready.` but no prompt, try sending a
   Ctrl-C to reset the input buffer.
5. If the boot log shows `[picoceci] domain crash` or similar, the domain failed to start.
   Re-flash: `make picoceci-flash`.

---

## Summary

Canal and picoceci together provide a genuinely safe, interactive programming learning
environment on real hardware. The key properties:

- **Crash isolation**: A student program that goes wrong is stopped by the hardware (MMU +
  PID controller), not by fragile software checks. The rest of the system continues.
- **Capability least-privilege**: Students can only reach the hardware and services they
  need for the current lesson. Mistakes are bounded in scope.
- **Instant feedback**: The picoceci REPL gives results in milliseconds and hot-patching
  means the gap between "I had an idea" and "I see it working on hardware" is measured in
  seconds, not minutes.
- **Low operational overhead**: A classroom of 20 boards can be prepared and verified in
  under 30 minutes with the provided Makefile targets.

---

## Exercises

1. **Classroom preparation checklist.** You are setting up a classroom with 20 ESP32-S3
   boards. Write a brief checklist (5–8 items) that ensures each board is correctly
   flashed and ready for students before the first lesson. Include at least one
   verification step for each of: hardware connection, firmware version, and REPL
   availability.

2. **Debugging capability errors.** A student's picoceci script crashes with a "capability
   denied" error when trying to blink an LED. Walk through the debugging steps you would
   take, in order, to identify whether the problem is (a) a missing capability in the
   domain manifest, (b) a typo in the GPIO pin number, or (c) a hardware wiring error.

3. **Verifying crash isolation.** The safety model section explains that a crashed student
   domain cannot affect other domains. Design a short experiment a student could run to
   verify this claim: describe the setup, the deliberate crash to trigger, and the
   observation that proves isolation held.

4. **picoceci channels vs. Go goroutines.** After completing Lesson 3 (channels and
   concurrency), a student asks: "This looks like Go goroutines—are they the same?" Write
   a two-paragraph answer that explains the similarities and the key differences between
   picoceci channel tasks and Go goroutines in the standard runtime.

5. **Custom key-value domain.** Describe how you would extend the learning environment
   with a custom domain that exposes a simple key-value store capability. What capability
   name would you register, what messages would the domain accept, and how would a picoceci
   script use it?
