//go:build tinygo

package kernel

import (
    "unsafe"
)

// Global domain table
const maxDomains = 32
var domainTable [maxDomains]Domain
var domainTableLock uint32
var nextDomainID DomainID = 1

// Memory allocator (simple bump allocator for now)
var heapBase uintptr = 0x20010000 // Start of allocatable RAM
var heapCurrent uintptr = heapBase
var heapEnd uintptr = 0x20020000 // 64KB heap
var heapLock uint32

// Allocate memory region
func allocMemory(size uint32) uintptr {
    spinLock(&heapLock)
    defer spinUnlock(&heapLock)

    // Align to 32 bytes (cache line)
    size = (size + 31) & ^uint32(31)

    addr := heapCurrent
    heapCurrent += uintptr(size)

    if heapCurrent > heapEnd {
        return 0 // Out of memory
    }

    return addr
}

// Initialize domain table
func InitDomainTable() {
    for i := range domainTable {
        domainTable[i].State = DomainStateInvalid
    }
}

// Allocate domain ID
func allocDomainID() DomainID {
    spinLock(&domainTableLock)
    defer spinUnlock(&domainTableLock)

    id := nextDomainID
    nextDomainID++

    if nextDomainID >= maxDomains {
        nextDomainID = 1 // Wrap around (0 is invalid)
    }

    return id
}

// Domain entry point (wrapper)
//export domainEntry
func domainEntry(params unsafe.Pointer) {
    domainID := DomainID(uintptr(params))
    domain := &domainTable[domainID]

    // Configure MPU for this domain
    ConfigureDomainMPU(domain)

    // Call TinyGo runtime init
    // This will eventually call the domain's main()
    runtimeDomainInit(domainID, domain.SyscallQ, domain.ReplyQ, domain.HeapStart, domain.HeapSize)

    // If we return, domain exited
    DomainKill(domainID)
}

// Spawn a new domain
func DomainSpawn(
    name string,
    codeAddr, codeSize uint32,
    dataAddr, dataSize uint32,
    entryPoint unsafe.Pointer,
    priority uint8,
) (DomainID, error) {

    spinLock(&domainTableLock)

    // Find free slot
    var slot int = -1
    for i := 1; i < maxDomains; i++ {
        if domainTable[i].State == DomainStateInvalid {
            slot = i
            break
        }
    }

    if slot == -1 {
        spinUnlock(&domainTableLock)
        return 0, ErrDomainTableFull
    }

    // Allocate domain ID
    domainID := DomainID(slot)

    // Allocate heap
    heapSize := uint32(32 * 1024) // 32KB default
    heapAddr := allocMemory(heapSize)
    if heapAddr == 0 {
        spinUnlock(&domainTableLock)
        return 0, ErrOutOfMemory
    }

    // Create syscall queues
    syscallQ := xQueueCreate(8, uint32(unsafe.Sizeof(SyscallRequest{})))
    replyQ := xQueueCreate(8, uint32(unsafe.Sizeof(SyscallResponse{})))

    // Initialize domain
    domain := &domainTable[domainID]
    domain.ID = domainID
    domain.State = DomainStateRunning
    domain.Priority = priority
    domain.SyscallQ = unsafe.Pointer(syscallQ)
    domain.ReplyQ = unsafe.Pointer(replyQ)
    domain.HeapStart = heapAddr
    domain.HeapSize = heapSize
    domain.CapCount = 0

    // Copy name
    copy(domain.Name[:], name)

    // Configure MPU regions
    domain.MPURegion.Region0Addr = codeAddr
    domain.MPURegion.Region0Size = codeSize
    domain.MPURegion.Region1Addr = dataAddr
    domain.MPURegion.Region1Size = dataSize
    domain.MPURegion.Region2Addr = uint32(heapAddr)
    domain.MPURegion.Region2Size = heapSize

    spinUnlock(&domainTableLock)

    // Create FreeRTOS task
    var taskHandle TaskHandle_t
    result := xTaskCreate(
        unsafe.Pointer(&domainEntry),
        cstring(name),
        512, // Stack size (words)
        unsafe.Pointer(uintptr(domainID)),
        uint32(priority),
        &taskHandle,
    )

    if result != pdPASS {
        domain.State = DomainStateInvalid
        return 0, ErrOutOfMemory
    }

    domain.TaskHandle = unsafe.Pointer(taskHandle)

    return domainID, ErrNone
}

// Kill a domain
func DomainKill(domainID DomainID) error {
    if domainID >= maxDomains {
        return ErrInvalidDomain
    }

    spinLock(&domainTableLock)
    defer spinUnlock(&domainTableLock)

    domain := &domainTable[domainID]

    if domain.State == DomainStateInvalid {
        return ErrInvalidDomain
    }

    // Revoke all capabilities owned by this domain
    for i := uint8(0); i < domain.CapCount; i++ {
        CapRevoke(domain.Caps[i], domainID)
    }

    // Delete FreeRTOS task
    if domain.TaskHandle != nil {
        vTaskDelete(TaskHandle_t(domain.TaskHandle))
    }

    // Mark as invalid
    domain.State = DomainStateInvalid

    return ErrNone
}

// Find domain by task handle
func findDomainByTask(task TaskHandle_t) DomainID {
    for i := DomainID(1); i < maxDomains; i++ {
        if domainTable[i].TaskHandle == unsafe.Pointer(task) {
            return i
        }
    }
    return 0
}
