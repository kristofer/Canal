//go:build tinygo && esp32s3 && idf

package kernel

import "unsafe"

// FreeRTOS types aligned with ESP-IDF headers.
type TaskHandle_t unsafe.Pointer
type QueueHandle_t unsafe.Pointer
type BaseType_t int32

type usbSerialJtagDriverConfig struct {
	txBufferSize uint32
	rxBufferSize uint32
}

const (
	pdTRUE               BaseType_t = 1
	pdFALSE              BaseType_t = 0
	pdPASS               BaseType_t = 1
	portMAX_DELAY        uint32     = 0xFFFFFFFF
	tskIDLE_PRIORITY     uint32     = 0
	configMAX_PRIORITIES uint32     = 5
	tskNO_AFFINITY       int32      = 0x7FFFFFFF // CONFIG_FREERTOS_NO_AFFINITY in ESP-IDF v5+
)

// These symbols are provided by ESP-IDF/FreeRTOS at link-time.

//export xTaskCreatePinnedToCore
func xTaskCreatePinnedToCore(
	pvTaskCode unsafe.Pointer,
	pcName *byte,
	usStackDepth uint32,
	pvParameters unsafe.Pointer,
	uxPriority uint32,
	pvCreatedTask *TaskHandle_t,
	xCoreID int32,
) BaseType_t

func xTaskCreate(
	pvTaskCode unsafe.Pointer,
	pcName *byte,
	usStackDepth uint32,
	pvParameters unsafe.Pointer,
	uxPriority uint32,
	pvCreatedTask *TaskHandle_t,
) BaseType_t {
	return xTaskCreatePinnedToCore(
		pvTaskCode,
		pcName,
		usStackDepth,
		pvParameters,
		uxPriority,
		pvCreatedTask,
		tskNO_AFFINITY,
	)
}

//export canal_create_task
func canal_create_task(
	entry uint32,
	name *byte,
	stackWords uint32,
	params unsafe.Pointer,
	priority uint32,
	outHandle *TaskHandle_t,
) int32

//export vTaskStartScheduler
func vTaskStartScheduler()

//export vTaskDelay
func vTaskDelay(xTicksToDelay uint32)

//export xTaskGetTickCount
func xTaskGetTickCount() uint32

//export vTaskSuspend
func vTaskSuspend(xTaskToSuspend TaskHandle_t)

//export vTaskResume
func vTaskResume(xTaskToResume TaskHandle_t)

//export vTaskDelete
func vTaskDelete(xTaskToDelete TaskHandle_t)

//export xQueueGenericCreate
func xQueueGenericCreate(uxQueueLength uint32, uxItemSize uint32, ucQueueType uint8) QueueHandle_t

func xQueueCreate(uxQueueLength uint32, uxItemSize uint32) QueueHandle_t {
	return xQueueGenericCreate(uxQueueLength, uxItemSize, 0)
}

//export xQueueGenericSend
func xQueueGenericSend(
	xQueue QueueHandle_t,
	pvItemToQueue unsafe.Pointer,
	xTicksToWait uint32,
	xCopyPosition BaseType_t,
) BaseType_t

func xQueueSend(xQueue QueueHandle_t, pvItemToQueue unsafe.Pointer, xTicksToWait uint32) BaseType_t {
	// Match FreeRTOS xQueueSend macro (queueSEND_TO_BACK = 0).
	return xQueueGenericSend(xQueue, pvItemToQueue, xTicksToWait, 0)
}

//export xQueueReceive
func xQueueReceive(xQueue QueueHandle_t, pvBuffer unsafe.Pointer, xTicksToWait uint32) BaseType_t

//export xPortGetFreeHeapSize
func xPortGetFreeHeapSize() uint32

// USB Serial/JTAG driver + VFS hooks from ESP-IDF.
//
//export usb_serial_jtag_is_driver_installed
func usbSerialJtagIsDriverInstalled() bool

//export usb_serial_jtag_driver_install
func usbSerialJtagDriverInstall(config *usbSerialJtagDriverConfig) int32

