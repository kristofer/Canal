# Article 2: picoceci — A Smalltalk-syntax, Go-semantics Language for Microcontrollers

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
Canal domain on the ESP32-S3. It is described as "a small, high-protein language" — a
**Smalltalk-syntax, Go-semantics** interpreted language designed for microcontrollers.
picoceci gives you an interactive REPL (Read-Eval-Print Loop) over USB serial,
a message-passing object model, and first-class access to Canal's capability-based
inter-domain communication and FreeRTOS concurrency primitives — all within the tight
memory budget of a microcontroller.

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
  footprint push the memory budget uncomfortably. Python syntax can also hide the
  message-passing model that maps naturally onto Canal capabilities.
- **Lua** — lighter than Python, but the standard Lua VM still assumes a conventional OS
  heap and does not integrate cleanly with Canal's capability model.
- **JavaScript (e.g., Espruino / Duktape)** — interactive REPL is great, but heap
  fragmentation is a real concern over long run-times on small devices.
- **Forth** — runs on almost nothing, but its postfix stack-based syntax is famously
  unapproachable for programmers who have not seen it before.

None of these hit the exact target: a language that is *resource-safe*, *teachable*,
*interactive*, and whose object/message model *maps naturally* onto Canal's capability
objects and FreeRTOS concurrency primitives.

### The Three Constraints

**1. Tight resource budget.**
The picoceci interpreter and runtime must fit in less than 128 KB of RAM on the
ESP32-S3-N16R8. The bytecode representation must be compact enough to fit in the
remaining flash budget, and the runtime must not rely on unbounded heap growth.

**2. Safety without raw pointers.**
A crashed picoceci script must not corrupt the Canal kernel, other domains, or the
interpreter itself. Canal's capability model enforces this at the kernel level, and
picoceci reinforces it: there is no raw pointer arithmetic, no way to call arbitrary C
functions, and no way to forge a capability handle.

**3. Familiar, composable, and concurrent.**
The syntax should feel familiar to programmers who have seen Smalltalk or Ruby. The
object model should favour composition over inheritance (like Go's interfaces and
embedding). Concurrency should be first-class and map directly onto FreeRTOS tasks,
queues, and semaphores via TinyGo.

---

## 2. Influences

picoceci draws deliberately from two primary sources. Understanding these influences
helps you read picoceci code and reason about its design trade-offs.

### 2.1 Smalltalk — Syntax and Message-Passing

Smalltalk pioneered the idea that *everything is an object* and all computation is
*sending messages*. picoceci borrows Smalltalk's elegant three-level message grammar:

| Level | Form | Example |
|-------|------|---------|
| Unary | receiver message | `42 factorial` |
| Binary | receiver op arg | `3 + 4` |
| Keyword | receiver key: arg … | `dict at: #key put: value` |

picoceci also inherits Smalltalk's **blocks** — first-class closures written as
`[ :arg | expression ]` — which provide a uniform mechanism for control flow,
iteration, and deferred computation.

Comments in picoceci use Smalltalk's double-quote style:

```picoceci
"This is a comment — ignored by the parser."
```

What picoceci does *not* borrow is Smalltalk's class hierarchy and inheritance. Instead
it adopts Go's philosophy of *composition over inheritance*.

### 2.2 Go — Semantics, Interfaces, and Concurrency

Go contributes the *semantics* of picoceci:

- **Structural typing** — an object satisfies an interface if it responds to all the
  required messages, with no explicit `implements` declaration.
- **Composition over inheritance** — objects are assembled from other objects using
  `compose`, mirroring Go's struct embedding.
- **Concurrency via channels and queues** — picoceci exposes FreeRTOS primitives
  (`Task`, `Queue`, `Semaphore`, `Channel`) that mirror Go's goroutines and channels.

This alignment with Go is intentional: Canal itself is implemented in TinyGo, and the
kernel IPC is already built around FreeRTOS queues. picoceci scripts use the same
conceptual abstractions as the kernel around them.

---

## 3. Distinctive Features

### 3.1 Message-Passing Object Model

Every operation in picoceci is a *message send*. There are no standalone functions — only
objects that respond to messages. This makes the language highly uniform: the same
syntax that adds two numbers also controls an LED or queries a sensor.

```picoceci
"Arithmetic — binary message"
3 + 4.              "=> 7"

"String concatenation — binary message"
'Hello, ' , 'picoceci!'.

"Conditional — keyword message sent to a boolean"
(x > 0) ifTrue: [ Console println: 'positive' ].

"Iteration — keyword message sent to an integer"
5 timesRepeat: [ Console println: 'tick' ].
```

