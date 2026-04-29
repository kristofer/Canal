// kernel/boot_esp32s3.go
//go:build tinygo && esp32s3

package kernel

import (
	"kernel/hal"
	"machine"
	"unsafe"
)

var memoryProtection hal.MemoryProtection

func millis() uint32 {
	return xTaskGetTickCount()
}

//export app_main
func app_main() {
	// Called by ESP-IDF after initialization

	machine.InitSerial()
	println("\n=== Canal ESP32-S3 ===")
	println("Boot time:", millis(), "ms")

	// Initialize MMU
	memoryProtection = hal.NewMemoryProtection()
	err := memoryProtection.Init()
	if err != nil {
		panic("MMU init failed")
	}
	println("MMU initialized")

	// Initialize capability table
	InitCapTable()
	println("Capability table ready")

	// Initialize domain table
	InitDomainTable()
	println("Domain table ready")

	// Initialize syscall handler
	InitSyscall()
	println("Syscall handler ready")

	// Create syscall handler task
	var syscallTask TaskHandle_t
	xTaskCreate(
		nil,
		cstring("syscall"),
		4096, // Larger stack for ESP32
		nil,
		configMAX_PRIORITIES-1,
		&syscallTask,
	)
	println("Syscall task created")

	// Start domains as goroutines (flash-loading is future work)
	println("Starting domains...")
	startDomainRuntime("wifi")
	startDomainRuntime("http-server")
	startDomainRuntime("led-blinker")

	println("=== Boot Complete ===")
	println("Free heap:", xPortGetFreeHeapSize()/1024, "KB")

	// FreeRTOS scheduler is already running on ESP-IDF
	// We just created tasks, they'll be scheduled automatically

	// Idle loop
	for {
		vTaskDelay(1000)
		printStats()
	}
}

func loadDomainFromFlash(name string, flashOffset uint32) DomainID {
	const (
		espImageMagic = 0xE9
		flashMapBase  = 0x42000000
	)

	type espImageHeader struct {
		Magic          uint8
		SegmentCount   uint8
		FlashMode      uint8
		FlashSizeFreq  uint8
		EntryAddr      uint32
		WPpin          uint8
		DriveSettings  [3]uint8
		ChipID         uint16
		MinChipRev     uint8
		MinChipRevFull uint16
		MaxChipRevFull uint16
		Reserved       [4]uint8
		HashAppended   uint8
	}

	type espSegmentHeader struct {
		LoadAddr uint32
		DataLen  uint32
	}

	base := uintptr(flashMapBase) + uintptr(flashOffset)
	hdr := (*espImageHeader)(unsafe.Pointer(base))
	if hdr.Magic != espImageMagic {
		println("  invalid image magic for", name)
		return 0
	}

	// Walk ESP image segments and map them into the domain memory view.
	cursor := base + unsafe.Sizeof(espImageHeader{})
	var codeVirt uint32
	var totalLen uint32
	for i := uint8(0); i < hdr.SegmentCount; i++ {
		seg := (*espSegmentHeader)(unsafe.Pointer(cursor))
		segData := cursor + unsafe.Sizeof(espSegmentHeader{})
		flashPhys := flashOffset + uint32(segData-base)
		if codeVirt == 0 {
			codeVirt = seg.LoadAddr
		}

		if memoryProtection != nil {
			_ = memoryProtection.Map(uintptr(seg.LoadAddr), uintptr(flashPhys), seg.DataLen, hal.PermRead|hal.PermExecute|hal.PermUser)
		}

		totalLen += seg.DataLen
		cursor = segData + uintptr(seg.DataLen)
		cursor = (cursor + 3) &^ uintptr(3)
	}

	if codeVirt == 0 {
		return 0
	}

	domainID, err := DomainSpawn(name, codeVirt, totalLen, 0, 0, nil, 2)
	if err != ErrNone {
		println("  failed to spawn", name, "err", err)
		return 0
	}

	startDomainRuntime(name)
	return domainID
}

func startDomainRuntime(name string) {
	switch name {
	case "wifi":
		go runWiFiDomain()
	case "http-server":
		go runHTTPDomain()
	case "led-blinker":
		go runLEDDomain()
	}
}

func runWiFiDomain() {
	println("[WiFi] Domain starting (ESP32-S3)")
	ticker := 0
	for {
		ticker++
		if ticker%10 == 0 {
			println("[WiFi] alive")
		}
		vTaskDelay(500)
	}
}

func runHTTPDomain() {
	println("[HTTP] Domain starting (ESP32-S3)")
	ticker := 0
	for {
		ticker++
		if ticker%10 == 0 {
			println("[HTTP] alive")
		}
		vTaskDelay(500)
	}
}

func runLEDDomain() {
	println("[LED] Starting WS2812 on GPIO 48")

	err := machine.SPI0.Configure(machine.SPIConfig{
		Frequency: 3_200_000, // 80MHz/25 = 3.2MHz; 4 SPI bits = 1.25µs WS2812 bit
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

	// color order: R, G, B (ws2812Write sends as GRB)
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
		vTaskDelay(600)
	}
}

// ws2812Write sends one WS2812 LED color via SPI MOSI at 3.2MHz.
// Each WS2812 bit is encoded as 4 SPI bits: 1→1110 (0xE), 0→1000 (0x8).
// Two WS2812 bits pack into one SPI byte, so 8 WS2812 bits = 4 SPI bytes.
// WS2812 expects GRB byte order.
func ws2812Write(r, g, b uint8) {
	var buf [14]byte // 12 bytes GRB data + 2 zero bytes (reset pulse start)
	ws2812EncodeByte(g, buf[0:4])
	ws2812EncodeByte(r, buf[4:8])
	ws2812EncodeByte(b, buf[8:12])
	// buf[12], buf[13] = 0x00 — begins the >50µs reset; vTaskDelay finishes it
	machine.SPI0.Tx(buf[:], nil)
}

// ws2812EncodeByte encodes one WS2812 data byte into 4 SPI bytes (MSB first).
// bit=1 → SPI nibble 1110; bit=0 → SPI nibble 1000.
// Each SPI byte carries two WS2812 bits in its high and low nibbles.
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

func printStats() {
	println("\n--- System Stats ---")
	println("Uptime:", millis()/1000, "seconds")
	println("Free heap:", xPortGetFreeHeapSize()/1024, "KB")

	// Print domain stats
	for i := DomainID(1); i < maxDomains; i++ {
		if domainTable[i].State != DomainStateInvalid {
			name := string(trimNull(domainTable[i].Name[:]))
			println("Domain", i, name, "- caps:", domainTable[i].CapCount)
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