//export usb_serial_jtag_vfs_use_driver
func usbSerialJtagVFSUseDriver()

//export usb_serial_jtag_vfs_use_nonblocking
func usbSerialJtagVFSUseNonblocking()

//export usb_serial_jtag_write_bytes
func usbSerialJtagWriteBytes(src unsafe.Pointer, size uint32, ticksToWait uint32) int32

//export usb_serial_jtag_wait_tx_done
func usbSerialJtagWaitTxDone(ticksToWait uint32) int32

// ESP-IDF WiFi API - exported to domains
//
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

// LwIP socket API - exported to domains
//
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

// ESP-IDF network interface initialization
//
//export esp_netif_init
func espNetifInit() int32

//export esp_event_loop_create_default
func espEventLoopCreateDefault() int32

//export esp_netif_create_default_wifi_sta
func espNetifCreateDefaultWifiSta() unsafe.Pointer

//export esp_netif_get_ip_info
func espNetifGetIPInfo(netif unsafe.Pointer, ipInfo unsafe.Pointer) int32

// NVS (non-volatile storage) - required for WiFi
//
//export nvs_flash_init
func nvsFlashInit() int32

//export nvs_flash_erase
func nvsFlashErase() int32

var wifiStaNetif unsafe.Pointer

//export canal_wifi_sta_netif
func canalWiFiStaNetif() unsafe.Pointer {
	return wifiStaNetif
}

func initWiFi() {
	println("[kernel] Initializing WiFi stack...")

	// Initialize NVS (required for WiFi calibration data)
	ret := nvsFlashInit()
	if ret != 0 {
		// NVS partition might be corrupted, try erasing and reinitializing
		println("[kernel] NVS init failed, erasing...")
		if nvsFlashErase() == 0 {
			ret = nvsFlashInit()
		}
	}
	if ret != 0 {
		println("[kernel] NVS init failed:", ret)
		return
	}
	println("[kernel] NVS initialized")

	// Initialize TCP/IP adapter (netif)
	if ret := espNetifInit(); ret != 0 {
		println("[kernel] esp_netif_init failed:", ret)
		return
	}

	// Create default event loop
	if ret := espEventLoopCreateDefault(); ret != 0 {
		println("[kernel] esp_event_loop_create_default failed:", ret)
		return
	}

	// Create default WiFi station interface
	if netif := espNetifCreateDefaultWifiSta(); netif == nil {
		println("[kernel] esp_netif_create_default_wifi_sta failed")
		return
	} else {
		wifiStaNetif = netif
	}

	// Initialize WiFi driver with ESP-IDF default config.
	if ret := canalWiFiInitDefault(); ret != 0 {
		println("[kernel] esp_wifi_init failed:", ret)
		return
	}
	println("[kernel] WiFi driver initialized")

	println("[kernel] WiFi stack initialized (domains can now use WiFi)")
}

func initUSBSerialJTAGConsole() {
	if usbSerialJtagIsDriverInstalled() {
		usbSerialJtagVFSUseDriver()
		usbSerialJtagVFSUseNonblocking()
		return
	}

	cfg := usbSerialJtagDriverConfig{
		txBufferSize: 256,
		rxBufferSize: 256,
	}
	if usbSerialJtagDriverInstall(&cfg) == 0 {
		usbSerialJtagVFSUseDriver()
		usbSerialJtagVFSUseNonblocking()
	}
}

var cstringPool [8][17]byte
var cstringNext uint8

func cstring(s string) *byte {
	if len(s) == 0 {
		return nil
	}

	// Avoid heap allocation: TinyGo allocator can fault this early in boot.
	slot := cstringNext % uint8(len(cstringPool))
	cstringNext++

	buf := &cstringPool[slot]
	max := len(buf) - 1 // keep one byte for NUL terminator
	n := len(s)
	if n > max {
		n = max
	}
	for i := 0; i < n; i++ {
		buf[i] = s[i]
	}
	buf[n] = 0

	return &buf[0]
}
