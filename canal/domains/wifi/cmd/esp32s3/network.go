//go:build tinygo && esp32s3

package main

import (
	"unsafe"
)

// connectToWiFi initializes WiFi and connects to the specified AP
func connectToWiFi(ssid, password string) bool {
	// Note: esp_wifi_init is now called by kernel during boot
	println("[WiFi] connectToWiFi begin")
	vTaskDelay(20)

	println("[WiFi] Step 1: esp_wifi_set_mode")
	// Set station mode
	ret := espWifiSetMode(WIFI_MODE_STA)
	if ret == ESP_ERR_WIFI_NOT_INIT {
		println("[WiFi] WiFi not initialized, attempting default init")
		initRet := canalWiFiInitDefault()
		println("[WiFi] canal_wifi_init_default returned:", initRet)
		if initRet == 0 {
			ret = espWifiSetMode(WIFI_MODE_STA)
			println("[WiFi] esp_wifi_set_mode retry returned:", ret)
		}
	}
	println("[WiFi] esp_wifi_set_mode returned:", ret)
	if ret != 0 {
		println("[WiFi] Failed to set mode")
		return false
	}

	println("[WiFi] Step 2: Preparing config")
	// Configure WiFi credentials
	var config wifiConfigT
	copy(config.ssid[:], ssid)
	copy(config.password[:], password)

	println("[WiFi] Step 3: esp_wifi_set_config")
	ret = espWifiSetConfig(WIFI_IF_STA, unsafe.Pointer(&config))
	println("[WiFi] esp_wifi_set_config returned:", ret)
	if ret != 0 {
		println("[WiFi] Failed to set config")
		return false
	}
	vTaskDelay(1)

	println("[WiFi] Step 4: esp_wifi_start")
	// Start WiFi
	ret = espWifiStart()
	println("[WiFi] esp_wifi_start returned:", ret)
	if ret != 0 {
		println("[WiFi] Failed to start")
		return false
	}
	vTaskDelay(1)

	println("[WiFi] Step 5: esp_wifi_connect to", ssid)

	// Connect to AP
	ret = espWifiConnect()
	println("[WiFi] esp_wifi_connect returned:", ret)
	if ret != 0 {
		println("[WiFi] Failed to connect")
		return false
	}
	vTaskDelay(1)

	// Wait for DHCP/IP assignment and stop as soon as a non-zero address appears.
	println("[WiFi] Step 6: Waiting for IP address (10 seconds)...")
	netif := canalWiFiStaNetif()
	if netif == nil {
		println("[WiFi] STA netif unavailable")
		return false
	}
	for i := 0; i < 100; i++ {
		var ipInfo espNetifIPInfo
		if espNetifGetIPInfo(netif, unsafe.Pointer(&ipInfo)) == 0 && ipInfo.ip.addr != 0 {
			println("[WiFi] Got IP address")
			return true
		}
		vTaskDelay(100) // Wait 100ms
		if i%10 == 0 {
			println("[WiFi] Still waiting... (", i/10, "seconds)")
		}
	}

	println("[WiFi] Timed out waiting for IP address")
	return false
}

// createTCPServer creates a TCP server socket listening on the specified port
func createTCPServer(port uint16) int32 {
	logToSerialLine("[TCP] Creating server socket...")

	// Create socket
	serverFd := lwipSocket(AF_INET, SOCK_STREAM, IPPROTO_TCP)
	if serverFd < 0 {
		logToSerialLine("[TCP] Failed to create socket")
		return -1
	}

	// Set socket options
	optval := int32(1)
	lwipSetsockopt(serverFd, SOL_SOCKET, SO_REUSEADDR,
		unsafe.Pointer(&optval), 4)

	// Bind to port
	var addr sockaddrIn
	addr.sinFamily = AF_INET
	addr.sinPort = htons(port)
	// INADDR_ANY = 0.0.0.0

	if lwipBind(serverFd, unsafe.Pointer(&addr), uint32(unsafe.Sizeof(addr))) != 0 {
		logToSerialLine("[TCP] Failed to bind")
		lwipClose(serverFd)
		return -1
	}

	// Listen
	if lwipListen(serverFd, 1) != 0 {
		logToSerialLine("[TCP] Failed to listen")
		lwipClose(serverFd)
		return -1
	}

	logToSerialLine("[TCP] Server listening on port " + itoa(int(port)))
	return serverFd
}

// acceptTCPClient waits for and accepts a TCP client connection
func acceptTCPClient(serverFd int32) int32 {
	logToSerialLine("[TCP] Waiting for client connection...")

	var clientAddr sockaddrIn
	addrLen := uint32(unsafe.Sizeof(clientAddr))

	clientFd := lwipAccept(serverFd, unsafe.Pointer(&clientAddr), unsafe.Pointer(&addrLen))

	if clientFd >= 0 {
		// Extract client IP
		ip := clientAddr.sinAddr
		ipStr := itoa(int(ip[0])) + "." + itoa(int(ip[1])) + "." +
			itoa(int(ip[2])) + "." + itoa(int(ip[3]))
		logToSerialLine("[TCP] Client connected from " + ipStr)
	}

	return clientFd
}

// htons converts host byte order to network byte order (16-bit)
func htons(n uint16) uint16 {
	return (n<<8 | n>>8)
}

// itoa converts int to string (simple implementation)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var buf [20]byte
	i := len(buf) - 1

	for n > 0 {
		buf[i] = byte('0' + n%10)
		n /= 10
		i--
	}

	if negative {
		buf[i] = '-'
		i--
	}

	return string(buf[i+1:])
}
