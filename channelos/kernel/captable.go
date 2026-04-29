//go:build tinygo

package kernel

import (
    "unsafe"
)

// Global capability table (fixed size for embedded)
const maxCapabilities = 256
var capTable [maxCapabilities]Capability
var capTableLock uint32 // Spinlock

// Spinlock implementation
func spinLock(lock *uint32) {
    for !arm.Cas(lock, 0, 1) {
        arm.Asm("wfe") // Wait for event (power efficient)
    }
}

func spinUnlock(lock *uint32) {
    *lock = 0
    arm.Asm("sev") // Send event to wake waiting cores
}

// Initialize capability table
func InitCapTable() {
    for i := range capTable {
        capTable[i].Type = CapTypeInvalid
    }
}

// Allocate a new capability
func CapAlloc(owner DomainID, capType uint8, target unsafe.Pointer, rights uint32) CapabilityID {
    spinLock(&capTableLock)
    defer spinUnlock(&capTableLock)

    // Find free slot
    for i := uint32(0); i < maxCapabilities; i++ {
        if capTable[i].Type == CapTypeInvalid {
            capTable[i] = Capability{
                ID:       CapabilityID(i),
                Type:     capType,
                Rights:   rights,
                Owner:    owner,
                Target:   target,
                RefCount: 1,
            }
            return CapabilityID(i)
        }
    }

    return CapabilityID(0xFFFFFFFF) // Invalid
}

// Validate capability
func CapValidate(capID CapabilityID, requiredRights uint32) error {
    if capID >= maxCapabilities {
        return ErrInvalidCap
    }

    cap := &capTable[capID]

    if cap.Type == CapTypeInvalid {
        return ErrInvalidCap
    }

    if (cap.Rights & requiredRights) != requiredRights {
        return ErrPermissionDenied
    }

    return ErrNone
}

// Grant capability to another domain
func CapGrant(capID CapabilityID, granter, grantee DomainID) error {
    spinLock(&capTableLock)
    defer spinUnlock(&capTableLock)

    if capID >= maxCapabilities {
        return ErrInvalidCap
    }

    cap := &capTable[capID]

    // Only owner can grant
    if cap.Owner != granter {
        return ErrPermissionDenied
    }

    // Check grant right
    if (cap.Rights & RightGrant) == 0 {
        return ErrPermissionDenied
    }

    // Add to grantee's cap list
    domain := &domainTable[grantee]
    if domain.CapCount >= uint8(len(domain.Caps)) {
        return ErrCapTableFull
    }

    domain.Caps[domain.CapCount] = capID
    domain.CapCount++
    cap.RefCount++

    return ErrNone
}

// Revoke capability
func CapRevoke(capID CapabilityID, revoker DomainID) error {
    spinLock(&capTableLock)
    defer spinUnlock(&capTableLock)

    if capID >= maxCapabilities {
        return ErrInvalidCap
    }

    cap := &capTable[capID]

    // Only owner can revoke
    if cap.Owner != revoker {
        return ErrPermissionDenied
    }

    cap.RefCount--

    // If no more references, free the capability
    if cap.RefCount == 0 {
        // Clean up target (close queue, free memory, etc.)
        if cap.Type == CapTypeChannel {
            // FreeRTOS queue will be cleaned up
        }

        cap.Type = CapTypeInvalid
    }

    return ErrNone
}

// Send data through channel capability
func CapSend(capID CapabilityID, sender DomainID, data unsafe.Pointer, size uint32) error {
    if err := CapValidate(capID, RightWrite); err != ErrNone {
        return err
    }

    cap := &capTable[capID]

    if cap.Type != CapTypeChannel {
        return ErrInvalidCap
    }

    // Send to FreeRTOS queue
    queueHandle := QueueHandle_t(cap.Target)

    // Copy data to queue (blocking)
    result := xQueueSend(queueHandle, data, portMAX_DELAY)

    if result != pdTRUE {
        return ErrPermissionDenied // Queue full or error
    }

    return ErrNone
}

// Receive data from channel capability
func CapRecv(capID CapabilityID, receiver DomainID, data unsafe.Pointer, size uint32) error {
    if err := CapValidate(capID, RightRead); err != ErrNone {
        return err
    }

    cap := &capTable[capID]

    if cap.Type != CapTypeChannel {
        return ErrInvalidCap
    }

    // Receive from FreeRTOS queue
    queueHandle := QueueHandle_t(cap.Target)

    result := xQueueReceive(queueHandle, data, portMAX_DELAY)

    if result != pdTRUE {
        return ErrPermissionDenied
    }

    return ErrNone
}
