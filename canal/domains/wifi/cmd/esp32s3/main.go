//go:build tinygo && esp32s3

package main

import (
	"unsafe"
)

var domainMode bool

// Build-time configuration (set via -ldflags -X)
var wifiSSID string
var wifiPassword string
var tcpPort string = "2323"
var wifiDomainBuild string = "wifi-domain-tcp-shell-v1"

var heapInitialized bool

// domain_entry is called by the kernel's ELF loader via xTaskCreate.
//
//export domain_entry
func domain_entry(param unsafe.Pointer) {
	// Bootstrap a tiny DRAM heap immediately so any hidden TinyGo allocation
	// in early startup or logging has valid backing memory.
	initDomainHeapEarly()
	heapInitialized = true

	domainMode = true
	// Keep loader ABI-compatible signature: kernel passes task params here.
	_ = param
	_ = runWiFi()

	// If runWiFi ever returns, park safely instead of executing fallthrough bytes.
	for {
		vTaskDelay(portMAX_DELAY)
	}
}

func main() {
	logToSerialLine("[WiFi] Standalone start")
	_ = runWiFi()
	for {
		vTaskDelay(portMAX_DELAY)
	}
}

func runWiFi() bool {
	ssid := safeCfgString(wifiSSID)
	password := safeCfgString(wifiPassword)
	portCfg := safeCfgString(tcpPort)
	_ = safeCfgString(wifiDomainBuild)

	// Check if WiFi credentials are configured
	if ssid == "" {
		println("[WiFi] ERROR: WiFi SSID not configured")
		return false
	}

	// Connect to WiFi
	if !connectToWiFi(ssid, password) {
		println("[WiFi] Failed to connect, parking domain...")
		return false
	}

	// Parse TCP port
	port := uint16(2323)
	if portCfg != "" {
		port = uint16(atoi(portCfg))
	}

	// Create TCP server
	serverFd := createTCPServer(port)
	if serverFd < 0 {
		logToSerialLine("[WiFi] Failed to create server")
		return false
	}

	logToSerialLine("[WiFi] Ready! Connect with: nc <device-ip> " + itoa(int(port)))
	logToSerialLine("[WiFi] USB serial is now for logging only")

	// Accept connections and run REPL
	for {
		clientFd := acceptTCPClient(serverFd)
		if clientFd < 0 {
			logToSerialLine("[TCP] Accept failed, retrying...")
			vTaskDelay(1000)
			continue
		}

		// Switch future allocations to PSRAM once the domain is fully up.
		if heapInitialized {
			initDomainHeap()
			heapInitialized = false
		}

		// Run REPL for this client
		console := newTCPConsole(clientFd)
		console.Println("")
		console.Println("=== picoceci over TCP ===")
		console.Println("")
		runREPL(console)

		// Client disconnected
		lwipClose(clientFd)
		logToSerialLine("[TCP] Client disconnected")
	}

	// Unreachable in normal operation.
	return true
}

type stringHeader struct {
	data uintptr
	len  int
}

// safeCfgString prevents crashes if a -X injected string header is malformed.
// It returns an empty string when pointer/length look invalid.
func safeCfgString(s string) string {
	h := (*stringHeader)(unsafe.Pointer(&s))
	if h.len == 0 {
		return ""
	}
	if h.len < 0 || h.len > 256 {
		return ""
	}
	if h.data < 0x3c000000 || h.data >= 0x60000000 {
		return ""
	}
	return s
}

// atoi converts string to int (simple implementation)
func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n = n*10 + int(s[i]-'0')
		}
	}
	return n
}
