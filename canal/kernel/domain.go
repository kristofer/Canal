//go:build tinygo

package kernel

import (
	"unsafe"
)

// Per-domain heap sizes. Domains declare their memory budget at spawn time.
// The slice is allocated from the Go GC heap; HeapStart holds its address
// for future MPU region configuration.
const (
	HeapTiny   uint32 = 2 * 1024  //  2 KB — LED blinker, simple sensor domains
	HeapSmall  uint32 = 8 * 1024  //  8 KB — typical service domain
	HeapMedium uint32 = 32 * 1024 // 32 KB — WiFi, HTTP, or other complex domains
)

const maxDomains = 32

var domainTable [maxDomains]Domain
var domainTableLock uint32

// InitDomainTable marks every slot as empty.
func InitDomainTable() {
	for i := range domainTable {
		domainTable[i].State = DomainStateInvalid
	}
}


// DomainSpawn registers a domain and launches its entry function as a goroutine.
// heapSize reserves real memory from the Go GC heap; use the HeapTiny/Small/Medium
// constants or supply a custom value. Pass nil entry to register without running.
func DomainSpawn(name string, heapSize uint32, entry func(), priority uint8) (DomainID, uint8) {
	// Allocate the domain's heap before taking the table lock — make() can block.
	heap := make([]byte, heapSize)

	spinLock(&domainTableLock)

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

	domainID := DomainID(slot)
	syscallQ := xQueueCreate(8, uint32(unsafe.Sizeof(SyscallRequest{})))
	replyQ := xQueueCreate(8, uint32(unsafe.Sizeof(SyscallResponse{})))

	domain := &domainTable[domainID]
	domain.ID = domainID
	domain.State = DomainStateRunning
	domain.Priority = priority
	domain.SyscallQ = unsafe.Pointer(syscallQ)
	domain.ReplyQ = unsafe.Pointer(replyQ)
	domain.Heap = heap
	domain.HeapStart = uintptr(unsafe.Pointer(&heap[0]))
	domain.HeapSize = heapSize
	domain.CapCount = 0
	copy(domain.Name[:], name)

	spinUnlock(&domainTableLock)

	if entry != nil {
		go entry()
	}

	return domainID, ErrNone
}

// Kill a domain
func DomainKill(domainID DomainID) uint8 {
	if domainID >= DomainID(maxDomains) {
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
