//go:build tinygo && esp32s3

package main

import (
	"fmt"
	"unsafe"
)

type queueHandle unsafe.Pointer
type baseType int32

const (
	pdTRUE   baseType = 1
	qSendBack baseType = 0
)

const (
	rightRead  uint32 = 1 << 0
	rightWrite uint32 = 1 << 1
)

const (
	sysCapRequest uint8 = iota
	sysCapGrant
	sysCapRevoke
	sysCapSend
	sysCapRecv
)

const (
	errNone uint8 = iota
	errInvalidCap
	errPermissionDenied
	errOutOfMemory
	errInvalidDomain
	errCapTableFull
	errDomainTableFull
)

type domainParams struct {
	ID       uint16
	_        uint16
	SyscallQ unsafe.Pointer
	ReplyQ   unsafe.Pointer
}

type syscallRequest struct {
	Op       uint8
	DomainID uint16
	CapID    uint32
	Arg0     uint32
	Arg1     uint32
	Arg2     uint32
	Arg3     uint32
	DataPtr  unsafe.Pointer
	DataLen  uint32
	_padding uint32
}

type syscallResponse struct {
	Result   int32
	CapID    uint32
	Error    uint8
	_padding [3]byte
}

var capCtx struct {
	ready    bool
	domainID uint16
	syscallQ queueHandle
	replyQ   queueHandle
}

var (
	errCapShimUnavailable = fmt.Errorf("capability shim unavailable")
	errCapRequestFailed   = fmt.Errorf("capability request failed")
	errCapSendFailed      = fmt.Errorf("capability send failed")
	errCapRecvFailed      = fmt.Errorf("capability receive failed")
)

//export xQueueGenericSend
func xQueueGenericSend(xQueue queueHandle, pvItemToQueue unsafe.Pointer, xTicksToWait uint32, xCopyPosition baseType) baseType

//export xQueueReceive
func xQueueReceive(xQueue queueHandle, pvBuffer unsafe.Pointer, xTicksToWait uint32) baseType

//export xQueueGenericCreate
func xQueueGenericCreate(uxQueueLength uint32, uxItemSize uint32, ucQueueType uint8) queueHandle

//export vQueueDelete
func vQueueDelete(xQueue queueHandle)

func xQueueCreate(uxQueueLength uint32, uxItemSize uint32) queueHandle {
	return xQueueGenericCreate(uxQueueLength, uxItemSize, 0)
}

func xQueueDelete(q queueHandle) {
	vQueueDelete(q)
}

func initCapabilityShimFromTaskParam(param unsafe.Pointer) {
	if param == nil {
		return
	}
	p := (*domainParams)(param)
	capCtx.domainID = p.ID
	capCtx.syscallQ = queueHandle(p.SyscallQ)
	capCtx.replyQ = queueHandle(p.ReplyQ)
	capCtx.ready = p.SyscallQ != nil && p.ReplyQ != nil
}

func capShimReady() bool {
	return capCtx.ready
}

func capRequest(name string, rights uint32) (uint32, error) {
	if !capCtx.ready {
		return 0, errCapShimUnavailable
	}
	if name == "" {
		return 0, errCapRequestFailed
	}

	nameBytes := []byte(name)
	req := syscallRequest{
		Op:       sysCapRequest,
		DomainID: capCtx.domainID,
		Arg0:     uint32(uintptr(unsafe.Pointer(&nameBytes[0]))),
		Arg1:     uint32(len(nameBytes)),
		Arg2:     rights,
	}
	var resp syscallResponse
	if err := capDoSyscall(&req, &resp); err != nil {
		return 0, err
	}
	if resp.Error != errNone {
		return 0, errCapRequestFailed
	}
	return resp.CapID, nil
}

func capSend(capID uint32, data unsafe.Pointer, size uint32) error {
	if !capCtx.ready {
		return errCapShimUnavailable
	}
	req := syscallRequest{
		Op:       sysCapSend,
		DomainID: capCtx.domainID,
		CapID:    capID,
		DataPtr:  data,
		DataLen:  size,
	}
	var resp syscallResponse
	if err := capDoSyscall(&req, &resp); err != nil {
		return err
	}
	if resp.Error != errNone {
		return errCapSendFailed
	}
	return nil
}

func capDoSyscall(req *syscallRequest, resp *syscallResponse) error {
	if xQueueGenericSend(capCtx.syscallQ, unsafe.Pointer(req), portMAX_DELAY, qSendBack) != pdTRUE {
		return errCapSendFailed
	}
	if xQueueReceive(capCtx.replyQ, unsafe.Pointer(resp), portMAX_DELAY) != pdTRUE {
		return errCapRecvFailed
	}
	return nil
}
