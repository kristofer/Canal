# Article 2: picoceci — A Language Built for Tiny Machines

> **Canal version**: 0.1.0-dev | **picoceci version**: 0.1.0-dev
> **Target audience**: Developers curious about embedded scripting and language design;
> educators exploring teaching languages.
> **Prerequisites**: [Article 1](./01-esp32s3-hardware-meets-software.md) or general
> familiarity with microcontrollers; some programming experience in any language.

---

## Introduction

When you write firmware for a microcontroller in C or C++, you are writing code that will
be compiled once, flashed, and run until the chip is powered off or reprogrammed. Want to
try a different LED blink rate? Edit, recompile, re-flash. This cycle is fine for
production firmware, but it is a poor fit for interactive exploration, education, or
rapid prototyping on live hardware.

**picoceci** solves this by providing a scripting language designed to run *inside* a
Canal domain on the ESP32-S3. It gives you an interactive REPL (Read-Eval-Print Loop)
over USB serial, a compact bytecode compiler/VM, and first-class support for Canal's
channel-based inter-domain communication—all within the tight memory budget of a
microcontroller.

This article covers:

- The motivation for a new language and the constraints it had to satisfy.
- The languages that influenced picoceci's design and why.
- The distinctive features that set it apart from general-purpose languages.
- A guided walkthrough of the language itself.
- How to get the interpreter running on hardware and try your first program.

---

## 1. Why a New Language?

Before designing picoceci, the Canal team considered existing options:

- **MicroPython** — mature, widely used, but its reference-counting GC and large runtime
  footprint push the memory budget uncomfortably. Python's dynamic typing means many
  errors surface only at runtime in ways that are hard to explain to beginners.
- **Lua** — lighter than Python, but the standard Lua VM still assumes a conventional OS
  heap and does not integrate cleanly with Canal's capability model.
- **JavaScript (e.g., Espruino / Duktape)** — interactive REPL is great, but heap
  fragmentation is a real concern over long run-times on small devices.
- **Forth** — runs on almost nothing, but its stack-based syntax is famously
  unapproachable for programmers who have not seen it before.

None of these hit the exact target: a language that is *resource-safe*, *teachable*,
*interactive*, and *natively expressive of Canal's concurrency model*.

### The Three Constraints

**1. Tight resource budget.**
The picoceci domain on the ESP32-S3 is allocated a `HeapMedium` (32 KB) heap by default.
The bytecode representation must be compact enough to fit in the remaining flash budget,
and the runtime must not rely on unbounded heap growth.

**2. Safety without a full type system.**
A crashed picoceci script must not corrupt the Canal kernel, other domains, or the
interpreter itself. Canal's MMU and capability model enforce this at the hardware level,
but the language design reinforces it: there is no raw pointer arithmetic, no way to call
arbitrary C functions, and no way to forge a capability.

**3. Easy to teach and fun on hardware.**
The target audience includes students and hobbyists encountering embedded programming for
the first time. Syntax should be forgiving, feedback should be immediate (hence the REPL),
and real hardware results (blinking LEDs, network requests) should be reachable in the
first lesson.

---

## 2. Influences

picoceci draws deliberately from four languages. Understanding these influences helps you
read picoceci code and reason about its design trade-offs.

### 2.1 Forth — Stack Discipline and a Tiny Interpreter

Forth is a language from the early 1970s that has never died, precisely because its interpreter
is tiny (kilobytes) and its execution model maps directly onto a stack machine. picoceci
borrows Forth's idea of *words* (named, reusable operations) and a compact bytecode
representation that fits comfortably in flash.

What picoceci does *not* borrow is Forth's post-fix syntax. Most programmers find
`3 4 +` harder to read than `3 + 4`, and since picoceci aims to be teachable, it uses a
more conventional notation.

### 2.2 Scheme / Lisp — First-Class Functions and Minimal Syntax

Scheme demonstrated that a powerful language can have a very small core. picoceci shares
Scheme's commitment to first-class functions and minimal special forms. A picoceci
function is a value that can be passed around, stored in a variable, or returned from
another function—without any extra ceremony.