### 3.2 Objects Defined by Composition, Not Inheritance

picoceci has no class hierarchy. An `object` declaration defines a named prototype with
instance variable slots and methods. Objects gain behaviour from other objects via
`compose` — analogous to Go's struct embedding:

```picoceci
object Counter {
    | count |
    init  [ count := 0 ]
    inc   [ count := count + 1. ^self ]
    value [ ^count ]
}

object LoggedCounter {
    compose Counter.
    inc [
        super inc.
        Console println: 'incremented to ', self value printString.
        ^self
    ]
}
```

`LoggedCounter` has all the slots and methods of `Counter`, and overrides `inc` to add
logging. There is no runtime class hierarchy — `super` dispatches to the composed
object's method, not up an inheritance chain.

### 3.3 Structural Typing via Interfaces

picoceci uses **structural typing**: declare an `interface` as a set of messages, and any
object that responds to all those messages satisfies the interface — no declaration needed.

```picoceci
interface Incrementable {
    inc
    dec
    value
}

| c |
c := LoggedCounter new.
(c satisfies: Incrementable) ifTrue: [ Console println: 'yes' ].
```

This is Go's duck typing brought to a Smalltalk-syntax language.

### 3.4 First-Class Blocks (Closures)

Blocks are the primary abstraction for deferred computation, iteration, and concurrency.
A block captures its enclosing scope:

```picoceci
| adder |
adder := [ :n | [ :x | x + n ] ].
(adder value: 5) value: 3.   "=> 8"
```

Blocks are how picoceci implements control flow without special keywords — `ifTrue:`,
`whileTrue:`, `timesRepeat:`, and `do:` are all ordinary keyword messages that take
blocks as arguments.

### 3.5 Deterministic Memory

picoceci uses reference counting (for MCU targets) or an arena allocator, not a
stop-the-world garbage collector. This means:

- **No GC pauses** — important for real-time tasks like sensor polling.
- **Predictable peak memory** — the heap budget is fixed at spawn time.
- **Small integers, booleans, nil, and characters** are *immediate values* encoded
  in the pointer word — they never touch the heap.

### 3.6 Interactive REPL over Serial/USB

The picoceci domain opens a line-oriented console on `machine.Serial`, which maps to USB
CDC (USB serial) on the ESP32-S3. Connect with any serial terminal at 115200 baud:

```
[picoceci] Starting v0.1.0-dev (Canal domain)
[picoceci] Ready.

>
```

The REPL compiles and evaluates each line as you type it. Every object definition is
immediately usable in the next line, enabling a tight edit-and-try loop on live hardware
without rebooting.

---

## 4. Language Walkthrough

This section gives a condensed tour of picoceci. Assume you are typing into a live REPL
unless noted otherwise.

### 4.1 Variables and Assignment

Variables must be declared in a `| ... |` block before use. Assignment uses `:=`:

```picoceci
| x y |
x := 42.
y := x + 1.
Console println: y printString.   "=> 43"
```

Comments are double-quoted strings and are ignored by the interpreter.

### 4.2 Values and Types

picoceci is dynamically typed at the script level, but the runtime tags every value with
its kind. The immediate (non-heap) types are:

| Type | Examples | Notes |
|------|---------|-------|
| SmallInt | `42`, `-7`, `0` | Tagged pointer — no heap allocation |
| Float | `3.14`, `-0.5` | 64-bit IEEE 754 |
| Bool | `true`, `false` | Tagged pointer |
| Nil | `nil` | Absence of value |
| Char | `$A`, `$\n` | Unicode code point |

Heap-allocated types include `String` (single-quoted, immutable), `Symbol` (`#hello`,
interned), `Array` (`#(1 2 3)`), `ByteArray` (`#[1 2 255]`), `Block`, and user-defined
`Object` instances.

```picoceci
"Integer arithmetic"
3 + 4.                    "=> 7"
10 \\ 3.                  "=> 1  (modulo — Smalltalk uses \\ not %)"
10 // 3.                  "=> 3  (integer division)"

"String concatenation"
'Hello' , ', ' , 'world'. "=> 'Hello, world'"

"Boolean"
(3 > 2) & (1 < 5).        "=> true"
true not.                 "=> false"

"Symbol (interned — same characters, same object)"
#hello == #hello.         "=> true"
```

### 4.3 Blocks and Control Flow

All control flow in picoceci is implemented as keyword messages that accept blocks.
There are no `if`, `while`, or `for` keywords.

**Conditional:**

```picoceci
| x |
x := -7.
(x < 0)
    ifTrue:  [ Console println: 'negative' ]
    ifFalse: [ Console println: 'non-negative' ].
```

