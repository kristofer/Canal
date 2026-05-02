# Educational Materials Plan

This document outlines a series of articles for learners who want to understand the Canal
system and its companion language, [picoceci](https://github.com/kristofer/picoceci).
The series is written to be read in order, but each article is also self-contained enough
to serve as a standalone reference.

---

## Article Series Overview

| # | Title | Focus |
|---|-------|-------|
| 1 | [The ESP32-S3 System: Hardware Meets Software](articles/01-esp32s3-hardware-meets-software.md) | Hardware platform overview |
| 2 | [picoceci: A Language Built for Tiny Machines](articles/02-picoceci-a-language-for-tiny-machines.md) | Language design and usage |
| 3 | [Canal and FreeRTOS: Running Go on Bare Metal](articles/03-canal-and-freertos-go-on-bare-metal.md) | OS/runtime architecture |
| 4 | [picoceci on Canal: Programming the Microkernel](articles/04-picoceci-on-canal-programming-the-microkernel.md) | Integration and capabilities |
| 5 | [Build a Programming Learning Environment on Canal](articles/05-build-a-programming-learning-environment.md) | End-to-end project tutorial |

---

## Article 1

### The ESP32-S3 System: Hardware Meets Software

**Summary**

An introduction to the ESP32-S3 microcontroller and how Canal turns its raw hardware into a
structured, safe computing platform. Readers will come away with a clear mental model of
what the chip provides and which parts Canal relies on.

**Target audience**: Developers who are new to embedded systems or the ESP32 family.

**Prerequisites**: Basic programming familiarity; no embedded-systems experience required.

**Suggested length**: 2,000–3,000 words + diagrams.

**Key topics**

1. ESP32-S3 at a glance
   - Dual-core Xtensa LX7 at 240 MHz
   - 512 KB SRAM, 8 MB PSRAM, up to 16 MB flash
   - WiFi 4, Bluetooth 5 / BLE
   - Rich peripheral set: SPI, I2C, UART, USB OTG, ADC, touch sensors

2. Memory architecture
   - Internal SRAM layout (DRAM vs IRAM)
   - PSRAM over QSPI and cache interaction
   - Flash memory map and partition tables

3. Hardware security features
   - Memory Management Unit (MMU) and page-based protection
   - PID controller for domain tagging
   - Secure boot and flash encryption (optional)

4. Why the ESP32-S3 for Canal
   - Hardware isolation support (MMU/PID) that makes capability domains feasible
   - Affordable, widely available, and well-documented
   - Community tooling (ESP-IDF, TinyGo, esptool)

5. Relationship between hardware, firmware, and software layers
   - Bootloader → Canal kernel → system domains → user domains
   - Diagram of the full software stack

**Exercises**

1. The ESP32-S3 has two Xtensa LX7 cores. Name two advantages this gives Canal over a
   single-core microcontroller when running multiple isolated domains simultaneously.

2. Draw a simple ASCII diagram that shows the four memory regions available on the
   ESP32-S3 (internal SRAM, IRAM, PSRAM, and flash) and note the typical size of each.
   Explain in one sentence why Canal maps user-domain heaps into PSRAM rather than
   internal SRAM.

3. Explain the role of the PID controller in Canal's security model. How does hardware
   tagging of memory accesses prevent one domain from reading another domain's data, even
   if a bug in that domain constructs an arbitrary pointer?

4. Trace the boot sequence from power-on to a running user domain by listing each layer
   in order (bootloader, Canal kernel, system domains, user domain) and writing one
   sentence describing what each layer sets up before passing control to the next.

5. Suppose a student has an Arduino background and asks "why can't I just use
   `malloc` everywhere?" Write a two-paragraph explanation—suitable for that student—of
   why Canal enforces per-domain memory budgets and what could go wrong on a
   microcontroller without that discipline.

---

## Article 2

### picoceci: A Language Built for Tiny Machines

**Summary**

A tour of picoceci—its design philosophy, the languages that influenced it, the features
that make it distinctive on a microcontroller, and a quick-start guide for writing real
programs.

**Target audience**: Developers curious about embedded scripting and language design;
educators exploring teaching languages.

**Prerequisites**: Article 1 or general familiarity with microcontrollers; some programming
experience in any language.

**Suggested length**: 2,500–3,500 words + code examples.

**Key topics**

1. Why a new language?
   - Tight resource budget (RAM, flash, no filesystem assumptions)
   - Safety without a full type system or garbage-collected runtime
   - Goal: a language that is easy to teach and fun to explore on hardware

2. Influences
   - Forth: stack discipline, small interpreter, extensibility
   - Scheme / Lisp: homoiconicity, first-class functions, minimal syntax
   - Go: channel-based concurrency model
   - Logo / Smalltalk: learner-friendly, interactive REPL

3. Unique features
   - Compact bytecode representation fits comfortably in flash
   - First-class channels map directly to Canal's IPC primitives
   - Hot-loadable definitions without a full OS reboot
   - Interactive REPL over serial/USB for live exploration
   - Deterministic memory: static arena or region-based allocation

4. Language walkthrough
   - Values and types (numbers, booleans, symbols, lists, channels)
   - Defining words / functions
   - Control flow
   - Talking to hardware (GPIO, timers) through Canal capabilities
   - Sending and receiving on channels

5. Running picoceci on the MCU
   - Flashing the interpreter binary
   - Connecting to the REPL (USB serial)
   - Writing and loading a first program

**Exercises**

1. picoceci draws design ideas from Forth, Scheme, Go, and Logo. Choose any two of those
   languages and write a short paragraph for each explaining which specific picoceci
   feature reflects that influence and why the designers likely borrowed it.

2. Open (or imagine) a picoceci REPL session. Write a short program that:
   - Defines a function called `square` that returns the square of its argument.
   - Uses `square` inside a loop to print the squares of 1 through 5.
   Label each line with a comment explaining what it does.

3. picoceci uses "deterministic memory" (static arena or region-based allocation).
   Compare this to garbage collection: list one benefit and one drawback of each
   approach in the context of a microcontroller with 512 KB of RAM.

4. Explain what "hot-loadable definitions" means in the context of picoceci. Describe a
   practical development scenario where being able to redefine a function without
   rebooting the board would save significant time.

---

## Article 3

### Canal and FreeRTOS: Running Go on Bare Metal

**Summary**

An in-depth look at how Canal is architected, how it leverages FreeRTOS as its task
scheduler, and how TinyGo's runtime is adapted to fit inside isolated domains.

**Target audience**: Systems programmers and embedded developers who want to understand
Canal's internals.

**Prerequisites**: Articles 1–2 or familiarity with RTOS concepts and Go/TinyGo.

**Suggested length**: 3,000–4,000 words + architecture diagrams.

**Key topics**

1. Canal's design goals
   - Capability-based security on constrained hardware
   - Crash isolation: a buggy domain cannot corrupt the kernel or other domains
   - Go-native APIs so application code feels like ordinary Go

2. FreeRTOS as the scheduling substrate
   - Tasks, queues, and semaphores used by Canal
   - How FreeRTOS tasks map to Canal domains
   - Tick interrupt, priority levels, and real-time considerations
   - What Canal adds on top (no FreeRTOS API surface exposed to user domains)

3. TinyGo runtime inside a domain
   - Goroutine scheduler (cooperative / async-preemptive)
   - Garbage collector: conservative, triggered per-domain
   - Channel implementation mapped to FreeRTOS queues
   - Memory layout: stack, heap, globals per domain

4. The kernel substrate
   - Capability table: structure, lookup, revocation
   - Domain lifecycle: create, load, start, crash, restart
   - Syscall interface: how a domain makes a request to the kernel
   - MMU / MPU configuration per domain

5. Inter-domain communication
   - Capability-mediated channels
   - Message passing protocol and zero-copy optimisations
   - Example: WiFi domain → TLS domain → HTTP domain message flow

6. Build system and toolchain
   - How TinyGo, clang, and ESP-IDF combine
   - Makefile targets and what they do
   - Flashing and debugging workflow

**Exercises**

1. In Canal, FreeRTOS tasks are the underlying mechanism, but user domains never call
   FreeRTOS APIs directly. Explain why hiding the FreeRTOS surface is a deliberate
   design choice. What security property would be lost if user domains could call
   `xTaskCreate` themselves?

2. TinyGo's garbage collector runs per-domain rather than globally. Sketch (in prose or
   pseudocode) how a GC pause in Domain A could affect Domain B, and then explain why
   Canal's per-domain GC prevents this problem.

