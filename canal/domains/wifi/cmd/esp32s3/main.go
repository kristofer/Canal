//go:build tinygo && esp32s3

package main

import "time"

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
