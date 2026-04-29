//go:build tinygo

package runtime

import (
    "unsafe"
    "kernel"
)

// Request capability from kernel
func RequestCap(name string, rights uint32) (capHandle, error) {
    // Prepare syscall request
    req := kernel.SyscallRequest{
        Op:       kernel.SysCapRequest,
        DomainID: currentProcess.id,
        Arg0:     uint32(uintptr(unsafe.Pointer(&([]byte(name)[0])))),
        Arg1:     uint32(len(name)),
        Arg2:     rights,
    }

    // Send to kernel
    kernel.xQueueSend(
        kernel.QueueHandle_t(currentProcess.syscallQ),
        unsafe.Pointer(&req),
        kernel.portMAX_DELAY,
    )

    // Wait for response
    var resp kernel.SyscallResponse
    kernel.xQueueReceive(
        kernel.QueueHandle_t(currentProcess.replyQ),
        unsafe.Pointer(&resp),
        kernel.portMAX_DELAY,
    )

    if resp.Error != kernel.ErrNone {
        return 0, errFromCode(resp.Error)
    }

    // Store in process capability table
    handle := nextCapHandle
    nextCapHandle++
    currentProcess.caps[handle] = resp.CapID

    return handle, nil
}

// Send data via capability
func CapSend(handle capHandle, data interface{}) error {
    capID, ok := currentProcess.caps[handle]
    if !ok {
        return errInvalidCap
    }

    // Marshal data (simplified)
    buf := marshal(data)

    // Syscall
    req := kernel.SyscallRequest{
        Op:       kernel.SysCapSend,
        DomainID: currentProcess.id,
        CapID:    capID,
        DataPtr:  unsafe.Pointer(&buf[0]),
        DataLen:  uint32(len(buf)),
    }

    kernel.xQueueSend(
        kernel.QueueHandle_t(currentProcess.syscallQ),
        unsafe.Pointer(&req),
        kernel.portMAX_DELAY,
    )

    var resp kernel.SyscallResponse
    kernel.xQueueReceive(
        kernel.QueueHandle_t(currentProcess.replyQ),
        unsafe.Pointer(&resp),
        kernel.portMAX_DELAY,
    )

    return errFromCode(resp.Error)
}

// Receive data via capability
func CapRecv(handle capHandle, data interface{}) error {
    capID, ok := currentProcess.caps[handle]
    if !ok {
        return errInvalidCap
    }

    // Allocate buffer
    buf := make([]byte, 256)  // Max message size

    // Syscall
    req := kernel.SyscallRequest{
        Op:       kernel.SysCapRecv,
        DomainID: currentProcess.id,
        CapID:    capID,
        DataPtr:  unsafe.Pointer(&buf[0]),
        DataLen:  uint32(len(buf)),
    }

    kernel.xQueueSend(
        kernel.QueueHandle_t(currentProcess.syscallQ),
        unsafe.Pointer(&req),
        kernel.portMAX_DELAY,
    )

    var resp kernel.SyscallResponse
    kernel.xQueueReceive(
        kernel.QueueHandle_t(currentProcess.replyQ),
        unsafe.Pointer(&resp),
        kernel.portMAX_DELAY,
    )

    if resp.Error != kernel.ErrNone {
        return errFromCode(resp.Error)
    }

    // Unmarshal into data
    unmarshal(buf, data)

    return nil
}

// Simple marshaling (would use actual encoding in production)
func marshal(v interface{}) []byte {
    // TODO: Proper encoding
    return nil
}

func unmarshal(buf []byte, v interface{}) {
    // TODO: Proper decoding
}

// Error conversion
func errFromCode(code uint8) error {
    switch code {
    case kernel.ErrNone:
        return nil
    case kernel.ErrInvalidCap:
        return errInvalidCap
    case kernel.ErrPermissionDenied:
        return errPermissionDenied
    default:
        return errUnknown
    }
}

var (
    errInvalidCap       = &errorString{"invalid capability"}
    errPermissionDenied = &errorString{"permission denied"}
    errUnknown          = &errorString{"unknown error"}
)

type errorString struct {
    s string
}

func (e *errorString) Error() string {
    return e.s
}
