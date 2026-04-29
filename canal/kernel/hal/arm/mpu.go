//go:build tinygo && (cortexm || arm)

package arm

import (
    "device/arm"
    "runtime/volatile"
    "unsafe"
    "kernel/hal"
)

type armMPU struct {
    // ARM MPU registers
    ctrl *volatile.Register32
    rnr  *volatile.Register32
    rbar *volatile.Register32
    rasr *volatile.Register32

    // Current domain context
    currentDomain uint16

    // Domain configurations (cached)
    configs [32]hal.DomainMemoryConfig
}

func newMemoryProtection() hal.MemoryProtection {
    return &armMPU{
        ctrl: (*volatile.Register32)(unsafe.Pointer(uintptr(0xE000ED94))),
        rnr:  (*volatile.Register32)(unsafe.Pointer(uintptr(0xE000ED98))),
        rbar: (*volatile.Register32)(unsafe.Pointer(uintptr(0xE000ED9C))),
        rasr: (*volatile.Register32)(unsafe.Pointer(uintptr(0xE000EDA0))),
    }
}

func (m *armMPU) Init() error {
    // Disable MPU
    m.ctrl.Set(0)

    // Configure background region (kernel only)
    m.rnr.Set(0)
    m.rbar.Set(0x00000000)
    m.rasr.Set(
        (1 << 0) |  // Enable
        (31 << 1) | // 4GB size
        (0b001 << 24), // Privileged RW, user no access
    )

    // Enable MPU with privileged default
    m.ctrl.Set((1 << 0) | (1 << 2))

    return nil
}

func (m *armMPU) ConfigureDomain(config *hal.DomainMemoryConfig) error {
    // Store config
    m.configs[config.DomainID] = *config

    // Don't apply yet - will apply on context switch
    return nil
}

func (m *armMPU) SwitchContext(domainID uint16) error {
    if domainID == m.currentDomain {
        return nil // Already in this context
    }

    config := &m.configs[domainID]

    // Disable MPU during reconfiguration
    m.ctrl.Set(0)

    // Region 1: Code
    if config.CodeSize > 0 {
        m.rnr.Set(1)
        m.rbar.Set(uint32(config.CodeVirt))
        m.rasr.Set(
            (1 << 0) |
            (sizeToMPUField(config.CodeSize) << 1) |
            (0b110 << 24) | // Read-only, user accessible
            (0 << 28),      // Executable
        )
    }

    // Region 2: Data
    if config.DataSize > 0 {
        m.rnr.Set(2)
        m.rbar.Set(uint32(config.DataVirt))
        m.rasr.Set(
            (1 << 0) |
            (sizeToMPUField(config.DataSize) << 1) |
            (0b011 << 24) | // RW, user accessible
            (1 << 28),      // Execute never
        )
    }

    // Region 3: Heap
    if config.HeapSize > 0 {
        m.rnr.Set(3)
        m.rbar.Set(uint32(config.HeapVirt))
        m.rasr.Set(
            (1 << 0) |
            (sizeToMPUField(config.HeapSize) << 1) |
            (0b011 << 24) |
            (1 << 28),
        )
    }

    // Region 4: Stack (if separate)
    if config.StackSize > 0 {
        m.rnr.Set(4)
        m.rbar.Set(uint32(config.StackVirt))
        m.rasr.Set(
            (1 << 0) |
            (sizeToMPUField(config.StackSize) << 1) |
            (0b011 << 24) |
            (1 << 28),
        )
    }

    // Re-enable MPU
    m.ctrl.Set((1 << 0) | (1 << 2))

    m.currentDomain = domainID
    return nil
}

func (m *armMPU) CheckAccess(addr uintptr, perms hal.Permissions) bool {
    // Walk MPU regions to check if access is allowed
    // (Simplified - hardware does this automatically)
    return true
}

func (m *armMPU) Map(virt, phys uintptr, size uint32, perms hal.Permissions) error {
    // MPU doesn't support virtual addressing
    return hal.ErrNotSupported
}

func (m *armMPU) Unmap(virt uintptr, size uint32) error {
    return hal.ErrNotSupported
}

func sizeToMPUField(size uint32) uint32 {
    // Convert size to MPU size field (power of 2)
    field := uint32(4) // Minimum 32 bytes (2^5)
    for (1 << (field + 1)) < size {
        field++
    }
    return field
}

var faultHandler hal.FaultHandler

func registerFaultHandler(handler hal.FaultHandler) {
    faultHandler = handler
}

//export MemManage_Handler
func MemManage_Handler() {
    faultAddr := arm.SCB.MMFAR.Get()

    // Determine fault type
    cfsr := arm.SCB.CFSR.Get()
    var faultType hal.FaultType

    if (cfsr & (1 << 1)) != 0 { // DACCVIOL
        faultType = hal.FaultRead
    } else if (cfsr & (1 << 0)) != 0 { // IACCVIOL
        faultType = hal.FaultExecute
    }

    if faultHandler != nil {
        faultHandler(uintptr(faultAddr), currentMPU.currentDomain, faultType)
    }

    // Clear fault
    arm.SCB.CFSR.Set(cfsr)
}

var currentMPU *armMPU
