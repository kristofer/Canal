# Canal: A Capability-Based Operating System for IoT Education

**Teaching Secure, Resilient Embedded Systems Development**

*Version 1.0 - May 2026*

**Author:** Kristofer Younger, Director of Education, ZipCodeWilmington  
**Repository:** <https://github.com/kristofer/Canal>  
**License:** MIT

---

## Abstract

Canal is a capability-based microkernel operating system designed for embedded IoT devices, written in Go using TinyGo. Unlike traditional monolithic embedded systems where a single bug can crash the entire device, Canal isolates different functions (sensors, networking, data logging) into independent "domains" that cannot interfere with each other. This architecture makes Canal an ideal platform for teaching IoT development because students learn secure design patterns from day one, see immediate consequences of errors without bricking hardware, and understand how real operating systems manage resources. This paper presents Canal's design principles, demonstrates its application to IoT scenarios, and explains why domain isolation and capability-based security are essential concepts for the next generation of embedded systems developers.

---

## 1. Introduction: Why IoT Needs a New Approach

### 1.1 The Problem with Traditional Embedded Development

When you flash traditional firmware to a microcontroller (MCU), everything runs in a single memory space:

```
Traditional MCU:
┌─────────────────────────────────┐
│  All Code in One Binary         │
│  ├─ Sensor Reading              │
│  ├─ WiFi Stack                  │
│  ├─ Data Logger                 │
│  └─ LED Blinker                 │
│                                 │
│  Bug in ANY part crashes ALL    │
└─────────────────────────────────┘
```

**Real-world consequences:**

- A buffer overflow in WiFi code corrupts sensor data
- A faulty temperature sensor crashes the entire device
- Updating one feature requires reflashing everything
- Students fear experimenting because mistakes brick hardware
- Production devices fail in the field and can't recover

### 1.2 What Canal Changes

Canal runs each function as an isolated **domain** (like a program, but more protected/cocooned) with its own memory, permissions, and failure boundary:

```
Canal MCU:
┌─────────────────────────────────┐
│          Kernel (20KB)          │
│     Capability Manager          │
└────────┬────────────────────────┘
         │
    ┌────┴─────┬─────────┬────────┐
    │          │         │        │
┌───▼────┐ ┌──▼───┐ ┌───▼──┐ ┌───▼──┐
│Sensor  │ │WiFi  │ │Logger│ │ LED  │
│Domain  │ │Domain│ │Domain│ │Domain│
└────────┘ └──────┘ └──────┘ └──────┘

Bug in WiFi? → WiFi restarts
               Other domains keep running
```

**For students, this means:**

- Experiment fearlessly - bugs are isolated
- See how operating systems actually work
- Learn security by design, not as an afterthought
- Build production-ready IoT systems
- Update deployed devices over WiFi (OTA updates)

---

## 2. Core Concepts: Domains, Canals, and Capabilities

### 2.1 Domains: Isolated Functions

A **domain** is an isolated program running on the MCU. Think of it like a smartphone app, but for embedded systems. When it crashes, it doesn't take down the whole device - just that domain. Each domain has its own memory space and can only access hardware or services through explicit capabilities.

**Example - Temperature Monitor System:**

```go
// Domain 1: Temperature Sensor Reader
package main

func main() {
    sensor := requestCapability("device:i2c:0x48") // BME280 sensor
    logger := requestCapability("service:logger")
    
    for {
        temp := readTemperature(sensor)
        log(logger, "Temperature: ", temp)
        time.Sleep(60 * time.Second)
    }
}
```

```go
// Domain 2: Data Logger
package main

func main() {
    sdcard := requestCapability("service:filesystem")
    
    for {
        msg := receiveLog()
        appendToFile(sdcard, "/logs/temp.csv", msg)
    }
}
```

**What students learn:**

- Each domain has a single responsibility (like microservices)
- Domains communicate through message passing (no shared memory)
- If sensor code crashes, logger keeps running
- Clear separation of concerns from day one

### 2.2 Canals: Communication Channels

Domains communicate through **canals** - typed message channels built on Go's native channel system. Named after the original "ChannelOS" concept, canals enforce type safety and prevent unauthorized communication.

**Example - Sensor to Logger Canal:**

```go
// Sensor domain sends structured data
type SensorReading struct {
    Timestamp uint64
    Temperature float32
    Humidity float32
}

// Send through canal (just a variable), which is a channel, only able to send/receive SensorReading types
canal <- SensorReading{
    Timestamp: getCurrentTime(),
    Temperature: 23.5,
    Humidity: 65.0,
}
```

```go
// Logger domain receives only valid data
reading := <-canal
saveToFile(reading)
```

**Benefits for learning:**

- Type-safe communication (compiler catches mistakes)
- No race conditions (channels are synchronized)
- Clear data flow (request/response patterns)
- Maps to Go concurrency students already know

The Go programming model of _chan_nels fits perfectly with the concept of canals, making it intuitive for students to write concurrent, isolated code without worrying about low-level synchronization issues. No mutexes, no shared memory - just safe, structured communication.

This allows students to focus on the logic of their applications while learning fundamental OS concepts like isolation, interprocess communication (IPC), and resource management in a real embedded environment.

### 2.3 Capabilities: Permission Tokens

A **capability** is an un-forge-able token granting specific permissions to the domain. You can't access hardware or services without the right capability. It is like a "right" to the device or service. Without it, your domain is just code in memory with no power. With it, you can do specific things - but only what the capability allows.

**Example - GPIO Access:**

```go
// ❌ Traditional approach (direct hardware access)
machine.GPIO8.High()  // Anyone can touch any pin

// ✅ Canal approach (capability-based)
ledPin := requestCapability("device:gpio:8", WRITE)
if ledPin != nil {
    ledPin.Set(HIGH)
}
```

**The Security Model:**

1. **Least Privilege**: Domains only get what they need
2. **No Ambient Authority**: Can't "just access" hardware
3. **Explicit Delegation**: Capabilities can be transferred
4. **Revocable**: Permissions can be taken away

**Real IoT scenario:**

This example shows how a temperature sensor domain only gets access to the I2C bus and logger service, but not WiFi or filesystem. Even if the sensor code is compromised, it can't send data over the network or modify files.

```
Temperature Sensor Domain needs:
  ✓ I2C bus 0 (read) - for sensor
  ✓ Logger service (write) - for data
  ✗ WiFi - doesn't need network
  ✗ Flash filesystem - doesn't need storage
  ✗ GPIO pins - doesn't control outputs

→ If compromised, attacker still can't:
   - Send data over network
   - Modify stored files  
   - Control other hardware
```

