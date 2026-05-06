//go:build tinygo && esp32s3

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
	println("[WiFi] Domain", domainID, "starting from flash")
	runWiFi()
}

func main() {
	println("[WiFi] Standalone start")
	runWiFi()
}

func runWiFi() {
	println("[WiFi] Domain running (ESP32-S3)")
	if domainMode {
		println("[WiFi] domain mode: parking task (no WiFi stack yet)")
		// Yield via FreeRTOS — no Go scheduler required, no CPU burn.
		for {
			vTaskDelay(portMAX_DELAY)
		}
	}

	ticker := 0
	for {
		ticker++
		if ticker%10 == 0 {
			println("[WiFi] alive, tick:", ticker)
		}
		vTaskDelay(500) // 500 ms
	}
}
