//go:build tinygo && esp32s3

package main

import (
	"machine"
	"time"
)

func main() {
	println("[LED] Starting WS2812 on GPIO 48")

	err := machine.SPI0.Configure(machine.SPIConfig{
		Frequency: 3_200_000,
		Mode:      0,
		SCK:       machine.NoPin,
		SDO:       machine.GPIO48,
		SDI:       machine.NoPin,
	})
	if err != nil {
		println("[LED] SPI error:", err.Error())
		return
	}

	println("[LED] Cycling colors")

	colors := [][3]uint8{
		{255, 0, 0},
		{0, 255, 0},
		{0, 0, 255},
		{255, 80, 0},
		{80, 0, 255},
		{0, 0, 0},
	}
	i := 0
	for {
		c := colors[i%len(colors)]
		ws2812Write(c[0], c[1], c[2])
		i++
		time.Sleep(600 * time.Millisecond)
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
