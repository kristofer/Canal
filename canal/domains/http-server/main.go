//go:build tinygo && esp32s3

package main

import (
	"time"
)

// HTTP server domain - serves web pages
// Uses WiFi capability for networking

func main() {
	println("[HTTP] Domain starting (ESP32-S3)")
	ticker := 0
	for {
		ticker++
		if ticker%10 == 0 {
			println("[HTTP] alive")
		}
		time.Sleep(500 * time.Millisecond)
	}
}