**Loops:**

```picoceci
"Count from 1 to 5"
1 to: 5 do: [ :i | Console println: i printString ].

"While loop"
| n |
n := 10.
[ n > 0 ] whileTrue: [ n := n - 1 ].

"Repeat N times"
5 timesRepeat: [ Console println: 'tick' ].
```

**Collection iteration:**

```picoceci
#(1 2 3 4) do: [ :each |
    Console println: each printString
].

| doubled |
doubled := #(1 2 3) collect: [ :each | each * 2 ].
"=> #(2 4 6)"

| sum |
sum := #(1 2 3 4) inject: 0 into: [ :acc :each | acc + each ].
"=> 10"
```

### 4.4 Defining Objects

Objects are picoceci's primary unit of abstraction. They have instance variables
(declared in `| ... |`) and methods (each method is a block headed by its message
signature):

```picoceci
object Counter {
    | count |

    init [
        count := 0
    ]

    inc [
        count := count + 1.
        ^self
    ]

    dec [
        count := count - 1.
        ^self
    ]

    add: n [
        count := count + n.
        ^self
    ]

    value [
        ^count
    ]

    printString [
        ^'Counter(', count printString, ')'
    ]
}

| c |
c := Counter new.
c inc; inc; inc.           "cascade: send inc three times to c"
Console println: c value printString.   "=> 3"
c add: 10.
Console println: c printString.        "=> Counter(13)"
```

`^` returns a value from a method. `^self` returns the receiver, enabling cascades with `;`.

### 4.5 Composition

`compose` pulls all slots and methods of another object into the current one:

```picoceci
object LoggedCounter {
    compose Counter.

    inc [
        super inc.
        Console println: 'incremented to ', self value printString.
        ^self
    ]
}

| lc |
lc := LoggedCounter new.
lc inc; inc.
"prints:
incremented to 1
incremented to 2"
```

### 4.6 Talking to Hardware Through Canal Capabilities

Hardware access in picoceci goes through Canal capability objects. A picoceci script
cannot crash the Wi-Fi stack or corrupt GPIO configuration of another domain, because it
does not hold a capability for those resources unless one has been explicitly granted.

**GPIO:**

```picoceci
| led |
led := GPIO pin: 2 direction: #output.
led high.
Task delay: 500.    "wait 500 ms"
led low.
```

**Blink loop:**

```picoceci
| led |
led := GPIO pin: 2 direction: #output.
[ true ] whileTrue: [
    led toggle.
    Task delay: 500
].
```

**UART:**

```picoceci
| uart |
uart := UART new: 0 baud: 115200.
uart println: 'Hello from picoceci'.
```

**Acquiring a Canal capability by name:**

```picoceci
| cap |
cap := Canal capability: #uart0.
cap send: 'hello\n' asBytes.
cap close.
```

### 4.7 Concurrency with FreeRTOS Primitives

picoceci exposes FreeRTOS tasks, queues, semaphores, and timers as first-class objects.

**Spawning a task:**

```picoceci
| task |
task := Task spawn: [
    | led |
    led := GPIO pin: 2 direction: #output.
    [ true ] whileTrue: [
        led toggle.
        Task delay: 1000
    ]
].
task name: 'blinker'.
```

**Producer/consumer with a Queue:**

```picoceci
| q |
q := Queue new: 10.

Task spawn: [
    1 to: 5 do: [ :i | q send: i ]
].

Task spawn: [
    5 timesRepeat: [
        | item |
        item := q receive.
        Console println: item printString
    ]
].
```

**Higher-level Channel (Go-like `<-` operator syntax):**

Channels provide a higher-level abstraction over `Queue`. For familiarity with Go
developers, picoceci uses `<-` operator syntax for send and receive rather than keyword
messages:

```picoceci
| ch |
ch := Channel new: 5.
ch <- 42.         "send  (binary operator '<-')"
| v |
v := <-ch.        "receive (unary operator '<-')"
Console println: v printString.   "=> 42"
```

**Semaphore:**

```picoceci
| sem |
sem := Semaphore new.    "binary semaphore"
sem take.
"... critical section ..."
sem give.
```

**Timer:**

```picoceci
| t |
t := Timer every: 1000 do: [ Console println: 'tick' ].
Task delay: 5000.
t stop.
```

---

## 5. Running picoceci on the MCU

### 5.1 Prerequisites

You need:

- An ESP32-S3 development board (such as the Espressif ESP32-S3-N16R8 or any compatible
  board that exposes the native USB port).
