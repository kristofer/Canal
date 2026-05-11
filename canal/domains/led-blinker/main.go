//go:build tinygo && esp32s3

package main

import (
	"machine"
	"unsafe"
)

var domainMode bool

// domain_entry is called by the kernel's ELF loader via xTaskCreate.
// It receives the FreeRTOS task parameter pointer allocated by the kernel.
// The first field is DomainID.
//
//export domain_entry
func domain_entry(param unsafe.Pointer) {
	domainMode = true
	_ = param
	runLED()
	for {
		vTaskDelay(portMAX_DELAY)
	}
}

func main() {
	runLED()
	for {
		vTaskDelay(portMAX_DELAY)
	}
}

func runLED() {
	err := machine.SPI0.Configure(machine.SPIConfig{
		Frequency: 3_200_000,
		Mode:      0,
		SCK:       machine.NoPin,
		SDO:       machine.GPIO48,
		SDI:       machine.NoPin,
	})
	if err != nil {
		_ = err
		return
	}

	// Alternate blue/white twice for a blinking effect, then orange, violet, off.
	colors := [8][3]uint8{
		{0, 0, 255},    // blue
		{253, 218, 13}, // yellow (blink 1)
		{0, 0, 255},    // blue
		{253, 218, 13}, // yellow (blink 2)
		{0, 0, 255},    // blue
		{255, 80, 0},   // orange
		{80, 0, 255},   // violet
		{0, 0, 0},      // off
	}
	i := 0
	for {
		c := colors[i%len(colors)]
		ws2812Write(c[0], c[1], c[2])
		i++
		vTaskDelay(750) // 750 ms between color changes
	}
}

func ws2812Write(r, g, b uint8) {
	var buf [14]byte
	ws2812EncodeByte(g, buf[0:4])
	ws2812EncodeByte(r, buf[4:8])
	ws2812EncodeByte(b, buf[8:12])
	machine.SPI0.Tx(buf[:], nil)
}

func ws2812EncodeByte(b uint8, out []byte) {
	for i := 0; i < 4; i++ {
		hi := (b >> (7 - uint(i)*2)) & 1
		lo := (b >> (6 - uint(i)*2)) & 1
		var v byte
		if hi != 0 {
			v |= 0xE0
		} else {
			v |= 0x80
		}
		if lo != 0 {
			v |= 0x0E
		} else {
			v |= 0x08
		}
		out[i] = v
	}
}