3. Describe what is stored in Canal's capability table for a domain that has been granted
   `service:wifi`. What happens—step by step—when that domain's capability is revoked
   while a WiFi operation is in flight?

4. The article mentions a zero-copy optimization for inter-domain messages. Explain what
   "zero-copy" means in this context, under what conditions Canal can safely use it, and
   why a naïve implementation without capability checking would be a security hole.

5. Draw a sequence diagram (ASCII or prose) showing the full message flow described in
   the article: WiFi domain → TLS domain → HTTP domain. For each arrow, note which
   capability is exercised and what data crosses the domain boundary.

---

## Article 4

### picoceci on Canal: Programming the Microkernel

**Summary**

Explains how the picoceci interpreter runs as a Canal domain, how scripts interact with
system services through capabilities and channels, and what unique things picoceci can
do because of this integration.

**Target audience**: Developers who have read Articles 2 and 3 and want to use picoceci
for real tasks on Canal.

**Prerequisites**: Articles 1–3.

**Suggested length**: 2,500–3,500 words + code examples and sequence diagrams.

**Key topics**

1. The picoceci domain
   - Domain manifest: capabilities requested at startup
   - Memory budget for the interpreter, stack, and loaded programs
   - Lifecycle: boot, REPL listen loop, program execution, error recovery

