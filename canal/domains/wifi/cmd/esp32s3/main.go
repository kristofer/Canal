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
	// Keep entry path minimal; defer heap setup until REPL startup.
	println("[WiFi] Domain entry called")

	domainMode = true
	_ = param // Use param to avoid unused warning

	println("[WiFi] About to start runWiFi")
	runWiFi()
}

func main() {
	logToSerialLine("[WiFi] Standalone start")
	runWiFi()
}

func runWiFi() {
	println("[WiFi] runWiFi started")
	println("[WiFi] Build:", wifiDomainBuild)

	// Check if WiFi credentials are configured
	if wifiSSID == "" {
		println("[WiFi] ERROR: WiFi SSID not configured")
		println("[WiFi] Parking domain...")
		for {
			vTaskDelay(portMAX_DELAY)
		}
	}

	println("[WiFi] SSID configured, about to connect")

	// Connect to WiFi with detailed logging
	println("[WiFi] Calling connectToWiFi...")
	if !connectToWiFi(wifiSSID, wifiPassword) {
		println("[WiFi] Failed to connect, parking domain...")
		for {
			vTaskDelay(1000)
			println("[WiFi] (parked after failed connect)")
		}
	}

	println("[WiFi] WiFi connected! Setting up TCP server...")

	// Parse TCP port
	port := uint16(2323)
	if tcpPort != "" {
		port = uint16(atoi(tcpPort))
	}

	// Create TCP server
	serverFd := createTCPServer(port)
	if serverFd < 0 {
		logToSerialLine("[WiFi] Failed to create server, parking domain...")
		for {
			vTaskDelay(portMAX_DELAY)
		}
	}

	logToSerialLine("[WiFi] Ready! Connect with: telnet <device-ip> " + itoa(int(port)))
	logToSerialLine("[WiFi] USB serial is now for logging only")

	// Accept connections and run REPL
	for {
		clientFd := acceptTCPClient(serverFd)
		if clientFd < 0 {
			logToSerialLine("[TCP] Accept failed, retrying...")
			vTaskDelay(1000)
			continue
		}

		// Initialize TinyGo heap lazily before first REPL session.
		if !heapInitialized {
			initDomainHeap()
			heapInitialized = true
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