- A data-capable USB cable.
- TinyGo 0.32 or later and the `esptool.py` flash utility installed on your host machine.
  See the [Canal quick-start guide](../GETTING_STARTED.md) for exact installation steps.

### 5.2 Building and Flashing

picoceci is built as part of the Canal image and lives in `Canal/canal/domains/picoceci/`.

```bash
# Clone the repository (if you haven't already)
git clone https://github.com/kristofer/Canal.git
cd Canal/canal

# Build, flash, and open a monitor in one step
make picoceci-run
```

Or step by step:

```bash
# Flash the picoceci binary
tinygo flash -target=esp32s3-generic \
             -port=/dev/cu.usbmodem11201 \
             ./target/esp32s3

# Open the serial monitor
tinygo monitor
```

### 5.3 Connecting to the REPL

You should see the Canal boot log followed by the picoceci banner:

```
[picoceci] Starting v0.1.0-dev (Canal domain)
[picoceci] Ready.

>
```

The console handles backspace, Ctrl-C (interrupt current input), and Ctrl-D (exit).

### 5.4 Your First Program

Try arithmetic and string output at the prompt:

```picoceci
> Console println: 'Hello, picoceci!'.
Hello, picoceci!

> 3 + 4.
7
```

Then define an object and use it interactively:

```picoceci
> object Greeter {
    greet: name [
        Console println: 'Hello, ', name, '!'
    ]
  }
> Greeter new greet: 'Canal'.
Hello, Canal!
```

Then try something hardware-related:

```picoceci
> | led |
> led := GPIO pin: 2 direction: #output.
> led high.
> Task delay: 1000.
> led low.
```

If the LED on your board blinks, you have a working picoceci environment on live hardware.

### 5.5 Loading a Program from a File

Once Canal's SD-card domain is fully wired, you will be able to import picoceci source
files from the SD card (files conventionally use the `.pc` extension; the extension
may be omitted from `import` statements):

```picoceci
import '/sdcard/blink.pc'.

| led |
led := GPIO pin: 2 direction: #output.
Blinker runOn: led times: 10.
```

For now, you can paste multi-line programs directly into the REPL — the parser handles
incomplete input gracefully.

---

## Summary

picoceci is a purpose-built scripting language for Canal: small enough to fit the ESP32-S3
memory budget, expressive enough to teach real programming concepts, and integrated
tightly with Canal's capability model and FreeRTOS concurrency primitives. It combines
**Smalltalk's elegant message-passing syntax** with **Go's structural typing and
composition-over-inheritance semantics**, arriving at a language that feels familiar to
developers from both camps while being at home on a microcontroller.

Key design choices to remember:

| Choice | Rationale |
|--------|-----------|
| Smalltalk message syntax | Uniform, readable, no special `if`/`while` keywords |
| No class hierarchy | Composition via `compose` is simpler and avoids fragile inheritance |
| Structural typing | Objects satisfy interfaces automatically — like Go's duck typing |
| FreeRTOS-backed concurrency | Real hardware tasks, queues, and semaphores — not simulated |
| Canal capabilities as objects | Safe access to hardware without raw pointers |

In [Article 3](./03-canal-and-freertos-go-on-bare-metal.md) we will go deeper into Canal's
architecture: how FreeRTOS tasks map to domains, how TinyGo's runtime fits inside an
isolated domain, and how the kernel arbitrates capability-mediated inter-domain messages.

---

## Exercises

1. **Language influences.** picoceci takes its syntax from Smalltalk and its object
   semantics from Go. Choose one feature from each language and write a short paragraph
   explaining *why* that design choice is a good fit for an embedded scripting language
   running on a microcontroller.

2. **REPL practice.** Open (or imagine) a picoceci REPL session. Write a short program
   that:
   - Defines an object `Accumulator` with an `add:` method and a `total` method.
   - Creates an instance, adds the numbers 1 through 5 using a `to:do:` loop, and prints
     the total.
   Label each line with a comment explaining what it does.

3. **Composition vs inheritance.** picoceci uses `compose` instead of class inheritance.
   Given the `Counter` and `LoggedCounter` examples in section 4.5, explain what would
   happen in a traditional inheritance-based language if someone added a new method to the
   parent class `Counter`. Would the same problem arise with picoceci's `compose`? Why or
   why not?

4. **Concurrency model.** Describe the difference between a `Queue` and a `Channel` in
   picoceci. In what scenario would you prefer a `Queue`, and in what scenario would you
   prefer a `Channel`?

5. **Memory model.** picoceci avoids a stop-the-world GC, using reference counting or
   arena allocation instead. List one benefit and one drawback of each approach in the
   context of a real-time sensor-polling task on a microcontroller.