---

## 3. Resilience: How Canal Handles Failures

### 3.1 Domain Crash Isolation

When operating in a high-failure environment like IoT, crashes are inevitable. Traditional firmware crashes take down the whole device, requiring a physical reset and causing data loss. Canal's domain isolation means that if one domain crashes, it can be restarted without affecting others.

This pushes the concept of "resilience" to the lowest level of the system. Instead of a single point of failure, each domain is a self-contained unit that can fail and recover independently. This is a fundamental shift in how embedded systems are designed and taught, as it allows students to see the benefits of fault isolation and recovery in real time.

**Traditional firmware crash:**

```
[10:23:14] Reading sensor...
[10:23:15] WiFi connecting...
[10:23:16] *** NULL POINTER in WiFi code ***
[10:23:16] SYSTEM HALT
           
           → Device is dead
           → Must physically reset
           → Loses all data
```

**Canal domain crash:**

```
[10:23:14] [Sensor] Reading temperature: 23.5°C
[10:23:15] [WiFi] Connecting to network...
[10:23:16] [Kernel] ⚠️  WiFi domain crashed (null pointer)
[10:23:16] [Kernel] Restarting WiFi domain...
[10:23:17] [WiFi] Reconnecting to network...
[10:23:14] [Sensor] Reading temperature: 23.6°C ← Still working!
[10:23:14] [Logger] Writing to SD card ← Still working!

           → Faulty domain restarted
           → Other domains unaffected
           → No data loss
```

**How it works:**

1. **Memory Protection Unit (MPU)** catches invalid memory access
2. **Kernel fault handler** identifies crashed domain
3. **Domain manager** marks domain as dead, frees resources
4. **Restart policy** spawns fresh domain instance
5. **Other domains** never knew anything happened

### 3.2 Teaching Moment: Error Recovery

Students can deliberately crash domains to understand recovery:

```go
// Exercise: Crash the WiFi domain and observe recovery

package main

func main() {
    println("[WiFi] Starting...")
    
    // Intentional crash after 5 seconds
    time.Sleep(5 * time.Second)
    
    // Dereference null pointer
    var ptr *int
    *ptr = 42  // ← Causes MPU fault
    
    // Never reaches here
    println("This won't print")
}
```

**Expected output:**

```
[WiFi] Starting...
[Kernel] Domain 2 (wifi) crashed at 0x40001234
[Kernel] Fault type: Memory access violation
[Kernel] Restarting domain 2...
[WiFi] Starting...
```

**What students learn:**

- How operating systems handle faults
- Importance of defensive programming
- Difference between recoverable and fatal errors
- How production IoT devices stay online

### 3.3 Network Partition Resilience

Operating in a trusted, yet not quite reliable environment is a common scenario for IoT devices. Networks can drop, servers can go down, and power can fluctuate. Canal's domain isolation allows the device to continue functioning even when external dependencies fail.

Wifi can go down, and instead of the whole device crashing or losing data, the sensor domain can keep reading and logging locally. When the network comes back, it can sync data without any loss. This teaches students about designing for real-world conditions where perfect connectivity can't be assumed. (Like on-board a ship in the middle of the ocean, or a weather station in a remote location, or a spacecraft orbiting Earth - all places where network reliability is not guaranteed.)

**IoT Challenge:** WiFi drops, server goes down, or network splits

**Canal's approach:** Domains continue local operation

```go
// Weather Station Domain
package main

func main() {
    sensor := requestCapability("device:bme280")
    sdcard := requestCapability("service:filesystem")
    network := requestCapability("service:wifi")
    
    for {
        reading := readSensor(sensor)
        
        // Always log locally (even if network is down)
        logToSDCard(sdcard, reading)
        
        // Try to upload (fails gracefully if offline)
        err := uploadToCloud(network, reading)
        if err != nil {
            println("Offline - data saved locally")
        }
        
        time.Sleep(5 * time.Minute)
    }
}
```

**Behavior:**

- **Network available**: Upload readings, local backup
- **Network down**: Save locally, retry on reconnect
- **SD card fails**: Upload to cloud if possible
- **Both fail**: Keep latest reading in RAM

**Educational value:** Students learn distributed systems concepts on an MCU

---

## 4. IoT Learning Applications

### 4.1 Progressive Complexity Path

Starting small and building up complexity is key to learning. Canal allows students to start with simple domains and gradually add more features, teaching new concepts at each step while reinforcing previous lessons.

Decomposing a complex IoT system into isolated domains also teaches good software architecture principles, making it easier for students to manage complexity as they add features like networking, storage, and OTA updates.

But we have to provide a student with getting *something* working early on to build confidence and motivation. (Even if that is nothing more than a blinking LED on an MCU board.) Then we can build on that foundation with more complex domains and interactions, showing how the concepts of isolation, capabilities, and resilience apply at each step.

Canal allows students to start simple and add complexity gradually:

#### **Week 1: Hello World Domain**

Start really simply. Just blink an LED using a GPIO capability. This teaches the basics of domains, capabilities, and hardware access without overwhelming students with too many concepts at once.

```go
package main

func main() {
    led := requestCapability("device:gpio:8", WRITE)
    
    for {
        led.Set(HIGH)
        time.Sleep(500 * time.Millisecond)
        led.Set(LOW)
        time.Sleep(500 * time.Millisecond)
    }
}
```

**Learning:** Domain basics, capability requests, GPIO control

#### **Week 2: Sensor Reading Domain**

Adds a sensor domain that reads from a BME280 temperature/humidity sensor over I2C. This introduces students to hardware communication protocols and structured data, while still keeping the domain simple and focused.

```go
package main

func main() {
    i2c := requestCapability("device:i2c:0", READ|WRITE)
    bme280 := initBME280(i2c)
    
    for {
        temp := bme280.ReadTemperature()
        println("Temperature:", temp, "°C")
        time.Sleep(1 * time.Minute)
    }
}
```

**Learning:** I2C communication, sensor protocols, structured data

#### **Week 3: Multi-Domain System**

Capturing data and then logging it to an SD card in a separate domain. This teaches inter-domain communication through canals, as well as the concept of decoupling different functions into separate domains.

```go
// Sensor Domain → Logger Domain → Display Domain

// Sensor domain
canal <- SensorReading{Temp: 23.5, Humidity: 65}

// Logger domain
reading := <-canal
logToFile(reading)
displayCanal <- reading

// Display domain
reading := <-displayCanal
updateOLED(reading.Temp, reading.Humidity)
```

**Learning:** Inter-domain communication, data pipelines, decoupling

