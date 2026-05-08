//go:build tinygo && esp32s3

package main

import "unsafe"

// WiFi modes
const (
	WIFI_MODE_NULL  = 0
	WIFI_MODE_STA   = 1
	WIFI_MODE_AP    = 2
	WIFI_MODE_APSTA = 3
)

// WiFi interface
const (
	WIFI_IF_STA = 0
	WIFI_IF_AP  = 1
)

// WiFi config structure (simplified)
type wifiConfigT struct {
	ssid           [32]byte
	password       [64]byte
	scanMethod     uint32
	bssidSet       bool
	bssid          [6]byte
	channel        uint8
	listenInterval uint16
	sortMethod     uint32
	threshold      authModeThreshold
	pmfCfg         pmfCfg
	sapH2eId       [32]byte
}

type authModeThreshold struct {
	mode uint32
	rssi int8
}

type pmfCfg struct {
	capable  bool
	required bool
}

// Socket constants
const (
	ESP_ERR_WIFI_NOT_INIT = 0x3001

	AF_INET      = 2
	SOCK_STREAM  = 1
	SOCK_DGRAM   = 2
	IPPROTO_TCP  = 6
	IPPROTO_UDP  = 17
	SOL_SOCKET   = 0xfff
	SO_REUSEADDR = 0x0004
	SO_KEEPALIVE = 0x0008
	SO_RCVTIMEO  = 0x1006
	SO_SNDTIMEO  = 0x1005
	F_GETFL      = 3
	F_SETFL      = 4
	O_NONBLOCK   = 0x4000
)

// sockaddr_in structure for IPv4
type sockaddrIn struct {
	sinLen    uint8
	sinFamily uint8
	sinPort   uint16
	sinAddr   [4]byte
	sinZero   [8]byte
}

// FreeRTOS - provided by kernel via --just-symbols
//export vTaskDelay
func vTaskDelay(xTicksToDelay uint32)

const portMAX_DELAY uint32 = 0xFFFFFFFF

// ESP-IDF WiFi API - provided by kernel via ESP-IDF link
//export esp_wifi_init
func espWifiInit(config unsafe.Pointer) int32

//export esp_wifi_set_mode
func espWifiSetMode(mode uint32) int32

//export esp_wifi_set_config
func espWifiSetConfig(interface_ uint32, conf unsafe.Pointer) int32

//export esp_wifi_start
func espWifiStart() int32

//export esp_wifi_connect
func espWifiConnect() int32

//export canal_wifi_init_default
func canalWiFiInitDefault() int32

//export canal_domain_psram_alloc
func canalDomainPsramAlloc(size uint32) unsafe.Pointer

// LwIP socket API - provided by kernel via ESP-IDF link
//export lwip_socket
func lwipSocket(domain, typ, protocol int32) int32

//export lwip_bind
func lwipBind(s int32, name unsafe.Pointer, namelen uint32) int32

//export lwip_listen
func lwipListen(s int32, backlog int32) int32

//export lwip_accept
func lwipAccept(s int32, addr unsafe.Pointer, addrlen unsafe.Pointer) int32

//export lwip_recv
func lwipRecv(s int32, mem unsafe.Pointer, len int32, flags int32) int32

//export lwip_send
func lwipSend(s int32, dataptr unsafe.Pointer, size int32, flags int32) int32

//export lwip_close
func lwipClose(s int32) int32

//export lwip_setsockopt
func lwipSetsockopt(s int32, level, optname int32, optval unsafe.Pointer, optlen uint32) int32

//export lwip_fcntl
func lwipFcntl(s int32, cmd int32, val int32) int32

// Logging helpers
func logToSerial(msg string) {
	println(msg)
}

func logToSerialLine(msg string) {
	println(msg)
}
