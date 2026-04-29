//go:build tinygo

package kernel

import "unsafe"

// Domain ID type
type DomainID uint16

// Capability ID type
type CapabilityID uint32

// Capability types
const (
    CapTypeInvalid uint8 = iota
    CapTypeChannel       // IPC channel
    CapTypeMemory        // Memory region
    CapTypeDevice        // Hardware device
    CapTypeIRQ           // Interrupt
    CapTypeService       // Service endpoint
)

// Capability rights (bitmask)
const (
    RightNone    uint32 = 0
    RightRead    uint32 = 1 << 0
    RightWrite   uint32 = 1 << 1
    RightExecute uint32 = 1 << 2
    RightGrant   uint32 = 1 << 3
    RightRevoke  uint32 = 1 << 4
)

// Capability descriptor
type Capability struct {
    ID       CapabilityID
    Type     uint8
    Rights   uint32
    Owner    DomainID
    Target   unsafe.Pointer  // Queue handle, memory addr, etc.
    RefCount uint16
}

// Domain state
type Domain struct {
    ID          DomainID
    State       uint8
    Priority    uint8
    TaskHandle  unsafe.Pointer     // FreeRTOS task
    SyscallQ    unsafe.Pointer     // Syscall request queue
    ReplyQ      unsafe.Pointer     // Syscall response queue
    MPURegion   MPUConfig
    Caps        [16]CapabilityID   // Owned capabilities
    CapCount    uint8
    HeapStart   uintptr
    HeapSize    uint32
    Name        [16]byte
}

// Domain states
const (
    DomainStateInvalid uint8 = iota
    DomainStateRunning
    DomainStateSuspended
    DomainStateDead
)

// MPU configuration
type MPUConfig struct {
    Region0Addr uint32  // Code
    Region0Size uint32
    Region0Attr uint32
    Region1Addr uint32  // Data
    Region1Size uint32
    Region1Attr uint32
    Region2Addr uint32  // Heap
    Region2Size uint32
    Region2Attr uint32
}

// Syscall opcodes
const (
    SysCapRequest uint8 = iota
    SysCapGrant
    SysCapRevoke
    SysCapSend
    SysCapRecv
    SysMemAlloc
    SysDomainSpawn
    SysDomainKill
    SysDebugPrint
)

// Syscall request (32 bytes, cache-line aligned)
type SyscallRequest struct {
    Op       uint8
    DomainID DomainID
    CapID    CapabilityID
    Arg0     uint32
    Arg1     uint32
    Arg2     uint32
    Arg3     uint32
    DataPtr  unsafe.Pointer
    DataLen  uint32
    _padding uint32
}

// Syscall response
type SyscallResponse struct {
    Result   int32
    CapID    CapabilityID
    Error    uint8
    _padding [3]byte
}

// Error codes
const (
    ErrNone uint8 = iota
    ErrInvalidCap
    ErrPermissionDenied
    ErrOutOfMemory
    ErrInvalidDomain
    ErrCapTableFull
    ErrDomainTableFull
)
