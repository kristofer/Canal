// kernel/boot_esp32s3.go
//go:build tinygo && esp32s3

package kernel

import (
	"kernel/hal"
	"machine"
)

var memoryProtection hal.MemoryProtection

func millis() uint32 {
	return xTaskGetTickCount()
}

//export app_main
func app_main() {
	machine.InitSerial()
	println("\n=== Canal ESP32-S3 ===")
	println("Boot time:", millis(), "ms")

	memoryProtection = hal.NewMemoryProtection()
	if err := memoryProtection.Init(); err != nil {
		panic("MMU init failed")
	}
	println("MMU initialized")

	InitCapTable()
	println("Capability table ready")

	InitDomainTable()
	println("Domain table ready")

	InitSyscall()
	println("Syscall handler ready")

	println("Loading domains from flash...")
	bootDomains()

	println("=== Boot Complete ===")
	println("Free heap:", xPortGetFreeHeapSize()/1024, "KB")

	for {
		vTaskDelay(1000)
		printStats()
	}
}

// ── Domain boot ──────────────────────────────────────────────────────────────

// bootDomains tries to load each domain from its flash partition via the ELF
// loader. If a partition doesn't contain a valid ELF (e.g. during development
// before domains are flashed), it falls back to the in-kernel goroutine entry.
func bootDomains() {
	type domainDef struct {
		name     string
		heapSize uint32
		fallback func()
		priority uint8
	}

	domains := []domainDef{
		{"led", HeapTiny, ledDomainEntry, 2},
		{"wifi", HeapMedium, wifiDomainEntry, 2},
		{"logger", HeapSmall, nil, 2},
	}

	for _, def := range domains {
		id, errno := SpawnDomainFromFlash(def.name, def.priority)
		if errno == ErrNone {
			println("  [flash]", def.name, "id:", id)
			continue
		}
		// Flash partition missing or invalid ELF — fall back to goroutine.
		if def.fallback != nil {
			id, errno = DomainSpawn(def.name, def.heapSize, def.fallback, def.priority)
			if errno == ErrNone {
				println("  [goroutine]", def.name, "id:", id)
			}
		} else {
			println("  [skip]", def.name, "(no fallback)")
		}
	}
}

// ── Domain entry points ───────────────────────────────────────────────────────
// Fallback goroutine roots used when flash partitions are not yet programmed.

func ledDomainEntry() {
	println("[LED Domain] Starting WS2812 on GPIO 48")

	err := machine.SPI0.Configure(machine.SPIConfig{
		Frequency: 3_200_000, // 80 MHz / 25 = 3.2 MHz → 4 SPI bits = 1.25 µs WS2812 bit
		Mode:      0,
		SCK:       machine.NoPin,
		SDO:       machine.GPIO48,
		SDI:       machine.NoPin,
	})
	if err != nil {
		println("[LED Domain] SPI configure error:", err.Error())
		return
	}

	println("[LED Domain] Cycling colors")

	// R, G, B — ws2812Write sends GRB on the wire
	colors := [][3]uint8{
		{255, 0, 0},  // red
		{0, 255, 0},  // green
		{255, 0, 0},  // red
		{0, 0, 255},  // blue
		{255, 80, 0}, // orange
		{80, 0, 255}, // violet
		{0, 0, 0},    // off
	}
	i := 0
	for {
		c := colors[i%len(colors)]
		ws2812Write(c[0], c[1], c[2])
		i++
		vTaskDelay(600)
	}
}

func wifiDomainEntry() {
	println("[WiFi Domain] Starting")
	ticker := 0
	for {
		ticker++
		if ticker%10 == 0 {
			println("[WiFi Domain] alive, tick:", ticker)
		}
		vTaskDelay(500)
	}
}

// ── WS2812 SPI driver ─────────────────────────────────────────────────────────
// Drives a single WS2812 via SPI MOSI at 3.2 MHz.
// Each WS2812 bit → 4 SPI bits: 1 = 1110 (0xE nibble), 0 = 1000 (0x8 nibble).
// Two WS2812 bits pack per SPI byte → 8 WS2812 bits = 4 SPI bytes.
// Protocol order: GRB.

func ws2812Write(r, g, b uint8) {
	var buf [14]byte // 12 bytes GRB + 2 zero bytes to begin reset pulse
	ws2812EncodeByte(g, buf[0:4])
	ws2812EncodeByte(r, buf[4:8])
	ws2812EncodeByte(b, buf[8:12])
	// buf[12], buf[13] stay 0x00; vTaskDelay below finishes the >50 µs reset
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

// ── Kernel stats ──────────────────────────────────────────────────────────────

func printStats() {
	println("\n--- System Stats ---")
	println("Uptime:", millis()/1000, "seconds")
	println("Free heap:", xPortGetFreeHeapSize()/1024, "KB")
	for i := DomainID(1); i < maxDomains; i++ {
		if domainTable[i].State != DomainStateInvalid {
			name := string(trimNull(domainTable[i].Name[:]))
			println("  domain", i, name, "caps:", domainTable[i].CapCount)
		}
	}
}

func trimNull(b []byte) []byte {
	for i, c := range b {
		if c == 0 {
			return b[:i]
		}
	}
	return b
}
