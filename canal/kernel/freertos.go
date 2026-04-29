//go:build tinygo && !esp32s3

package kernel

import "unsafe"

// FreeRTOS types
type TaskHandle_t unsafe.Pointer
type QueueHandle_t unsafe.Pointer
type BaseType_t int32

// Constants
const (
	pdTRUE               BaseType_t = 1
	pdFALSE              BaseType_t = 0
	pdPASS               BaseType_t = 1
	portMAX_DELAY        uint32     = 0xFFFFFFFF
	tskIDLE_PRIORITY     uint32     = 0
	configMAX_PRIORITIES uint32     = 5
)

// FreeRTOS function declarations
// These are linked from the FreeRTOS C library

//export xTaskCreate
func xTaskCreate(
	pvTaskCode unsafe.Pointer,
	pcName *byte,
	usStackDepth uint16,
	pvParameters unsafe.Pointer,
	uxPriority uint32,
	pvCreatedTask *TaskHandle_t,
) BaseType_t

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

//export xQueueCreate
func xQueueCreate(uxQueueLength uint32, uxItemSize uint32) QueueHandle_t

//export xQueueSend
func xQueueSend(
	xQueue QueueHandle_t,
	pvItemToQueue unsafe.Pointer,
	xTicksToWait uint32,
) BaseType_t

//export xQueueReceive
func xQueueReceive(
	xQueue QueueHandle_t,
	pvBuffer unsafe.Pointer,
	xTicksToWait uint32,
) BaseType_t

//export xPortGetFreeHeapSize
func xPortGetFreeHeapSize() uint32

// Helper: Convert Go string to C string
func cstring(s string) *byte {
	if len(s) == 0 {
		return nil
	}
	b := []byte(s)
	b = append(b, 0)
	return &b[0]
}