2. Capabilities picoceci can hold
   - `service:wifi` — initiate and tear down network connections
   - `service:tls` — open encrypted channels without touching key material
   - `device:gpio` — read and write pins
   - `device:uart` — communicate with peripherals
   - `fs:read` / `fs:write` — access SD card files
   - Custom capabilities defined by user domains

3. Channel-based I/O in picoceci programs
   - Opening a channel to a service domain
   - Sending typed messages
   - Receiving responses and handling errors
   - Concurrency: running multiple goroutine-like tasks in picoceci

4. Live coding on a running system
   - Defining new words in the REPL and immediately calling them
   - Hot-patching a running program without rebooting
   - Debugging with the capability inspector

5. Worked examples
   - Blink an LED and log the state to a file
   - Fetch a URL over HTTPS and display the response on a serial console
   - React to a sensor reading and publish an MQTT message

6. Limitations and safety model
   - What picoceci cannot do (no raw memory access, no capability escalation)
   - How a crashed script is sandboxed from the rest of the system

**Exercises**

1. A picoceci script wants to read a GPIO pin, write the result to a file on the SD card,
   and publish the value over MQTT. List the three capabilities the domain manifest must
   declare, and explain what would happen at runtime if any one of those capabilities
   were missing.

2. Write a short picoceci pseudo-program that opens a channel to the WiFi service domain,
   sends a connect request, waits for the acknowledgement, and closes the channel on
   error. Add inline comments explaining how each channel operation maps to the
   underlying Canal IPC mechanism described in the article.

3. The article describes "hot-patching a running program without rebooting." Identify two
   situations where hot-patching would be unsafe and explain what guards Canal puts in
   place to prevent those situations (or what the programmer must do manually).

4. Compare the capability model for picoceci scripts to the UNIX permission model
   (read/write/execute bits). Give one example of a security guarantee the Canal
   capability model provides that UNIX file permissions cannot express.

5. Trace through the "Fetch a URL over HTTPS" worked example step by step, identifying
   every domain boundary crossed and every capability exercised. What is the minimum set
   of capabilities a picoceci domain needs to complete this task?

---

## Article 5

### Build a Programming Learning Environment on Canal

**Summary**

