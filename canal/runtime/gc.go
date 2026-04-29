//go:build tinygo

package runtime

import (
    "unsafe"
    "kernel"
)

// GC uses conservative mark-sweep algorithm
// Each domain has its own GC heap - completely isolated

// Heap structure (per-domain)
type gcHeap struct {
    base     uintptr
    size     uint32
    current  uintptr  // Bump allocator pointer
    blocks   [256]gcBlock
    markBits [256]uint64  // Bitmap for marking (256 blocks * 64 bits)
}

// Memory block descriptor
type gcBlock struct {
    addr  uintptr
    size  uint32
    mark  uint8
    inUse uint8
}

// Global (per-domain) heap
var heap gcHeap
var gcLock uint32

// Initialize GC heap for this domain
func gcInit(heapAddr uintptr, heapSize uint32) {
    heap.base = heapAddr
    heap.size = heapSize
    heap.current = heapAddr

    // Clear mark bits
    for i := range heap.markBits {
        heap.markBits[i] = 0
    }

    debugPrintf("[GC] Initialized heap: 0x%08x - 0x%08x (%d KB)\n",
        heapAddr, heapAddr+uintptr(heapSize), heapSize/1024)
}

// Allocate memory from heap
//export runtime.alloc
func runtimeAlloc(size uintptr) unsafe.Pointer {
    spinLock(&gcLock)
    defer spinUnlock(&gcLock)

    // Align to 8 bytes
    size = (size + 7) & ^uintptr(7)

    // Check if we have space
    if heap.current+size > heap.base+uintptr(heap.size) {
        // Trigger GC
        gcCollect()

        // Try again after GC
        if heap.current+size > heap.base+uintptr(heap.size) {
            // Still no space - panic or return nil
            return nil
        }
    }

    // Bump allocate
    addr := heap.current
    heap.current += size

    // Record block
    recordBlock(addr, uint32(size))

    return unsafe.Pointer(addr)
}

// Record allocated block
func recordBlock(addr uintptr, size uint32) {
    for i := range heap.blocks {
        if heap.blocks[i].inUse == 0 {
            heap.blocks[i].addr = addr
            heap.blocks[i].size = size
            heap.blocks[i].inUse = 1
            heap.blocks[i].mark = 0
            return
        }
    }

    // Block table full - this is bad
    debugPrintln("[GC] WARNING: Block table full!")
}

// Garbage collection
func gcCollect() {
    debugPrintln("[GC] Starting collection...")

    startTime := millis()

    // Phase 1: Mark
    gcMark()

    // Phase 2: Sweep
    freed := gcSweep()

    endTime := millis()

    debugPrintf("[GC] Collected %d bytes in %d ms\n", freed, endTime-startTime)
}

// Mark phase - scan roots and mark reachable objects
func gcMark() {
    // Clear mark bits
    for i := range heap.markBits {
        heap.markBits[i] = 0
    }

    // Scan stack (conservative)
    gcScanStack()

    // Scan global variables (would need compiler support)
    gcScanGlobals()

    // Scan goroutine stacks
    gcScanGoroutines()
}

// Scan current stack for pointers
func gcScanStack() {
    // Get stack bounds
    var stackTop uintptr
    arm.Asm("mov %0, sp" : "=r"(stackTop))

    stackBottom := getStackBottom() // From TinyGo runtime

    // Scan every word on stack
    for addr := stackTop; addr < stackBottom; addr += unsafe.Sizeof(uintptr(0)) {
        ptr := *(*uintptr)(unsafe.Pointer(addr))

        // Check if this looks like a heap pointer
        if ptr >= heap.base && ptr < heap.base+uintptr(heap.size) {
            gcMarkObject(ptr)
        }
    }
}

// Mark object and recursively mark anything it points to
func gcMarkObject(ptr uintptr) {
    // Find block containing this pointer
    blockIdx := findBlock(ptr)
    if blockIdx == -1 {
        return
    }

    block := &heap.blocks[blockIdx]

    // Already marked?
    if block.mark != 0 {
        return
    }

    // Mark it
    block.mark = 1

    // Scan block for more pointers (conservative)
    for offset := uintptr(0); offset < uintptr(block.size); offset += unsafe.Sizeof(uintptr(0)) {
        candidate := *(*uintptr)(unsafe.Pointer(block.addr + offset))

        if candidate >= heap.base && candidate < heap.base+uintptr(heap.size) {
            gcMarkObject(candidate)
        }
    }
}

// Find block containing address
func findBlock(addr uintptr) int {
    for i := range heap.blocks {
        if heap.blocks[i].inUse != 0 {
            blockStart := heap.blocks[i].addr
            blockEnd := blockStart + uintptr(heap.blocks[i].size)

            if addr >= blockStart && addr < blockEnd {
                return i
            }
        }
    }
    return -1
}

// Sweep phase - free unmarked objects
func gcSweep() uint32 {
    freed := uint32(0)

    for i := range heap.blocks {
        if heap.blocks[i].inUse != 0 && heap.blocks[i].mark == 0 {
            // Unmarked - free it
            freed += heap.blocks[i].size
            heap.blocks[i].inUse = 0

            // Note: We don't actually reclaim memory in bump allocator
            // A real GC would compact or use freelists
        }
    }

    // Compact heap (simplified - just reset if all blocks freed)
    allFree := true
    for i := range heap.blocks {
        if heap.blocks[i].inUse != 0 {
            allFree = false
            break
        }
    }

    if allFree {
        // Reset heap
        heap.current = heap.base
        debugPrintln("[GC] Heap reset (all blocks freed)")
    }

    return freed
}

// Scan global variables (requires compiler support)
func gcScanGlobals() {
    // TinyGo compiler would emit a list of global roots
    // For now, this is a no-op
}

// Scan goroutine stacks
func gcScanGoroutines() {
    // Iterate through all goroutines in this domain
    for g := allgs; g != nil; g = g.alllink {
        if g.stack.lo != 0 && g.stack.hi != 0 {
            // Scan this goroutine's stack
            for addr := g.stack.lo; addr < g.stack.hi; addr += unsafe.Sizeof(uintptr(0)) {
                ptr := *(*uintptr)(unsafe.Pointer(addr))

                if ptr >= heap.base && ptr < heap.base+uintptr(heap.size) {
                    gcMarkObject(ptr)
                }
            }
        }
    }
}

// Trigger GC manually
//export runtime.GC
func GC() {
    gcCollect()
}

// Get heap stats
type MemStats struct {
    Alloc      uint64  // Bytes allocated
    TotalAlloc uint64  // Total bytes allocated
    Sys        uint64  // Bytes from system
    NumGC      uint32  // Number of GC runs
}

var memStats MemStats
var numGC uint32

func ReadMemStats(m *MemStats) {
    spinLock(&gcLock)
    defer spinUnlock(&gcLock)

    alloc := uint64(0)
    for i := range heap.blocks {
        if heap.blocks[i].inUse != 0 {
            alloc += uint64(heap.blocks[i].size)
        }
    }

    m.Alloc = alloc
    m.TotalAlloc = memStats.TotalAlloc
    m.Sys = uint64(heap.size)
    m.NumGC = numGC
}
