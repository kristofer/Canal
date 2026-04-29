//go:build tinygo && esp32s3 && idf

package kernel

import "unsafe"

// FreeRTOS types aligned with ESP-IDF headers.
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
	tskNO_AFFINITY       int32      = -1
)

// These symbols are provided by ESP-IDF/FreeRTOS at link-time.

//export xTaskCreatePinnedToCore
func xTaskCreatePinnedToCore(
	pvTaskCode unsafe.Pointer,
	pcName *byte,
	usStackDepth uint16,
	pvParameters unsafe.Pointer,
	uxPriority uint32,
	xCoreID int32,
	pvCreatedTask *TaskHandle_t,
) BaseType_t

func xTaskCreate(
	pvTaskCode unsafe.Pointer,
	pcName *byte,
	usStackDepth uint16,
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
		tskNO_AFFINITY,
		pvCreatedTask,
	)
}

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

//export xQueueSend
func xQueueSend(xQueue QueueHandle_t, pvItemToQueue unsafe.Pointer, xTicksToWait uint32) BaseType_t

//export xQueueReceive
func xQueueReceive(xQueue QueueHandle_t, pvBuffer unsafe.Pointer, xTicksToWait uint32) BaseType_t

//export xPortGetFreeHeapSize
func xPortGetFreeHeapSize() uint32

func cstring(s string) *byte {
	return nil
}
