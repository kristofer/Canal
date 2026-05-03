// domains/logger/main.go
//go:build tinygo

package main

import (
	"time"
	"unsafe"
)

// domain_entry is called by the kernel's ELF loader via xTaskCreate.
// FreeRTOS passes a single void* pvParameters argument which we ignore here.
//
//export domain_entry
func domain_entry(_ unsafe.Pointer) {
	println("[Logger] Domain starting from flash")
	runLogger()
}

func main() {
	println("[Logger] Standalone start")
	runLogger()
}

func runLogger() {
	println("[Logger] Starting...")

	ticker := 0
	for {
		ticker++
		if ticker%10 == 0 {
			println("[Logger] alive, tick:", ticker)
		}
		time.Sleep(10 * time.Second)
	}
}
