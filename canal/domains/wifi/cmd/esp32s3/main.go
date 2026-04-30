//go:build tinygo && esp32s3

package main

import (
	"time"
	"unsafe"
)

// domain_entry is called by the kernel's ELF loader via xTaskCreate.
//
//export domain_entry
func domain_entry(domainID uint16, syscallQ, replyQ unsafe.Pointer) {
	println("[WiFi] Domain", domainID, "starting from flash")
	runWiFi()
}

func main() {
	println("[WiFi] Standalone start")
	runWiFi()
}

func runWiFi() {
	println("[WiFi] Domain running (ESP32-S3)")
	ticker := 0
	for {
		ticker++
		if ticker%10 == 0 {
			println("[WiFi] alive, tick:", ticker)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
