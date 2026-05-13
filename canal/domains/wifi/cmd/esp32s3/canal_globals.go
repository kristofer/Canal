//go:build tinygo && esp32s3

package main

import (
	"fmt"
	cfs "stdlib/fs"
	"strings"
	"unsafe"

	"github.com/kristofer/picoceci/pkg/bytecode"
	"github.com/kristofer/picoceci/pkg/object"
)

type fsOperation uint8

const (
	fsRightRead      = rightRead
	fsRightReadWrite = rightRead | rightWrite

	fsModeRead     uint8 = 1 << 0
	fsModeWrite    uint8 = 1 << 1
	fsModeCreate   uint8 = 1 << 2
	fsModeTruncate uint8 = 1 << 4

	fsMaxPathLen   = 192
	fsMaxChunkSize = 224
	fsMaxListItems = 16
	fsPayloadSize  = 512
	fsTimeoutTicks = 10000

	fsOpOpen fsOperation = iota
	fsOpClose
	fsOpRead
	fsOpWrite
	fsOpSeek
	fsOpSync
	fsOpStat
	fsOpList
	fsOpMkdir
	fsOpRemove
	fsOpRename
)

type fsMessage struct {
	Op      fsOperation
	ReplyQ  uint32
	Payload [fsPayloadSize]byte
}

type fsOpenRequest struct {
	Path [fsMaxPathLen]byte
	Mode uint8
}

type fsOpenResponse struct {
	Handle  uint32
	Size    uint64
	Success bool
}

type fsCloseRequest struct {
	Handle uint32
}

type fsReadRequest struct {
	Handle uint32
	Length uint16
}

type fsReadResponse struct {
	BytesRead uint16
	EOF       bool
	Success   bool
	Data      [fsMaxChunkSize]byte
}

type fsWriteRequest struct {
	Handle uint32
	Length uint16
	Data   [fsMaxChunkSize]byte
}

type fsWriteResponse struct {
	BytesWritten uint16
	Success      bool
}

type fsSyncRequest struct {
	Handle uint32
}

type fsStatRequest struct {
	Path [fsMaxPathLen]byte
}

type fsStatResponse struct {
	Exists   bool
	IsDir    bool
	Readable bool
	Writable bool
	Success  bool
	Size     uint64
	ModTime  uint64
}

type fsListRequest struct {
	Path     [fsMaxPathLen]byte
	MaxItems uint16
}

type fsListEntry struct {
	Name    [fsMaxPathLen]byte
	IsDir   bool
	Size    uint64
	ModTime uint64
}

type fsListResponse struct {
	NumItems uint16
	Success  bool
	Items    [fsMaxListItems]fsListEntry
}

type fsBoolResponse struct {
	Success bool
}

var (
	errNotAString  = fmt.Errorf("argument must be String or Symbol")
	errNeedsString = fmt.Errorf("data must be String or ByteArray")
	fsReadCapID    uint32
	fsWriteCapID   uint32
)

func installCanalGlobals(vm *bytecode.VM) {
	vm.SetGlobal("Canal", makeCanalObject())
	vm.SetGlobal("FS", makeFSObject())
}

func makeCanalObject() *object.Object {
	o := object.NewObject(make(map[string]*object.MethodDef))
	openChannelMethod := &object.MethodDef{Native: func(_ *object.Object, args []*object.Object) (*object.Object, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("openChannel: expects one symbol or string argument")
		}

		name, err := symbolOrString(args[0])
		if err != nil {
			return nil, err
		}

		switch name {
		case CapabilityFSRead, CapabilityFSWrite, CapabilityFSReadWrite:
			return makeFSObject(), nil
		default:
			supported := fmt.Sprintf("%s, %s, %s", CapabilityFSRead, CapabilityFSWrite, CapabilityFSReadWrite)
			return nil, fmt.Errorf("unknown capability #%s; supported: %s", name, supported)
		}
	}}
	o.Methods["openChannel:"] = openChannelMethod
	// Deprecated compatibility alias; prefer openChannel: in new picoceci code.
	// Planned removal: after phase-5 typed-channel fast-path validation.
	o.Methods["capability:"] = openChannelMethod

	o.Methods["printString"] = &object.MethodDef{Native: func(_ *object.Object, _ []*object.Object) (*object.Object, error) {
		return object.StringObject("Canal"), nil
	}}

	return o
}