#### **Week 4: Network-Connected System**

Adding in the networking domain to upload data to the cloud. This introduces students to network programming, REST APIs, and error handling in a distributed system, while still maintaining isolation between domains.

```go
// Weather Station: Sensor → Logger → WiFi Uploader

// Add WiFi domain
wifi := requestCapability("service:wifi")
wifi.Connect("MyNetwork", "password")

server := requestCapability("service:http-client")
server.Post("https://api.weather.com/readings", reading)
```

**Learning:** Network protocols, REST APIs, error handling

#### **Week 5: Production System**

Now we have a complete IoT device with sensor reading, local logging, and cloud connectivity. We can add an **over-the-air** (OTA) update domain to show how devices can be maintained after deployment, teaching students about real-world device management and versioning.

```go
// Complete IoT device with OTA updates

// Add update domain
updater := requestCapability("service:ota")
if updater.CheckForUpdate() {
    println("Downloading new sensor firmware...")
    updater.UpdateDomain("sensor", "v1.2.0")
    // Sensor domain automatically restarts with new code
}
```

**Learning:** Device management, versioning, deployment

### 4.2 Hands-On Projects for Students

#### **Project 1: Smart Plant Monitor**

Larger systems that are inter-related but still isolated are great for teaching the value of domain separation. A smart plant monitor with multiple sensors, a water pump, local logging, and cloud connectivity can be broken down into several domains that interact through canals, demonstrating how complex systems can be built from simple, isolated components.

Each of these domains can be developed and tested independently, allowing students to see the benefits of isolation and modularity while building a real-world IoT application.

**Domains:**

- Soil moisture sensor reader
- Water pump controller
- Data logger to SD card
- Web dashboard over WiFi
- Alert system (SMS/email)

**Learning objectives:**

- Analog sensor interfacing
- Actuator control with safety limits
- Time-series data storage
- Web server implementation
- External API integration

**Failure scenarios to handle:**

- Sensor disconnected
- Pump stuck on/off
- SD card full
- WiFi dropped
- Server unreachable

#### **Project 2: Air Quality Monitor**

Onboard an aircraft or spacecraft, you can't afford to have a single point of failure. An air quality monitor with multiple sensors (particulate matter, temperature/humidity), a local display, and cloud connectivity can be designed with strict domain isolation to ensure that if one sensor fails or the network goes down, the device continues to function and provide critical information. This project teaches students about designing for high-reliability environments and the importance of fault tolerance in IoT systems.

**Domains:**

- Particulate matter sensor (PM2.5/PM10)
- Temperature/humidity sensor
- OLED display controller
- Data upload to cloud
- Local data cache

**Learning objectives:**

- Multiple I2C devices on one bus
- Display graphics and UI
- Cloud platform integration (ThingSpeak, Adafruit IO)
- Data retention during offline periods
- Power management

#### **Project 3: Smart Doorbell**

But we also need to show how real-world IoT devices often have multiple functions that need to be isolated for security. A smart doorbell with a motion sensor, camera, local storage, WiFi connectivity, and audio playback can be designed with strict domain separation to prevent a compromise in one area (like the camera) from affecting the entire system. This project teaches students about designing secure IoT devices where sensitive functions are isolated from network-facing components.

This kind of project is also a lot more comprehensiable and fun for student whose imagination is not yet able to grasp the problems of a spacecraft and Deep Black of space.

**Domains:**

- Motion sensor (PIR)
- Camera controller
- Image storage
- WiFi notification sender
- Audio playback

**Learning objectives:**

- Interrupt-driven sensors
- Camera interfacing and JPEG encoding
- File system management
- Real-time notifications
- Audio codec control

**Security lesson:**
Each domain runs with minimal permissions. Camera domain can't send images over network - must pass them to upload domain through canal. This prevents backdoor access to camera feed.

### 4.3 Teaching Security Through Practice

These example exercises show how students can learn about security concepts like isolation, least privilege, and defense in depth by trying to break the system themselves. By attempting to access memory from another domain or forge capabilities, students see firsthand how hardware-enforced protections work and why they are essential for secure IoT design.

#### **Exercise: Breaking Isolation**

Give students a challenge: "Try to access another domain's memory"

```go
// Attacker domain attempts to read sensor data
package main

func main() {
    // Try to access sensor domain's memory
    sensorMemory := (*[1024]byte)(unsafe.Pointer(uintptr(0x3FC80000)))
    
    // Read attempt
    secretData := sensorMemory[0]  // ← MPU fault!
}
```

**Expected result:**

```
[Kernel] Domain 5 (attacker) crashed
[Kernel] Fault: Memory protection violation
[Kernel] Attempted access to 0x3FC80000 (domain 1 memory)
[Kernel] Access denied - domain killed
```

**What students learn:**

- Hardware memory protection is real
- Isolation isn't just software convention
- Security is enforced by hardware, not trust

#### **Exercise: Capability Forgery**

Challenge: "Create a GPIO capability without requesting it"

```go
// Try to forge a capability
package main

func main() {
    // Attempt 1: Guess capability ID
    fakeGPIO := Capability{ID: 0x1008}  // ← Won't work
    fakeGPIO.Set(HIGH)  // Error: Invalid capability
    
    // Attempt 2: Copy someone else's capability
    stolenCap := <-untrustedCanal  // ← Won't work
    // Capabilities are domain-local, can't be transferred without kernel
    
    // Only works:
    validGPIO := requestCapability("device:gpio:8", WRITE)
    validGPIO.Set(HIGH)  // ✓ Success
}
```

**Lesson:** Capabilities aren't just integers - they're kernel-managed permissions

---

## 5. Over-The-Air (OTA) Updates: Maintaining Deployed Devices

OTA will be required for any real-world IoT device, but traditional firmware updates are risky and often fail in practice. Canal's domain isolation allows for granular updates to specific domains without risking the entire device, making OTA updates safer and more reliable. This is a critical concept for students to understand, as it reflects the realities of managing IoT devices in the field where bugs are inevitable and physical access is often impossible.

### 5.1 The Problem with Traditional Firmware Updates

Breaking the world is bad in IoT, and breaking the device is the start of all that. Traditional firmware updates require flashing an entire new binary, which can fail due to power loss, network issues, or bugs in the update itself. If the update fails, the device is often bricked and requires physical intervention to recover.

**Current state of IoT updates:**

```
To update a smart thermostat:
1. Download 2MB firmware file
2. Put device in bootloader mode
3. Upload entire firmware (WiFi disconnects)
4. Wait 5 minutes
5. Hope it doesn't brick
6. If it fails → device is dead
```

