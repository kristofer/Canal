//go:build tinygo && esp32s3

package main

import (
	"time"
)

// WiFi service domain - handles all WiFi operations
// Runs with elevated privileges to access WiFi hardware

func main() {
	println("[WiFi] Domain starting (ESP32-S3)")
	ticker := 0
	for {
		ticker++
		if ticker%10 == 0 {
			println("[WiFi] alive")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func ipString(ip [4]byte) string {
	// Simple IP to string (would use proper formatting)
	return "192.168.1.123" // Placeholder
}
