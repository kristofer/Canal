//go:build tinygo

package kernel

import (
    "unsafe"
)

var kernelSyscallQ QueueHandle_t

// Initialize syscall system
func InitSyscall() {
    // Create kernel syscall queue
    kernelSyscallQ = xQueueCreate(32, uint32(unsafe.Sizeof(SyscallRequest{})))
}

// Syscall handler task
func SyscallHandler(params unsafe.Pointer) {
    var req SyscallRequest
    var resp SyscallResponse

    for {
        // Wait for syscall request
        result := xQueueReceive(
            QueueHandle_t(kernelSyscallQ),
            unsafe.Pointer(&req),
            portMAX_DELAY,
        )

        if result != pdTRUE {
            continue
        }

        // Dispatch syscall
        switch req.Op {
        case SysCapRequest:
            resp = handleCapRequest(&req)

        case SysCapGrant:
            resp = handleCapGrant(&req)

        case SysCapRevoke:
            resp = handleCapRevoke(&req)

        case SysCapSend:
            resp = handleCapSend(&req)

        case SysCapRecv:
            resp = handleCapRecv(&req)

        case SysMemAlloc:
            resp = handleMemAlloc(&req)

        case SysDomainSpawn:
            resp = handleDomainSpawn(&req)

        case SysDomainKill:
            resp = handleDomainKill(&req)

        case SysDebugPrint:
            resp = handleDebugPrint(&req)

        default:
            resp = SyscallResponse{
                Result: -1,
                Error:  ErrInvalidCap,
            }
        }

        // Send response back to requesting domain
        domain := &domainTable[req.DomainID]
        xQueueSend(
            QueueHandle_t(domain.ReplyQ),
            unsafe.Pointer(&resp),
            portMAX_DELAY,
        )
    }
}

// Handle capability request
func handleCapRequest(req *SyscallRequest) SyscallResponse {
    // req.Arg0 = pointer to capability name string
    // req.Arg1 = length of name
    // req.Arg2 = requested rights

    namePtr := (*byte)(unsafe.Pointer(uintptr(req.Arg0)))
    nameLen := int(req.Arg1)
    rights := req.Arg2

    // Get capability name
    var name string
    nameBytes := unsafe.Slice(namePtr, nameLen)
    name = string(nameBytes)

    // Look up capability by name (simplified - in reality would have a registry)
    capID := findCapabilityByName(name, req.DomainID, rights)

    if capID == CapabilityID(0xFFFFFFFF) {
        return SyscallResponse{
            Result: -1,
            Error:  ErrInvalidCap,
        }
    }

    // Add to domain's capability list
    domain := &domainTable[req.DomainID]
    if domain.CapCount >= uint8(len(domain.Caps)) {
        return SyscallResponse{
            Result: -1,
            Error:  ErrCapTableFull,
        }
    }

    domain.Caps[domain.CapCount] = capID
    domain.CapCount++

    return SyscallResponse{
        Result: 0,
        CapID:  capID,
        Error:  ErrNone,
    }
}

// Handle send through capability
func handleCapSend(req *SyscallRequest) SyscallResponse {
    err := CapSend(req.CapID, req.DomainID, req.DataPtr, req.DataLen)

    return SyscallResponse{
        Result: int32(err),
        Error:  uint8(err),
    }
}

// Handle receive from capability
func handleCapRecv(req *SyscallRequest) SyscallResponse {
    err := CapRecv(req.CapID, req.DomainID, req.DataPtr, req.DataLen)

    return SyscallResponse{
        Result: int32(err),
        Error:  uint8(err),
    }
}

// Handle memory allocation
func handleMemAlloc(req *SyscallRequest) SyscallResponse {
    size := req.Arg0
    addr := allocMemory(size)

    if addr == 0 {
        return SyscallResponse{
            Result: -1,
            Error:  ErrOutOfMemory,
        }
    }

    return SyscallResponse{
        Result: int32(addr),
        Error:  ErrNone,
    }
}

// Handle domain spawn
func handleDomainSpawn(req *SyscallRequest) SyscallResponse {
    // Simplified - would load from ELF
    return SyscallResponse{
        Result: -1,
        Error:  ErrPermissionDenied, // Not implemented
    }
}

// Handle domain kill
func handleDomainKill(req *SyscallRequest) SyscallResponse {
    targetID := DomainID(req.Arg0)
    err := DomainKill(targetID)

    return SyscallResponse{
        Result: int32(err),
        Error:  uint8(err),
    }
}

// Handle debug print
func handleDebugPrint(req *SyscallRequest) SyscallResponse {
    // req.DataPtr = pointer to string
    // req.DataLen = length

    data := unsafe.Slice((*byte)(req.DataPtr), req.DataLen)
    debugWrite(data)

    return SyscallResponse{
        Result: 0,
        Error:  ErrNone,
    }
}

// Capability registry (simplified)
var capRegistry = make(map[string]CapabilityID)

func findCapabilityByName(name string, requestor DomainID, rights uint32) CapabilityID {
    // Check registry first
    if capID, ok := capRegistry[name]; ok {
        return capID
    }

    // Create new capability for known services
    // In reality, this would check against a policy

    switch name {
    case "device:gpio":
        // Create channel capability for GPIO service
        queue := xQueueCreate(4, 32) // Small queue, 32-byte messages
        capID := CapAlloc(requestor, CapTypeChannel, unsafe.Pointer(queue), rights)
        capRegistry[name] = capID
        return capID

    case "device:uart0":
        queue := xQueueCreate(16, 64)
        capID := CapAlloc(requestor, CapTypeChannel, unsafe.Pointer(queue), rights)
        capRegistry[name] = capID
        return capID

    default:
        return CapabilityID(0xFFFFFFFF)
    }
}