func makeFSObject() *object.Object {
	o := object.NewObject(make(map[string]*object.MethodDef))

	o.Methods["list:"] = &object.MethodDef{Native: func(self *object.Object, args []*object.Object) (*object.Object, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("list: expects one string argument")
		}
		path, err := symbolOrString(args[0])
		if err != nil {
			return nil, err
		}

		items, err := fsList(path)
		if err != nil {
			return nil, fmt.Errorf("list %q failed: %w", path, err)
		}
		arr := object.ArrayObject(len(items))
		for i := range items {
			arr.Items[i] = object.StringObject(items[i])
		}
		return arr, nil
	}}

	o.Methods["readFile:"] = &object.MethodDef{Native: func(self *object.Object, args []*object.Object) (*object.Object, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("readFile: expects one string argument")
		}
		path, err := symbolOrString(args[0])
		if err != nil {
			return nil, err
		}

		data, err := fsReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("readFile %q failed: %w", path, err)
		}
		return object.StringObject(string(data)), nil
	}}

	o.Methods["exists:"] = &object.MethodDef{Native: func(self *object.Object, args []*object.Object) (*object.Object, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("exists: expects one string argument")
		}
		path, err := symbolOrString(args[0])
		if err != nil {
			return nil, err
		}
		_, err = fsReadFile(path)
		return object.BoolObject(err == nil), nil
	}}

	o.Methods["writeFile:data:"] = &object.MethodDef{Native: func(self *object.Object, args []*object.Object) (*object.Object, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("writeFile:data: expects path and data arguments")
		}

		path, err := symbolOrString(args[0])
		if err != nil {
			return nil, err
		}

		var data []byte
		switch args[1].Kind {
		case object.KindString:
			data = []byte(args[1].SVal)
		case object.KindByteArray:
			data = args[1].Bytes
		default:
			return nil, errNeedsString
		}

		if err := fsWriteFile(path, data); err != nil {
			return nil, fmt.Errorf("writeFile %q failed: %w", path, err)
		}
		return object.Nil, nil
	}}

	o.Methods["printString"] = &object.MethodDef{Native: func(_ *object.Object, _ []*object.Object) (*object.Object, error) {
		return object.StringObject("CanalFS(service:fs)"), nil
	}}

	return o
}

func symbolOrString(o *object.Object) (string, error) {
	if o == nil {
		return "", fmt.Errorf("expected Symbol or String, got nil")
	}
	switch o.Kind {
	case object.KindSymbol, object.KindString:
		return cleanPath(o.SVal), nil
	default:
		return "", errNotAString
	}
}

func readModuleFromFS(path string) ([]byte, error) {
	return fsReadFile(path)
}

func ensureFSCapability(rights uint32) (uint32, error) {
	if rights&rightWrite != 0 {
		if fsWriteCapID != 0 {
			return fsWriteCapID, nil
		}
		capID, err := capRequest("service:fs", fsRightReadWrite)
		if err != nil {
			return 0, err
		}
		fsWriteCapID = capID
		return capID, nil
	}

	if fsReadCapID != 0 {
		return fsReadCapID, nil
	}
	capID, err := capRequest("service:fs", fsRightRead)
	if err != nil {
		return 0, err
	}
	fsReadCapID = capID
	return capID, nil
}

func doFSRequest(op fsOperation, rights uint32, req unsafe.Pointer, reqSize uintptr, resp unsafe.Pointer, respSize uintptr) error {
	if !capShimReady() {
		return fmt.Errorf("filesystem service not available (shim not initialized)")
	}
	capID, err := ensureFSCapability(rights)
	if err != nil {
		return fmt.Errorf("cannot request filesystem capability: %w", err)
	}

	replyQ := xQueueCreate(1, uint32(respSize))
	if replyQ == nil {
		return fmt.Errorf("cannot allocate reply queue")
	}
	defer xQueueDelete(replyQ)

	var msg fsMessage
	msg.Op = op
	msg.ReplyQ = uint32(uintptr(unsafe.Pointer(replyQ)))
	if req != nil && reqSize > 0 {
		size := int(reqSize)
		copy(msg.Payload[:], unsafe.Slice((*byte)(req), size))
	}

	if err := capSend(capID, unsafe.Pointer(&msg), uint32(unsafe.Sizeof(msg))); err != nil {
		return fmt.Errorf("cannot send filesystem request: %w", err)
	}

	if xQueueReceive(replyQ, resp, fsTimeoutTicks) != pdTRUE {
		return fmt.Errorf("filesystem request timed out (waited %d ticks)", fsTimeoutTicks)
	}

	return nil
}

func fsList(path string) ([]string, error) {
	items, err := cfs.ReadDir(path)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(items))
	for _, item := range items {
		entry := item.Name
		if item.IsDir {
			entry += "/"
		}
		out = append(out, entry)
	}
	return out, nil
}

func fsReadFile(path string) ([]byte, error) {
	return cfs.ReadFile(path)
}

func fsWriteFile(path string, data []byte) error {
	return cfs.WriteFile(path, data)
}

func cleanPath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}