**Why this fails in practice:**

- Users won't update (too complex)
- Power loss during update = bricked device
- All-or-nothing: can't rollback
- Downtime during update
- Requires physical access

### 5.2 Canal's Granular OTA

Imagining a flash layout where each domain is stored in its own partition, Canal can update just the sensor domain without touching the WiFi or logger domains. If the new sensor code has a bug, it can be rolled back without affecting the rest of the system.

We need to allow students develop the mental models of this type of systemic environment where the device is not a single monolithic blob of code, but a collection of isolated components that can be updated independently. This is a fundamental shift in how embedded systems are designed and taught, and it prepares students for the realities of modern IoT development where devices must be maintained and updated over their lifecycle.

And then, when viewed at the overall level of the network, many the student sees the interrelated groups of devices, each with their own domains and capabilities, all communicating through canals. This is the real world of IoT, and Canal gives students a hands-on experience with it.

**Canal updates individual domains:**

```
Flash Layout:
┌─────────────────────────────────────┐
│ 0x010000: Kernel (960KB) ← Rarely updates
├─────────────────────────────────────┤
│ 0x100000: Sensor v1.0 (64KB) ← Update this
├─────────────────────────────────────┤
│ 0x110000: WiFi v2.1 (128KB) ← Keep this
├─────────────────────────────────────┤
│ 0x130000: Logger v1.5 (64KB) ← Keep this
└─────────────────────────────────────┘

To fix sensor bug:
→ Download 64KB sensor domain
→ Write to 0x100000
→ Kernel restarts sensor domain
→ Everything else keeps running
```

**Update flow:**

```go
// OTA Updater Domain
package main

func main() {
    updater := requestCapability("service:ota")
    wifi := requestCapability("service:wifi")
    
    // Check for updates every hour
    for {
        manifest := updater.CheckServer("https://updates.mydevice.com")
        
        if manifest.SensorVersion > currentVersion {
            println("New sensor domain available!")
            
            // Download only the changed domain
            binary := updater.Download(manifest.SensorURL)
            
            // Verify signature
            if updater.Verify(binary, manifest.Signature) {
                // Stop old sensor domain
                kernel.StopDomain("sensor")
                
                // Flash new version
                updater.FlashPartition(0x100000, binary)
                
                // Start new sensor domain
                kernel.StartDomain("sensor")
                
                println("Sensor updated to", manifest.SensorVersion)
            }
        }
        
        time.Sleep(1 * time.Hour)
    }
}
```

### 5.3 Rollback and Safety

And when the code fails, and it will, how does the device recover? Canal can automatically roll back to the previous version if the new domain crashes repeatedly, ensuring that the device remains functional even if an update introduces a critical bug. This teaches students about the importance of resilience and recovery in IoT systems, as well as the practical challenges of maintaining devices in the field.

**Version tracking:**

```
Flash Partitions:
┌─────────────────────────────────────┐
│ 0x100000: Sensor v1.1 (active)
│ 0x110000: Sensor v1.0 (backup)
└─────────────────────────────────────┘

If v1.1 crashes 3 times in 5 minutes:
→ Kernel rolls back to v1.0
→ Marks v1.1 as bad
→ Reports to update server
```

**Student exercise:** Create an update that crashes, observe automatic rollback

```go
// Buggy sensor update
package main

func main() {
    println("[Sensor v1.1] Starting with intentional bug...")
    
    // Crash immediately
    panic("Oops!")
}
```

**Expected behavior:**

```
[Kernel] Sensor v1.1 starting...
[Sensor v1.1] Starting with intentional bug...
[Kernel] Sensor crashed (1/3)
[Kernel] Restarting...
[Sensor v1.1] Starting with intentional bug...
[Kernel] Sensor crashed (2/3)
[Kernel] Restarting...
[Sensor v1.1] Starting with intentional bug...
[Kernel] Sensor crashed (3/3)
[Kernel] Excessive crashes detected
[Kernel] Rolling back to v1.0...
[Sensor v1.0] Starting (stable version)
[Kernel] ✓ Rollback successful
```

### 5.4 Real-World Application

But while spacecraft are the shiny, future, students will be more comfortable with the familiar, and the familiar is the smart home. So we can show how this same approach applies to a common IoT device like a smart thermostat, where a bug in the sensor code could cause incorrect temperature readings, but with Canal's domain isolation and OTA updates, the issue can be fixed without bricking the device or requiring customer intervention.

**Smart Home Thermostat Example:**

Deployed: 10,000 thermostats in homes

**Bug found:** Temperature sensor reads 10°F too high in summer

**Traditional approach:**

- Email 10,000 customers
- Hope they update
- Result: 2% update rate, 98% have broken devices

**Canal approach:**

1. Push sensor domain update v1.2 to update server
2. Devices automatically download and install overnight
3. If any device has issues, auto-rollback to v1.1
4. Result: 100% update rate, zero customer intervention, zero downtime

**What students learn:**

- Real-world device management
- Importance of atomic updates
- Rollback strategies
- Version control for hardware

---

## 6. Educational Advantages

We cannot overemphasize how important it is for students to experience these concepts in a real embedded environment, rather than just in theory or in an emulator. If *touch grass* resets a student to real-world grounding, then *touch hardware* helps them understand the realities of embedded systems development. Canal's design allows students to see the immediate consequences of their code, understand how operating systems manage resources, and learn security principles through practice. This hands-on experience is invaluable for preparing students for careers in IoT development, where they will need to design secure, resilient systems that can operate in the real world.

### 6.1 Learning Operating System Concepts on Real Hardware

**Traditional OS course:**

- Theoretical lectures about processes, scheduling, memory protection
- Maybe write toy OS in emulator
- Never see it run on real hardware

**Canal approach:**

- Run actual OS on $10 ESP32 board
- See domains scheduled by FreeRTOS
- Watch MMU protect memory
- Experience crash isolation firsthand

**Concepts students experience:**

| Concept | How Canal Teaches It |
|---------|---------------------|
| **Processes** | Each domain is a process with own memory |
| **IPC** | Canals implement message passing |
| **Memory Protection** | MPU/MMU enforces domain boundaries |
| **Scheduling** | FreeRTOS round-robin across domains |
| **System Calls** | Capability requests go through kernel |
| **Device Drivers** | GPIO/I2C/SPI are kernel-managed |
| **Fault Handling** | Crash domain, kernel catches it |
| **Resource Management** | Capability system limits access |

### 6.2 Security by Design, Not as Afterthought

**Traditional embedded course:**

