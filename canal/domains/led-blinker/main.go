//go:build tinygo && esp32s3

package main

import (
    "machine"
    "time"
)

// Simple LED blinker domain
// Shows that not all domains need complex capabilities

func main() {
    println("[LED] Starting...")

    // ESP32-S3 built-in LED is usually GPIO 2
    led := machine.GPIO2
    led.Configure(machine.PinConfig{Mode: machine.PinOutput})

    println("[LED] Blinking on GPIO 2")

    state := false
    for {
        if state {
            led.High()
        } else {
            led.Low()
        }

        state = !state
        time.Sleep(500 * time.Millisecond)
    }
}
