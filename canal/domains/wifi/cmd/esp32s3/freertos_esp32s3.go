//go:build tinygo && esp32s3

package main

import "unsafe"

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

const wifiPSRAMHeapSize uint32 = 512 * 1024
const wifiEarlyDRAMHeapSize uintptr = 12 * 1024

// initDomainHeapEarly bootstraps TinyGo's leaking allocator without performing
// any allocation or logging. This makes early startup paths safe even if a
// hidden TinyGo print/string path allocates before the domain is fully up.
func initDomainHeapEarly() {
	base := uintptr(unsafe.Pointer(&_edataSymbol))
	runtimeHeapPtr = base
}

func initDomainHeap() {
	if psram := canalDomainPsramAlloc(wifiPSRAMHeapSize); psram != nil {
		base := uintptr(psram)
		runtimeHeapPtr = base
		println("[WiFi] Heap in PSRAM:", base, "size:", wifiPSRAMHeapSize)
		return
	}

	base := uintptr(unsafe.Pointer(&_edataSymbol))
	runtimeHeapPtr = base
	println("[WiFi] PSRAM alloc failed, using DRAM heap from _edata")
}