```
Week 1-8: Build working system
Week 9: "Now let's add security..."
         (Usually runs out of time)
```

This is not a good idea. Security is not a feature you can bolt on at the end - it's a fundamental aspect of how the system is designed and built. By teaching security concepts from day one through Canal's architecture, students learn to think about security as an integral part of their code, rather than an afterthought. They see how isolation, least privilege, and defense in depth work in practice, and they understand the
real-world consequences of security failures in embedded systems.

**Canal course:**

```
Day 1: Request capability to blink LED
       ↑ Security from the start
```

**Security concepts learned naturally:**

1. **Least Privilege** - Domain only gets capabilities it needs
2. **Defense in Depth** - Hardware + software isolation
3. **Principle of Complete Mediation** - All access through kernel
4. **Fail-Safe Defaults** - Deny unless explicitly granted
5. **Separation of Privilege** - Different domains for different tasks

**Example assignment progression:**

**Day 1:** Blink LED using GPIO capability

- Learn: Permission model

**Day 2:** Read sensor, log to SD card

- Learn: Multiple capabilities, resource sharing

**Day 3:** Add WiFi uploader domain

- Learn: Network isolation

**Day 4:** Try to make WiFi domain read sensor directly

- Learn: Fails! Must communicate through canal

**Day
 5:** Implement encrypted sensor data

- Learn: TLS domain holds keys, sensor can't see them

And at the end of this project, students will have a real IoT device running on actual hardware, with security principles built in from the ground up. They will understand how to design and build secure embedded systems, and they will be prepared for the challenges of real-world IoT development.

### 6.3 Modern Language for Embedded Systems

building canal and picoceci in **Go** (and then using **TinyGo** for the embedded domains) is a deliberate choice to make embedded systems programming more accessible and less error-prone for students. Go's memory safety, built-in concurrency, and modern tooling provide a much better learning experience than traditional C/C++ for embedded development. This allows students to focus on learning core concepts like isolation and capabilities without getting bogged down in low-level details that can lead to frustration and discouragement.

But the use of Go in a real-time context always makes the spectre of GC pauses and non-determinism loom over the project. This is where **TinyGo** comes in, allowing us to compile Go code down to bare-metal without a runtime, giving us the performance and predictability needed for embedded systems while still benefiting from Go's language features. This combination allows students to experience the best of both worlds: the safety and ease of Go with the performance and control of traditional embedded programming.

**Why Go/TinyGo matters:**

Traditional embedded: C/C++

- Manual memory management
- Easy to crash
- Hard to write concurrent code
- Steep learning curve

Canal: Go

- Garbage collected (per domain)
- Memory safe by default
- Built-in concurrency (goroutines, channels)
- Familiar to students

**Code comparison:**

**C approach:**

```c
// Traditional embedded C
uint8_t* buffer = malloc(1024);
if (buffer == NULL) {
    // Hope you remembered to check!
    return ERROR;
}

// Read sensor
read_sensor(buffer, 1024);

// Did you remember to free?
free(buffer);  // Forgot this? Memory leak!
```

**Go approach:**

```go
// Canal domain in Go
buffer := make([]byte, 1024)
// Automatically freed by GC when function returns

reading := readSensor()
// Type-safe, compiler checks types
```

**Concurrency comparison:**

**C approach:**

```c
// Manual threading and synchronization
pthread_t sensor_thread;
pthread_mutex_t data_lock;
pthread_create(&sensor_thread, NULL, read_sensor, NULL);
pthread_mutex_lock(&data_lock);
// Process data
pthread_mutex_unlock(&data_lock);
```

**Go approach:**

```go
// Canal concurrency
go readSensor()  // Goroutine
canal <- data    // Thread-safe by design
```

### 6.4 Incremental Learning Path

As a potential curriculum for a semester-long course, we can start with simple domains and gradually add complexity, teaching new concepts at each step while reinforcing previous lessons. This allows students to build confidence and understanding incrementally, rather than being overwhelmed by a complex system from the start.

We do not consider this curriculum to be all that useful, as with modern AI agent assited environments, development (and hence, **learning**) can be much faster and more iterative than a traditional semester-long course.

**Semester 1: IoT Fundamentals with Canal**

| Week | Topic | Lab |
|------|-------|-----|
| 1-2 | Domain basics, capabilities | LED blinker domain |
| 3-4 | GPIO and digital I/O | Button counter with debounce |
| 5-6 | I2C and sensors | Temperature logger |
| 7-8 | Inter-domain communication | Sensor → Display pipeline |
| 9-10 | File systems and storage | Data logger to SD card |
| 11-12 | WiFi and networking | IoT dashboard |
| 13-14 | OTA updates | Deploy and update device |
| 15 | Final project | Multi-domain IoT system |

**Semester 2: Advanced IoT with Canal**

| Week | Topic | Lab |
|------|-------|-----|
| 1-2 | Security and capabilities | Break domain isolation (safely) |
| 3-4 | TLS and encryption | Secure sensor data |
| 5-6 | Power management | Low-power weather station |
| 7-8 | Real-time constraints | Motor control domain |
| 9-10 | Multi-core systems | Parallel sensor processing |
| 11-12 | Production deployment | Field device management |
| 13-14 | Debugging and diagnostics | Remote troubleshooting |
| 15 | Capstone | Production IoT product |

---

## 7. Comparison with Other Embedded Systems

There many systems for these types of devices, but they all have their own trade-offs and limitations. By comparing Canal to other popular embedded platforms like Arduino, FreeRTOS, and Linux on embedded, we can highlight the unique advantages of Canal's architecture and design, and show why it is different for teaching modern IoT development.

### 7.1 Arduino

**Arduino:**

```cpp
void setup() {
    Serial.begin(9600);
    pinMode(LED, OUTPUT);
}

void loop() {
    // Everything in one loop
    readSensor();
    logData();
    checkWiFi();
    updateDisplay();
}
```

**Limitations:**

- No isolation (one crash = dead device)
- No memory protection
- Cooperative multitasking only
- Hard to update in field

**Canal equivalent:**

- Four separate domains
- Hardware-enforced isolation
- Preemptive multitasking
- Granular OTA updates

**When to use Arduino:** Quick prototypes, simple projects, learning basics

**When to use Canal:** Production IoT, security-critical, field-deployed devices

### 7.2 FreeRTOS

Yes, yes, we use FreeRTOS underneath Canal, but the way we use it is very different from how it's typically used in embedded development. FreeRTOS is often used as a simple scheduler for a single monolithic firmware, where all tasks share the same memory and can easily corrupt each other. Canal, on the other hand, uses FreeRTOS for scheduling but adds a layer of memory protection and capabilities to create true isolation between domains. This allows us to teach students about operating system concepts like processes, IPC, and memory protection in a real embedded environment, which is not possible with traditional FreeRTOS usage.

