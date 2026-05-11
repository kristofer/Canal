//go:build tinygo

package kernel

// spawnGoEntry is provided by domain_goroutine.go (non-IDF) or domain_task.go (IDF).
// It launches the fallback entry function for a domain when ELF loading is not available.

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
var domainParamsTable [maxDomains]DomainParams

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
		spawnGoEntry(entry, name, priority)
	}

	return domainID, ErrNone
}

// partitionOffset returns the flash partition offset for a domain.
// Must match build/targets/esp32s3/partitions.csv.
func partitionOffset(name string) (uint32, bool) {
	switch name {
	case "led":
		return 0x100000, true
	case "wifi":
		return 0x180000, true
	case "logger":
		return 0x200000, true
	case "picoceci":
		return 0x280000, true
	case "sdcard":
		return 0x300000, true
	case "tls":
		return 0x380000, true
	default:
		return 0, false
	}
}

// partitionSize returns the flash partition size for a domain.
// Must match build/targets/esp32s3/partitions.csv.
func partitionSize(name string) uint32 {
	return 0x80000 // all domain partitions are 512KB
}

// DomainParams is passed as pvParameters to the FreeRTOS task created for a
// loaded domain.
type DomainParams struct {
	ID       DomainID
	SyscallQ unsafe.Pointer
	ReplyQ   unsafe.Pointer
}

// SpawnDomainFromFlash loads a domain ELF from its flash partition, copies
// PT_LOAD segments into RAM, and creates a FreeRTOS task at the ELF entry
// point. The entry point is expected to be domain_entry (see domain-linker.ld).
func SpawnDomainFromFlash(name string, priority uint8) (DomainID, uint8) {
	partOffset, ok := partitionOffset(name)
	if !ok {
		println("[Loader] unknown domain:", name)
		return 0, ErrInvalidDomain
	}

	// Map domain code as executable and compute real entry point.
	entryPoint, mmapHandle, err := LoadDomainMapped(partOffset, partitionSize(name), name)
	if err != nil {
		println("[Loader] load failed:", err.Error())
		return 0, ErrInvalidDomain
	}

	// Allocate a kernel domain slot.
	spinLock(&domainTableLock)
	slot := -1
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

	d := &domainTable[domainID]
	d.ID = domainID
	d.State = DomainStateRunning
	d.Priority = priority
	d.SyscallQ = unsafe.Pointer(syscallQ)
	d.ReplyQ = unsafe.Pointer(replyQ)
	d.MmapHandle = mmapHandle
	copy(d.Name[:], name)

	spinUnlock(&domainTableLock)

	params := &domainParamsTable[domainID]
	*params = DomainParams{
		ID:       domainID,
		SyscallQ: unsafe.Pointer(syscallQ),
		ReplyQ:   unsafe.Pointer(replyQ),
	}

	var taskHandle TaskHandle_t
	stackWords := uint32(4096)
	if name == "wifi" {
		// WiFi + interpreter path is stack-heavy; keep a larger task stack.
		stackWords = 16384
	}
	println("[Kernel] Starting domain task", name, "entry:", entryPoint, "prio:", priority)
	result := BaseType_t(canal_create_task(
		entryPoint,
		cstring(name),
		stackWords,
		unsafe.Pointer(params),
		uint32(priority),
		&taskHandle,
	))

	if result != pdPASS {
		println("[Kernel] xTaskCreate failed for", name, "entry:", entryPoint, "result:", uint32(result))
		domainTable[domainID].State = DomainStateInvalid
		return 0, ErrDomainTableFull
	}

	domainTable[domainID].TaskHandle = unsafe.Pointer(taskHandle)

	println("[Kernel] Domain", name, "loaded, entry:", entryPoint)
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