The influence of *homoiconicity* (code-as-data) is visible in picoceci's list literals,
which are valid program fragments that can be manipulated at runtime.

### 2.3 Go — Channel-Based Concurrency

Go popularized the CSP (Communicating Sequential Processes) model in mainstream
programming: instead of shared memory and locks, goroutines communicate through typed
channels. picoceci adopts this model directly, mapping its channels onto the Canal IPC
primitives described in Article 1.

This is not accidental: Canal itself is written in Go (TinyGo), and its kernel IPC is
already built around FreeRTOS queues exposed as Go channels. picoceci scripts talk to
Canal system services through the same channel abstraction, so the mental model is
consistent all the way down.

### 2.4 Logo / Smalltalk — Learner-Friendly, Interactive REPL

Logo (best known for its turtle graphics) proved that beginners engage more deeply when
they get *immediate visual feedback* from their code. Smalltalk showed that a live,
image-based environment where you can inspect and redefine any object at runtime is
profoundly productive.

picoceci channels this spirit through its REPL: every definition is immediately callable,
results print automatically, and errors display with enough context to be informative
rather than cryptic.

---

## 3. Distinctive Features

### 3.1 Compact Bytecode

picoceci source is compiled to a compact bytecode format before execution. The compiler
is available on-device, so the REPL compiles each line as you type it. Bytecode chunks
fit comfortably in flash (typically a few hundred bytes for a simple program) and the
VM's instruction dispatch loop is small enough to sit in the ESP32-S3's IRAM, keeping
interpretation fast.

The picoceci domain in Canal wires the compiler and VM together in a single pipeline:

```
Source text
    │
    ▼
Lexer (lexer.NewString)
    │
    ▼
Parser (parser.New)
    │
    ▼
Compiler (bytecode.NewCompilerWithLoader)
    │  ← Module loader resolves imports
    ▼
Bytecode chunk
    │
    ▼
VM (bytecode.NewVM)
    │
    ▼
Result value
```

### 3.2 First-Class Channels

A channel in picoceci is a value, just like a number or a function. You can store it in a
variable, pass it to a function, and send or receive on it. Under the hood, each picoceci
channel corresponds to a Canal capability of type `CapTypeChannel`, which wraps a
FreeRTOS queue.

This means picoceci concurrency is not simulated inside the interpreter—it is real,
hardware-backed IPC that crosses domain boundaries when needed.

### 3.3 Hot-Loadable Definitions

Because the REPL compiles and evaluates each expression independently, you can redefine
a function without restarting the interpreter or rebooting the board. The new definition
shadows the old one immediately; any subsequent call picks up the new code. This
"hot-patching" workflow is invaluable during development: change a parameter, test on
live hardware, iterate.

### 3.4 Interactive REPL over Serial/USB

The picoceci domain opens a line-oriented console on `machine.Serial`, which maps to USB
CDC (USB serial) on the ESP32-S3. Connect with any serial terminal at 115200 baud and
you get an interactive prompt:

```
[picoceci] Starting v0.1.0-dev (Canal domain)
[picoceci] Ready.

>
```

The console handles backspace, Ctrl-C (interrupt current input), and Ctrl-D (exit).

### 3.5 Deterministic Memory

picoceci uses a static arena or region-based allocator for its value heap, rather than a
general-purpose garbage collector. This means:

- **No GC pauses** — important for real-time tasks like sensor polling.
- **Predictable peak memory** — the domain's heap budget is fixed at spawn time; you
  cannot accidentally exhaust PSRAM by allocating without bound.
- **Crash recovery** — after a script error, the arena is reset to the start of the
  region, reclaiming all allocations in O(1).

---

## 4. Language Walkthrough

This section gives a condensed tour of picoceci. Assume you are typing into a live REPL
unless noted otherwise.

### 4.1 Values and Types

picoceci is dynamically typed. The core value types are:

