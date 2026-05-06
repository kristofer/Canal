//go:build tinygo && esp32s3

package main

// vTaskDelay is provided by FreeRTOS (via ESP-IDF at link time).
// One tick = 1 ms with portTICK_PERIOD_MS=1 on ESP32-S3 IDF default config.
//
//export vTaskDelay
func vTaskDelay(xTicksToDelay uint32)
