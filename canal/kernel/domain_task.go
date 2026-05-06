//go:build tinygo && idf

package kernel

import "unsafe"

// spawnGoEntry launches the fallback domain entry as a FreeRTOS task.
// Used in IDF builds where the Go scheduler is absent (-scheduler=none).
// The function must be a non-closure package-level function; in TinyGo,
// such a func() value is a single code pointer so we extract it directly.
func spawnGoEntry(entry func(), name string, priority uint8) {
	if entry == nil {
		return
	}
	// In TinyGo a non-closure func() value is represented as a code pointer
	// (first word). Extract it so we can hand it to xTaskCreate.
	fp := *(*uintptr)(unsafe.Pointer(&entry))
	var handle TaskHandle_t
	result := xTaskCreate(
		unsafe.Pointer(fp),
		cstring(name),
		4096,
		nil,
		uint32(priority),
		&handle,
	)
	if result != pdPASS {
		println("[Kernel] fallback xTaskCreate failed for", name)
	}
}