| Type | Examples | Notes |
|------|---------|-------|
| Integer | `42`, `-7`, `0` | 64-bit signed |
| Float | `3.14`, `-0.5` | 64-bit IEEE 754 |
| Boolean | `true`, `false` | |
| String | `"hello"` | Immutable, UTF-8 |
| Nil | `nil` | Absence of value |
| List | `[1, 2, 3]` | Heterogeneous |
| Function | `fn(x) { x * x }` | First-class |
| Channel | (returned by `open_channel`) | Canal IPC primitive |

```picoceci
> 1 + 2
=> 3
> "hello" + " world"
=> "hello world"
> true == false
=> false
> [1, 2, 3]
=> [1, 2, 3]
```

### 4.2 Defining Functions (Words)

Functions are defined with `fn` and bound to a name with `let`:

```picoceci
> let square = fn(x) { x * x }
> square(5)
=> 25
```

Functions are values and can be passed as arguments:

```picoceci
> let apply = fn(f, x) { f(x) }
> apply(square, 4)
=> 16
```

Recursive functions refer to themselves by name:

```picoceci
> let factorial = fn(n) {
    if n <= 1 { 1 } else { n * factorial(n - 1) }
  }
> factorial(5)
=> 120
```

### 4.3 Control Flow

**Conditionals** use `if`/`else`:

```picoceci
> let abs = fn(x) { if x < 0 { -x } else { x } }
> abs(-7)
=> 7
```

**Loops** use `while` or functional recursion:

```picoceci
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

For many tasks, recursion is more idiomatic:

```picoceci
> let countdown = fn(n) {
    if n >= 0 {
      println(n)
      countdown(n - 1)
    }
  }
> countdown(3)
3
2
1
0
```

### 4.4 Lists

Lists are created with `[...]` and manipulated with built-in functions:

```picoceci
> let nums = [10, 20, 30]
> head(nums)
=> 10
> tail(nums)
=> [20, 30]
> len(nums)
=> 3
> push(nums, 40)
=> [10, 20, 30, 40]
```

Iteration over a list with a helper:

```picoceci
> let each = fn(lst, f) {
    if len(lst) > 0 {
      f(head(lst))
      each(tail(lst), f)
    }
  }
> each([1, 4, 9], fn(x) { println(x) })
1
4
9
```

### 4.5 Talking to Hardware Through Canal Capabilities

Hardware access in picoceci goes through Canal capabilities, not through raw register
writes. This is by design: a picoceci script cannot crash the Wi-Fi stack or corrupt GPIO
configuration of another domain, because it does not have a capability for those resources
unless one has been explicitly granted.

**GPIO example** (requires `device:gpio` capability):

```picoceci
> let pin = gpio_open(2)        # Open GPIO pin 2
> gpio_write(pin, true)         # Set HIGH
> gpio_write(pin, false)        # Set LOW
```

**Timer example**:

```picoceci
> let blink = fn(pin, n) {
    if n > 0 {
      gpio_write(pin, true)
      sleep_ms(500)
      gpio_write(pin, false)
      sleep_ms(500)
      blink(pin, n - 1)
    }
  }
> let led = gpio_open(2)
> blink(led, 5)                 # Blink 5 times
```

### 4.6 Channels and Concurrency

picoceci channels map directly to Canal IPC channels. You can open a channel to any
service domain that your domain has the capability for:

```picoceci
# Open a channel to the Wi-Fi service domain
> let wifi = open_channel("service:wifi")

# Send a connect request
> send(wifi, {ssid: "MyNetwork", password: "secret"})

# Wait for the acknowledgement
> let result = recv(wifi)
> if result.ok {
    println("Connected!")
  } else {
    println("Failed: " + result.error)
  }
```

Concurrency is expressed with `spawn`, which creates a new lightweight task:

```picoceci
> spawn fn() {
    let ch = open_channel("device:gpio")
    while true {
      send(ch, {pin: 2, value: true})
      sleep_ms(1000)
      send(ch, {pin: 2, value: false})
      sleep_ms(1000)
    }
  }