We also think the esp32s3 with it's memory protection unit (MPU) is a much better platform for teaching these concepts than the typical Cortex-M0/M3/M4 boards that FreeRTOS is often used on, which lack hardware memory protection and make it harder to demonstrate isolation and security principles. (Or which require a lot more software to introduce the concepts that are built into the hardware of the esp32s3.)

**FreeRTOS (traditional):**

```c
void sensor_task(void* params) {
    while(1) {
        read_sensor();
        vTaskDelay(1000);
    }
}

void logger_task(void* params) {
    while(1) {
        log_data();
        vTaskDelay(100);
    }
}

// Tasks share memory - can corrupt each other
```

**Canal on FreeRTOS:**

- Uses FreeRTOS for scheduling
- Adds memory protection (MPU/MMU)
- Adds capability system
- Written in Go, not C

**Relationship:** Canal is a capability-based OS layer on top of FreeRTOS

### 7.3 Linux on Embedded

We love Linux, we really do. BUT, Linux is a full operating system with all the features and complexity that comes with it, which makes it a great choice for powerful embedded devices like the Raspberry Pi, but it's often overkill for simple IoT sensors and actuators. Canal is designed to run on microcontrollers with limited resources, where a full Linux OS would be too heavy and complex. By comparing Canal to Linux on embedded, we can show students the trade-offs between a full OS and a lightweight, purpose-built OS like Canal, and help them understand when each is appropriate.

Imagining a series of Linux servers, managing a large network of small, yet mighty, MCU-based devices running Canal. The Linux servers can handle complex tasks like data aggregation, machine learning, and user interfaces, while the Canal devices focus on real-time sensing and actuation with strong isolation and security. This is a common architecture in IoT systems, where edge devices run lightweight OSes for local processing and security, while cloud or gateway servers run full OSes for heavy lifting.

**Embedded Linux:**

- Full OS features
- Large memory footprint (32MB+ RAM)
- Long boot time (10+ seconds)
- Complex build system (Yocto/Buildroot)
- Overkill for simple sensors

**Canal:**

- Kernel + domains < 1MB
- Boot time < 1 second
- Simple build (TinyGo + Make)
- Perfect for MCUs (512KB RAM)

**Use case distinction:**

- Linux: Raspberry Pi, multimedia, complex applications
- Canal: ESP32, sensors, battery-powered, real-time

---

## 8. Getting Started: Student Quick Start Guide

So how can we build this thing and get it running on real hardware? This section provides a step-by-step guide for students to set up their development environment, build Canal, and run their first domain on an ESP32-S3 board. We want to make it as easy as possible for students to get started and see results quickly, so they can stay motivated and engaged as they learn the concepts of operating systems, security, and IoT development through hands-on experience.

### 8.1 Hardware Requirements

**Minimum setup ($15):**

- ESP32-S3 DevKit ($10)
- USB-C cable ($3)
- Breadboard and LEDs ($2)

**Full IoT kit ($75):**

- ESP32-S3 DevKit ($10)
- BME280 temperature/humidity sensor ($5)
- SD card module ($3)
- OLED display ($8)
- Various sensors ($20)
- Breadboard, wires, components ($20)
- USB-C cable ($3)
- 5V power supply ($6)

**Optional advanced:**

- Logic analyzer (debugging)
- Oscilloscope (signal analysis)
- Multiple ESP32 boards (distributed systems)

### 8.2 Software Installation

Yes, we also have to give you all the magic incantations that you need to get the software installed and running. This includes installing TinyGo for compiling Go code to the ESP32, setting up the ESP-IDF for the underlying FreeRTOS kernel, and cloning the Canal repository to get the source code. We want to make this as straightforward as possible, with clear instructions and expected outputs, so that students can get up and running quickly and start experimenting with their own domains.

**Step 1: Install TinyGo**

```bash
# macOS
brew install tinygo

# Ubuntu/Debian
wget https://github.com/tinygo-org/tinygo/releases/download/v0.31.0/tinygo_0.31.0_amd64.deb
sudo dpkg -i tinygo_0.31.0_amd64.deb

# Windows
Download from tinygo.org/getting-started/install
```

**Step 2: Install ESP-IDF (for ESP32 targets)**

```bash
git clone --recursive https://github.com/espressif/esp-idf.git
cd esp-idf
./install.sh esp32s3
. ./export.sh
```

**Step 3: Clone Canal**

```bash
git clone https://github.com/kristofer/Canal.git
cd Canal
./scripts/setup.sh esp32s3
```

**Step 4: Build and Flash**

```bash
make TARGET=esp32s3
make flash PORT=/dev/ttyUSB0
make monitor
```

**Expected output:**

```
=== Canal OS Boot ===
[Kernel] Initializing...
[Kernel] Loading domains...
[LED] Domain 1 starting...
[Logger] Domain 2 starting...
=== System Ready ===
```

### 8.3 First Lab Exercise: LED Blinker Domain

**Objective:** Create an isolated domain that blinks an LED

**File:** `domains/my-led/main.go`

```go
//go:build tinygo

package main

import (
    "time"
    "machine"
    "runtime"
)

func main() {
    println("[MyLED] Domain starting...")
    
    // Request GPIO capability
    ledCap, err := runtime.RequestCap("device:gpio:8", runtime.RightWrite)
    if err != nil {
        println("[MyLED] Failed to get GPIO:", err)
        return
    }
    
    println("[MyLED] Got GPIO capability - blinking!")
    
    // Blink forever
    count := 0
    for {
        runtime.CapSend(ledCap, GPIOCommand{Op: GPIO_HIGH})
        time.Sleep(500 * time.Millisecond)
        
        runtime.CapSend(ledCap, GPIOCommand{Op: GPIO_LOW})
        time.Sleep(500 * time.Millisecond)
        
        count++
        if count % 10 == 0 {
            println("[MyLED] Blinks:", count)
        }
    }
}

type GPIOCommand struct {
    Op uint8
}

const (
    GPIO_HIGH = 1
    GPIO_LOW  = 2
)
```

**Build and test:**

```bash
# Add to Makefile
my-led:
 $(TINYGO) build $(TINYGO_FLAGS) \
  -o $(OUT_DIR)/my-led.elf \
  domains/my-led/main.go

# Build
make my-led

# Flash
make flash

# Observe
make monitor
```

**Questions for students:**

