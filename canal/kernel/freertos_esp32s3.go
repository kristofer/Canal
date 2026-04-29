//go:build tinygo && esp32s3 && !idf

package kernel

import (
	"time"
	"unsafe"
)

type TaskHandle_t unsafe.Pointer
type QueueHandle_t unsafe.Pointer
type BaseType_t int32

const (
	pdTRUE               BaseType_t = 1
	pdFALSE              BaseType_t = 0
	pdPASS               BaseType_t = 1
	portMAX_DELAY        uint32     = 0xFFFFFFFF
	tskIDLE_PRIORITY     uint32     = 0
	configMAX_PRIORITIES uint32     = 5
)

func xTaskCreate(
	pvTaskCode unsafe.Pointer,
	pcName *byte,
	usStackDepth uint16,
	pvParameters unsafe.Pointer,
	uxPriority uint32,
	pvCreatedTask *TaskHandle_t,
) BaseType_t {
	if pvCreatedTask != nil {
		*pvCreatedTask = TaskHandle_t(unsafe.Pointer(uintptr(1)))
	}
	return pdPASS
}

func vTaskStartScheduler() {}

// xTaskGetTickCount returns milliseconds since boot using the Go runtime clock.
func xTaskGetTickCount() uint32 {
	return uint32(time.Now().UnixMilli())
}

// vTaskDelay blocks for the given number of milliseconds (1 tick = 1 ms).
func vTaskDelay(xTicksToDelay uint32) {
	time.Sleep(time.Duration(xTicksToDelay) * time.Millisecond)
}

func vTaskSuspend(xTaskToSuspend TaskHandle_t) {}

func vTaskResume(xTaskToResume TaskHandle_t) {}

func vTaskDelete(xTaskToDelete TaskHandle_t) {}

func xQueueCreate(uxQueueLength uint32, uxItemSize uint32) QueueHandle_t {
	return QueueHandle_t(unsafe.Pointer(uintptr(1)))
}

func xQueueSend(xQueue QueueHandle_t, pvItemToQueue unsafe.Pointer, xTicksToWait uint32) BaseType_t {
	return pdTRUE
}

func xQueueReceive(xQueue QueueHandle_t, pvBuffer unsafe.Pointer, xTicksToWait uint32) BaseType_t {
	return pdTRUE
}

func xPortGetFreeHeapSize() uint32 {
	return 128 * 1024
}

func cstring(s string) *byte {
	if len(s) == 0 {
		return nil
	}
	b := []byte(s)
	b = append(b, 0)
	return &b[0]
}
