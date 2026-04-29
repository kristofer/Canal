// kernel/boot_esp32s3.go
//go:build tinygo && esp32s3

package kernel

import (
    "machine"
    "kernel/hal"
)

//export app_main
func app_main() {
    // Called by ESP-IDF after initialization

    machine.InitSerial()
    println("\n=== ChannelOS ESP32-S3 ===")
    println("Boot time:", millis(), "ms")

    // Initialize MMU
    memoryProtection = hal.NewMemoryProtection()
    err := memoryProtection.Init()
    if err != nil {
        panic("MMU init failed")
    }
    println("MMU initialized")

    // Initialize capability table
    InitCapTable()
    println("Capability table ready")

    // Initialize domain table
    InitDomainTable()
    println("Domain table ready")

    // Initialize syscall handler
    InitSyscall()
    println("Syscall handler ready")

    // Create syscall handler task
    var syscallTask TaskHandle_t
    xTaskCreate(
        unsafe.Pointer(&SyscallHandler),
        cstring("syscall"),
        4096, // Larger stack for ESP32
        nil,
        configMAX_PRIORITIES-1,
        &syscallTask,
    )
    println("Syscall task created")

    // Load domains from flash partitions
    println("Loading domains...")

    // WiFi service domain
    wifiID := loadDomainFromFlash("wifi", 0x100000)
    println("  WiFi domain:", wifiID)

    // HTTP server domain
    httpID := loadDomainFromFlash("http-server", 0x180000)
    println("  HTTP domain:", httpID)

    // LED blinker domain
    ledID := loadDomainFromFlash("led-blinker", 0x200000)
    println("  LED domain:", ledID)

    println("=== Boot Complete ===")
    println("Free heap:", xPortGetFreeHeapSize()/1024, "KB")

    // FreeRTOS scheduler is already running on ESP-IDF
    // We just created tasks, they'll be scheduled automatically

    // Idle loop
    for {
        vTaskDelay(1000)
        printStats()
    }
}

func loadDomainFromFlash(name string, flashOffset uint32) DomainID {
    // Read ELF from flash partition
    // Parse sections
    // Allocate memory
    // Configure MMU
    // Create task

    // Simplified - would parse actual ELF
    return 0
}

func printStats() {
    println("\n--- System Stats ---")
    println("Uptime:", millis()/1000, "seconds")
    println("Free heap:", xPortGetFreeHeapSize()/1024, "KB")

    // Print domain stats
    for i := DomainID(1); i < maxDomains; i++ {
        if domainTable[i].State != DomainStateInvalid {
            name := string(trimNull(domainTable[i].Name[:]))
            println("Domain", i, name, "- caps:", domainTable[i].CapCount)
        }
    }
}

func trimNull(b []byte) []byte {
    for i, c := range b {
        if c == 0 {
            return b[:i]
        }
    }
    return b
}