1. What happens if you request GPIO 9 instead of 8?
2. What happens if you request RightRead instead of RightWrite?
3. Can you crash this domain? What happens to other domains?
4. How would you add a second LED blinking at a different rate?

### 8.4 Second Lab: Sensor to Logger Pipeline

**Objective:** Create two domains that communicate through a canal

**Sensor domain:**

```go
// domains/temp-sensor/main.go
package main

func main() {
    i2c := runtime.RequestCap("device:i2c:0", runtime.RightReadWrite)
    loggerCanal := runtime.RequestCap("service:logger", runtime.RightWrite)
    
    bme280 := initBME280(i2c)
    
    for {
        temp := bme280.ReadTemperature()
        
        // Send to logger through canal
        runtime.CanalSend(loggerCanal, SensorReading{
            Timestamp: time.Now().Unix(),
            Temperature: temp,
        })
        
        time.Sleep(60 * time.Second)
    }
}
```

**Logger domain:**

```go
// domains/data-logger/main.go
package main

func main() {
    sdcard := runtime.RequestCap("service:filesystem", runtime.RightWrite)
    inputCanal := runtime.ExposeCap("service:logger", runtime.RightWrite)
    
    file := sdcard.Open("/data/temps.csv", CREATE|APPEND)
    
    for {
        reading := runtime.CanalRecv(inputCanal)
        
        line := fmt.Sprintf("%d,%f\n", reading.Timestamp, reading.Temperature)
        file.Write([]byte(line))
        file.Sync()
    }
}
```

**Learning objectives:**

- Inter-domain communication
- Structured data types
- File system operations
- Producer-consumer pattern

---

## 9. Research and Future Directions

### 9.1 Current Status (May 2026)

**Implemented:**

- ✅ FreeRTOS substrate on ESP32-S3
- ✅ Domain spawning and isolation
- ✅ Basic capability system
- ✅ GPIO capabilities
- ✅ LED blinker as isolated domain

**In Progress:**

- 🚧 ELF loader for dynamic domain loading
- 🚧 MMU configuration for ESP32-S3
- 🚧 Full capability table implementation
- 🚧 Inter-domain canals

**Planned:**

- 📋 System service domains (WiFi, TLS, Filesystem)
- 📋 OTA update mechanism
- 📋 Multi-core support
- 📋 Power management domains
- 📋 Formal verification of kernel

### 9.2 Open Research Questions

**For student projects:**

1. **Capability Delegation Chains**
   - How should domains delegate subsets of their capabilities?
   - Can we track delegation chains for auditing?

2. **Real-Time Scheduling**
   - How to guarantee timing for sensor sampling?
   - Mixed criticality domains (hard/soft real-time)

3. **Power-Aware Domain Management**
   - Sleep unused domains to save power
   - Wake domains based on events

4. **Distributed Canal Systems**
   - Multiple ESP32s communicating
   - Capability transfer across devices

5. **Formal Verification**
   - Prove kernel correctness
   - Verify isolation properties

### 9.3 Contributing to Canal

**Ways students can contribute:**

**Beginner:**

- Add new example domains
- Write tutorials and documentation
- Test on different hardware platforms
- Report bugs and issues

**Intermediate:**

- Implement system service domains
- Add sensor drivers
- Create testing frameworks
- Port to new MCUs

**Advanced:**

- Kernel improvements
- Security analysis
- Performance optimization
- Formal methods

**Process:**

1. Fork <https://github.com/kristofer/Canal>
2. Create feature branch
3. Implement and test
4. Submit pull request
5. Discuss with community

---

## 10. Conclusion: Why Canal Matters for IoT Education

The Internet of Things is growing exponentially, but traditional embedded development practices don't scale. Teaching students to write monolithic firmware prepares them for yesterday's problems, not tomorrow's challenges.

**Canal teaches the right concepts:**

- **Isolation** - Failures should be contained
- **Least privilege** - Grant minimal permissions
- **Explicit communication** - Message passing over shared memory
- **Resilience** - Systems should recover from errors
- **Maintainability** - Update deployed devices safely

**These aren't just embedded systems concepts** - they're the same principles behind:

- Microservices architectures
- Container orchestration (Kubernetes)
- Cloud security models
- Operating system design

**By learning Canal, students gain:**

1. **Practical OS knowledge** - Run real OS on $10 hardware
2. **Security mindset** - Design secure systems from day one
3. **Modern development** - Go instead of C, but for embedded
4. **Industry-relevant skills** - Capability systems, domain isolation, OTA updates
5. **Portfolio projects** - Build production-ready IoT devices

**The bottom line:**
Canal makes embedded systems development accessible, safe, and educational. Students can experiment fearlessly, recover from mistakes automatically, and build systems that work reliably in the real world.

IoT isn't going away. The next generation of developers needs to build it right.

**Learn more:**

- **Repository:** <https://github.com/kristofer/Canal>
- **Documentation:** See `docs/` folder
- **Examples:** See `examples/` folder
- **Community:** Open issues, discussions, and pull requests welcome

**For educators:**
If you're interested in using Canal in your IoT or embedded systems course, contact:

- Kristofer Younger - ZipCodeWilmington
- GitHub: @kristofer

---

## Appendix A: Complete Domain Example

**Smart Weather Station System**

**Architecture:**

```
┌──────────────┐
│   Kernel     │
└──────┬───────┘
       │
   ┌───┴────┬─────────┬──────────┬────────┐
   │        │         │          │        │
┌──▼──┐ ┌──▼──┐ ┌────▼───┐ ┌────▼───┐ ┌──▼───┐
│ BME │ │OLED │ │ Logger │ │  WiFi  │ │ OTA  │
│ 280 │ │ UI  │ │ (SD)   │ │Upload  │ │Update│
└─────┘ └─────┘ └────────┘ └────────┘ └──────┘
```

**Domain 1: BME280 Sensor**

```go
package main

import (
    "time"
    "runtime"
)

type Reading struct {
    Timestamp   uint64
    Temperature float32
    Humidity    float32
    Pressure    float32
}

func main() {
    i2c := runtime.RequestCap("device:i2c:0", runtime.RightReadWrite)
    uiCanal := runtime.RequestCap("canal:ui", runtime.RightWrite)
    loggerCanal := runtime.RequestCap("canal:logger", runtime.RightWrite)
    
    sensor := initBME280(i2c, 0x76)
    
    for {
        reading := Reading{
            Timestamp:   time.Now().Unix(),
            Temperature: sensor.ReadTemperature(),
            Humidity:    sensor.ReadHumidity(),
            Pressure:    sensor.ReadPressure(),
        }
        
        // Send to UI for display
        runtime.CanalSend(uiCanal, reading)
        
        // Send to logger for storage
        runtime.CanalSend(loggerCanal, reading)
        
        time.Sleep(5 * time.Second)
    }
}
```

