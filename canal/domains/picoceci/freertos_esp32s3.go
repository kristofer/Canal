//go:build tinygo && esp32s3

package main

import "unsafe"

// vTaskDelay is provided by FreeRTOS (via ESP-IDF at link time).
// One tick = 1 ms with portTICK_PERIOD_MS=1 on ESP32-S3 IDF default config.
//
//export vTaskDelay
func vTaskDelay(xTicksToDelay uint32)

//export read
func read(fd int32, buf unsafe.Pointer, count uint32) int32

//export write
func write(fd int32, buf unsafe.Pointer, count uint32) int32

//export usb_serial_jtag_read_bytes
func usbSerialJtagReadBytes(buf unsafe.Pointer, length uint32, ticksToWait uint32) int32

//export usb_serial_jtag_write_bytes
func usbSerialJtagWriteBytes(src unsafe.Pointer, size uint32, ticksToWait uint32) int32

//export usb_serial_jtag_wait_tx_done
func usbSerialJtagWaitTxDone(ticksToWait uint32) int32

// portMAX_DELAY parks a FreeRTOS task indefinitely (lowest overhead).
const portMAX_DELAY uint32 = 0xFFFFFFFF

// initDomainHeap initializes the TinyGo leaking GC heap pointer.
// When domain_entry is called from a FreeRTOS task, the TinyGo startup
// path (call_start_cpu0 → tinygo_main) is bypassed, so runtime.heapptr
// is never set and stays at 0 (BSS). Any heap allocation then crashes
// with StoreProhibited at address 0.
//
// We set heapptr = &_edata (end of initialized data), which is the first
// free byte in the domain's DRAM window.
//
// //go:extern creates an external reference (not a new definition) so it
// safely resolves to gc_leaking.go's existing heapptr without LTO conflict.
//
//go:extern runtime.heapptr
var runtimeHeapPtr uintptr

//go:extern _edata
var _edataSymbol [0]byte

func initDomainHeap() {
	runtimeHeapPtr = uintptr(unsafe.Pointer(&_edataSymbol))
}

func initDomainConsole() {
	// Kernel startup configures USB Serial/JTAG VFS in driver mode and owns
	// interrupt setup. Do not override it from inside the domain.
}
