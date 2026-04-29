//go:build tinygo

package kernel

import (
    "device/arm"
    "machine"
)

// Kernel main entry point
//export main
func main() {
    // Initialize hardware
    machine.InitSerial()

    debugPrintln("Canal Kernel Starting...")

    // Initialize MPU
    InitMPU()
    debugPrintln("MPU initialized")

    // Initialize capability table
    InitCapTable()
    debugPrintln("Capability table initialized")

    // Initialize domain table
    InitDomainTable()
    debugPrintln("Domain table initialized")

    // Initialize syscall system
    InitSyscall()
    debugPrintln("Syscall system initialized")

    // Create syscall handler task (highest priority)
    var syscallTask TaskHandle_t
    xTaskCreate(
        unsafe.Pointer(&SyscallHandler),
        cstring("syscall"),
        256,
        nil,
        configMAX_PRIORITIES-1,
        &syscallTask,
    )
    debugPrintln("Syscall handler created")

    // Spawn initial domains
    spawnInitialDomains()

    debugPrintln("Starting FreeRTOS scheduler...")

    // Start FreeRTOS scheduler (never returns)
    vTaskStartScheduler()

    // Should never reach here
    for {
        arm.Asm("wfi")
    }
}

// Spawn initial system domains
func spawnInitialDomains() {
    // In a real system, these would be loaded from flash
    // For now, we'll create them with dummy addresses

    // GPIO service domain
    gpioID, err := DomainSpawn(
        "gpio-svc",
        0x08000000, 4096,  // Code at flash start
        0x20000000, 2048,  // Data in RAM
        nil,               // Entry point (would be actual function)
        2,                 // Priority
    )
    if err == ErrNone {
        debugPrintf("GPIO service spawned (ID: %d)\n", gpioID)
    }

    // UART service domain
    uartID, err := DomainSpawn(
        "uart-svc",
        0x08001000, 4096,
        0x20000800, 2048,
        nil,
        2,
    )
    if err == ErrNone {
        debugPrintf("UART service spawned (ID: %d)\n", uartID)
    }
}

// Debug output functions
func debugWrite(data []byte) {
    for _, b := range data {
        machine.Serial.WriteByte(b)
    }
}

func debugPrintln(s string) {
    debugWrite([]byte(s))
    debugWrite([]byte{'\r', '\n'})
}

func debugPrintf(format string, args ...interface{}) {
    // Simplified printf (in reality would use proper formatting)
    debugWrite([]byte(format))
}