**Domain 2: OLED Display**

```go
package main

func main() {
    i2c := runtime.RequestCap("device:i2c:0", runtime.RightReadWrite)
    dataCanal := runtime.ExposeCap("canal:ui", runtime.RightRead)
    
    display := initSSD1306(i2c, 0x3C)
    display.Clear()
    
    for {
        reading := runtime.CanalRecv(dataCanal).(Reading)
        
        display.SetCursor(0, 0)
        display.Printf("Temp: %.1f C", reading.Temperature)
        display.SetCursor(0, 16)
        display.Printf("Humidity: %.1f%%", reading.Humidity)
        display.SetCursor(0, 32)
        display.Printf("Pressure: %.0f hPa", reading.Pressure)
        display.Update()
    }
}
```

**Domain 3: Data Logger**

```go
package main

func main() {
    sdcard := runtime.RequestCap("service:filesystem", runtime.RightWrite)
    dataCanal := runtime.ExposeCap("canal:logger", runtime.RightRead)
    uploadCanal := runtime.RequestCap("canal:upload", runtime.RightWrite)
    
    file := sdcard.Open("/weather/data.csv", CREATE|APPEND)
    defer file.Close()
    
    for {
        reading := runtime.CanalRecv(dataCanal).(Reading)
        
        // Write to SD card
        line := fmt.Sprintf("%d,%.2f,%.2f,%.2f\n",
            reading.Timestamp,
            reading.Temperature,
            reading.Humidity,
            reading.Pressure)
        file.Write([]byte(line))
        file.Sync()
        
        // Also send to upload domain
        runtime.CanalSend(uploadCanal, reading)
    }
}
```

**Domain 4: WiFi Uploader**

```go
package main

func main() {
    wifi := runtime.RequestCap("service:wifi", runtime.RightReadWrite)
    http := runtime.RequestCap("service:http", runtime.RightWrite)
    dataCanal := runtime.ExposeCap("canal:upload", runtime.RightRead)
    
    // Connect to WiFi
    wifi.Connect("MyNetwork", "password", 30000)
    
    // Upload readings
    buffer := []Reading{}
    ticker := time.NewTicker(10 * time.Minute)
    
    for {
        select {
        case reading := <-dataCanal:
            buffer = append(buffer, reading.(Reading))
            
        case <-ticker.C:
            if len(buffer) > 0 {
                json := marshalReadings(buffer)
                err := http.Post("https://api.weather.com/data", json)
                
                if err == nil {
                    println("[Upload] Uploaded", len(buffer), "readings")
                    buffer = buffer[:0]  // Clear buffer
                } else {
                    println("[Upload] Failed, will retry:", err)
                }
            }
        }
    }
}
```

**Domain 5: OTA Updater**

```go
package main

func main() {
    updater := runtime.RequestCap("service:ota", runtime.RightReadWrite)
    http := runtime.RequestCap("service:http", runtime.RightRead)
    
    // Check for updates every 6 hours
    for {
        manifest := http.Get("https://updates.weather.com/manifest.json")
        
        if manifest.SensorVersion > VERSION_SENSOR {
            binary := http.Get(manifest.SensorURL)
            
            if updater.Verify(binary, manifest.Signature) {
                updater.UpdateDomain("sensor", binary)
                println("[OTA] Sensor updated to", manifest.SensorVersion)
            }
        }
        
        time.Sleep(6 * time.Hour)
    }
}
```

**Building the system:**

```bash
make TARGET=esp32s3 sensor display logger uploader ota
make flash
```

**Result:** Complete weather station that:

- Reads sensors every 5 seconds
- Displays on OLED immediately
- Logs to SD card continuously
- Uploads to cloud every 10 minutes
- Updates itself when new firmware available
- Recovers from any single component failure

---

## Appendix B: Glossary

**Canal** - A typed communication channel between domains, based on Go channels

**Capability** - An unforgeable token granting specific permissions to a resource

**Domain** - An isolated program running on the MCU with its own memory and capabilities

**ELF** - Executable and Linkable Format, standard binary format for programs

**FreeRTOS** - Real-time operating system that provides task scheduling

**GPIO** - General Purpose Input/Output, pins for controlling hardware

**I2C** - Inter-Integrated Circuit, protocol for communicating with sensors

**Kernel** - Minimal trusted substrate managing domains and capabilities

**MCU** - Microcontroller Unit, the chip running the code (ESP32, etc.)

**MMU** - Memory Management Unit, hardware that enforces memory isolation

**MPU** - Memory Protection Unit, simpler version of MMU

**OTA** - Over-The-Air, updating device firmware wirelessly

**Partition** - Section of flash memory holding code or data

**TinyGo** - Go compiler for microcontrollers

---

**End of Whitepaper**

*This whitepaper describes Canal v0.1.0-alpha (May 2026)*  
*For latest information, see <https://github.com/kristofer/Canal>*  
*© 2026 Kristofer Younger, ZipCode Wilmington - Released under MIT License*

---

**Document Metadata:**

- **Version:** 1.0
- **Date:** May 3, 2026
- **Target Audience:** Undergraduate/Graduate CS students, Embedded Systems students, IoT developers
- **Prerequisites:** Basic programming, understanding of hardware concepts
- **Reading Time:** ~45 minutes
- **Suggested Use:** Course textbook supplement, workshop material, self-study guide

```

So, if _Canal_ works for some smaller projects and systems, then it can work for larger ones too. The same principles of isolation, capabilities, and modularity apply regardless of scale. By building a solid foundation with Canal on simple devices, students can gain the confidence and skills to tackle more complex systems in the future, whether that's a smart home device, an industrial sensor network, or even a spacecraft. 

Spacecraft, larger and more complex than the ones we have now like Artemis, will need well-designed, resilient, evolutionary systems to manage the myriad of systems and sub-systems to function. Environmental Life Support, Power, Communications, Sensors, Food Production, Command-and-Control systems all need to work seamlessly together. 

Networks of many, many, smaller, simpler devices (like MCUs) running Canal could be used to manage these complex systems, with each domain responsible for a specific function and communicating securely with other domains as needed.

All of this needs to be designed with security and reliability in mind, as failures can have catastrophic consequences. By learning these principles through Canal, students will be better prepared to design and build the next generation of space systems that can operate safely and effectively in the harsh environment of space.