A hands-on, end-to-end tutorial for setting up a complete programming learning
environment using Canal and picoceci. Aimed at educators and makers who want to give
students or workshop participants a safe, interactive platform to learn programming
concepts on real hardware.

**Target audience**: Educators, workshop organisers, and makers; technical but not
necessarily embedded-systems experts.

**Prerequisites**: Articles 1–4 or willingness to follow instructions without full
background knowledge.

**Suggested length**: 4,000–5,000 words + step-by-step instructions, screenshots.

**Key topics**

1. Vision and use cases
   - Classroom with 20 ESP32-S3 boards
   - Solo learner at home
   - Hackathon starter kit

2. Hardware shopping list
   - ESP32-S3 development board (recommended models)
   - USB cable (data-capable)
   - Optional: micro-SD card, LEDs, buttons, sensors

3. Setting up the host machine
   - Install TinyGo, ESP-IDF, esptool, pyserial
   - Clone Canal and picoceci repositories
   - One-command setup script walkthrough

4. Flashing the learning image
   - Building a Canal image that includes the picoceci domain plus example programs
   - Flashing and verifying the board powers on correctly
   - Connecting to the REPL for the first time

5. Curriculum sketches
   - Lesson 0: Hello, hardware! (print a message, blink an LED)
   - Lesson 1: Variables, loops, conditionals in picoceci
   - Lesson 2: Functions and recursion
   - Lesson 3: Channels and concurrency — two tasks talking to each other
   - Lesson 4: Reading a sensor and reacting to data
   - Lesson 5: WiFi and the internet — fetching data from a web API
   - Lesson 6: Saving and loading programs on the SD card

6. Safety model for a learning environment
   - Why a crashed student program cannot hurt other domains
   - How to reset a domain without rebooting the whole board
   - Restricting capabilities so students can only use approved services

7. Extending the environment
   - Adding a custom domain that students can interact with
   - Writing a simple web dashboard that shows live data from the board
   - Packaging a finished project for others to flash

8. Troubleshooting common issues
   - Board not detected over USB
   - Flash partition errors
   - REPL not responding

**Exercises**

1. You are setting up a classroom with 20 ESP32-S3 boards. Write a brief checklist
   (5–8 items) that ensures each board is correctly flashed and ready for students
   before the first lesson. Include at least one verification step for each of:
   hardware connection, firmware version, and REPL availability.

2. A student's picoceci script crashes with a "capability denied" error when trying to
   blink an LED. Walk through the debugging steps you would take, in order, to identify
   whether the problem is (a) a missing capability in the domain manifest, (b) a
   typo in the GPIO pin number, or (c) a hardware wiring error.

3. The safety model section explains that a crashed student domain cannot affect other
   domains. Design a short experiment a student could run to verify this claim: describe
   the setup, the deliberate crash to trigger, and the observation that proves isolation
   held.

4. After completing Lesson 3 (channels and concurrency), a student asks: "This looks
   like Go goroutines—are they the same?" Write a two-paragraph answer that explains the
   similarities and the key differences between picoceci channel tasks and Go goroutines
   in the standard runtime.

5. Describe how you would extend the learning environment with a custom domain that
   exposes a simple key-value store capability. What capability name would you register,
   what messages would the domain accept, and how would a picoceci script use it?

---

## Production Notes

- **Format**: Each article should be published as a Markdown file in the `docs/articles/`
  directory and linked from this plan. A rendered version (HTML or static site) is
  encouraged for wider reach.
- **Code listings**: All code samples must be tested on physical hardware or an accurate
  emulator before publication. Listings that are illustrative only must be clearly labelled.
- **Diagrams**: Prefer text-based diagrams (ASCII / Mermaid) embedded in the Markdown
  source so they render in GitHub and can be version-controlled. Export SVG/PNG for any
  diagram that is too complex to represent in ASCII.
- **Versioning**: Tag each article with the Canal and picoceci versions it was written for.
  Mark articles as outdated when a breaking change occurs.
- **Contributor guide**: New authors should open a draft pull request early to receive
  feedback before writing the full article. Use the outline above as the starting template.
