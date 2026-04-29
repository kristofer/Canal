//go:build tinygo

package runtime

import (
    "unsafe"
    "kernel"
)

// Channel structure
type hchan struct {
    queueHandle unsafe.Pointer  // FreeRTOS queue
    elemsize    uintptr
    elemtype    unsafe.Pointer  // Type info
    closed      uint32
    capID       kernel.CapabilityID  // If cross-domain
}

// Make channel
//export runtime.chanMake
func chanMake(elemsize uintptr, size int) unsafe.Pointer {
    // Create FreeRTOS queue
    queueHandle := kernel.xQueueCreate(uint32(size), uint32(elemsize))

    // Allocate channel structure
    ch := (*hchan)(runtimeAlloc(unsafe.Sizeof(hchan{})))
    ch.queueHandle = unsafe.Pointer(queueHandle)
    ch.elemsize = elemsize
    ch.closed = 0
    ch.capID = 0  // Local channel initially

    return unsafe.Pointer(ch)
}

// Send to channel
//export runtime.chanSend
func chanSend(ch unsafe.Pointer, elem unsafe.Pointer) {
    c := (*hchan)(ch)

    if c.capID != 0 {
        // Cross-domain send via capability
        syscallCapSend(c.capID, elem, uint32(c.elemsize))
    } else {
        // Local send via FreeRTOS queue
        kernel.xQueueSend(
            kernel.QueueHandle_t(c.queueHandle),
            elem,
            kernel.portMAX_DELAY,
        )
    }
}

// Receive from channel
//export runtime.chanRecv
func chanRecv(ch unsafe.Pointer, elem unsafe.Pointer) bool {
    c := (*hchan)(ch)

    if c.capID != 0 {
        // Cross-domain receive via capability
        return syscallCapRecv(c.capID, elem, uint32(c.elemsize))
    } else {
        // Local receive via FreeRTOS queue
        result := kernel.xQueueReceive(
            kernel.QueueHandle_t(c.queueHandle),
            elem,
            kernel.portMAX_DELAY,
        )
        return result == kernel.pdTRUE
    }
}

// Close channel
//export runtime.chanClose
func chanClose(ch unsafe.Pointer) {
    c := (*hchan)(ch)
    c.closed = 1

    // Wake up any blocked goroutines
    // (simplified - real implementation would track waiting goroutines)
}

// Select implementation (simplified)
//export runtime.selectgo
func selectgo(cas *_select, order0 *uint16, order1 *uint16) (int, bool) {
    // TinyGo's select would try each case
    // For now, just block on first receive case
    return 0, false
}

type _select struct {
    tcase     uint16
    ncase     uint16
    pollorder *byte
    lockorder *byte
}
