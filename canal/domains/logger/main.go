// domains/logger/main.go
//go:build tinygo

package main

import (
	"unsafe"
)

var domainMode bool

// domain_entry is called by the kernel's ELF loader via xTaskCreate.
//
//export domain_entry
func domain_entry(param unsafe.Pointer) {
	domainMode = true
	var domainID uint16
	if param != nil {
		domainID = *(*uint16)(param)
	}
	println("[Logger] Domain", domainID, "starting from flash")
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
		// vTaskDelay yields to the FreeRTOS scheduler — safe in domain mode.
		// 10 000 ticks = 10 s with the default 1 ms/tick config.
		vTaskDelay(10000)
	}
}