```

The spawned task runs concurrently with the REPL. Because it communicates through a
capability channel rather than shared memory, there is no race condition even though both
tasks run on the same domain heap.

---

## 5. Running picoceci on the MCU

### 5.1 Prerequisites

You need:

- An ESP32-S3 development board (any board with USB-C or USB-Micro that exposes the
  native USB port, such as the Espressif ESP32-S3-DevKitC-1).
- A data-capable USB cable.
- TinyGo 0.31.0 or later, ESP-IDF (for flashing), and `esptool.py` installed on your
  host machine. See the [Canal quick-start guide](../GETTING_STARTED.md) for exact
  installation steps.

### 5.2 Building and Flashing the Canal Image

```bash
# Clone the repository (if you haven't already)
git clone https://github.com/kristofer/Canal.git
cd Canal/canal

# Set up dependencies (downloads FatFS, mbedTLS, etc.)
chmod +x scripts/setup.sh
./scripts/setup.sh esp32s3

# Build the full Canal image including the picoceci domain
make TARGET=esp32s3

# Flash to the connected board
make flash PORT=/dev/ttyUSB0    # Linux
# make flash PORT=/dev/cu.usbmodem*  # macOS
```

### 5.3 Connecting to the REPL

```bash
# Open the serial monitor (115200 baud)
make monitor PORT=/dev/ttyUSB0
```

You should see the Canal boot log followed by the picoceci banner:

```
=== Canal ESP32-S3 ===
Boot time: 342 ms
...
=== Boot Complete ===
[picoceci] Starting v0.1.0-dev (Canal domain)
[picoceci] Ready.

>
```

### 5.4 Your First Program

Try this at the prompt:

```picoceci
> let greet = fn(name) { "Hello, " + name + "!" }
> greet("Canal")
=> "Hello, Canal!"
```

Then try something hardware-related:

```picoceci
> let led = gpio_open(2)
> gpio_write(led, true)    # LED on
> sleep_ms(1000)
> gpio_write(led, false)   # LED off
```

If the LED on your board blinks, you have a working picoceci environment on live hardware.

### 5.5 Loading a Program from a File

Once Canal's SD-card domain is fully wired (currently in progress), you will be able to
load a picoceci source file from the SD card:

```picoceci
> load("/sdcard/blink.pico")
[loaded blink.pico]
> blink(gpio_open(2), 10)
```

For now, you can paste multi-line programs directly into the REPL—the parser handles
incomplete input gracefully.

---

## Summary

picoceci is a purpose-built scripting language for Canal: small enough to live in a 32 KB
heap, expressive enough to teach real programming concepts, and integrated tightly with
Canal's channel-based capability model. It borrows a small Forth-style bytecode VM, Lisp's
first-class functions, Go's channels, and Logo's interactive REPL spirit—combining them
into a language that is at home on a microcontroller yet familiar enough for developers
coming from mainstream programming backgrounds.

In [Article 3](./03-canal-and-freertos-go-on-bare-metal.md) we will go deeper into Canal's
architecture: how FreeRTOS tasks map to domains, how TinyGo's runtime fits inside an
isolated domain, and how the kernel arbitrates capability-mediated inter-domain messages.

---

## Exercises

1. **Language influences.** picoceci draws design ideas from Forth, Scheme, Go, and Logo.
   Choose any two of those languages and write a short paragraph for each explaining which
   specific picoceci feature reflects that influence and why the designers likely borrowed
   it.

2. **REPL practice.** Open (or imagine) a picoceci REPL session. Write a short program
   that:
   - Defines a function called `square` that returns the square of its argument.
   - Uses `square` inside a loop to print the squares of 1 through 5.
   Label each line with a comment explaining what it does.

3. **Memory model.** picoceci uses "deterministic memory" (static arena or region-based
   allocation). Compare this to garbage collection: list one benefit and one drawback of
   each approach in the context of a microcontroller with 512 KB of RAM.

4. **Hot-loadable definitions.** Explain what "hot-loadable definitions" means in the
   context of picoceci. Describe a practical development scenario where being able to
   redefine a function without rebooting the board would save significant time.
