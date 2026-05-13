//go:build tinygo

package fs

import (
	"channel"
	"kernel"
	"runtime"
	"unsafe"
)

type File struct {
	handle uint32
	mode   OpenMode
}

// fsOperationTimeoutTicks waits up to 10s for the SD card service (1 tick = 1 ms).
const fsOperationTimeoutTicks = 10000

var (
	readServiceCap      runtime.CapHandle
	readWriteServiceCap runtime.CapHandle
)

func Open(path string) (*File, error) {
	return OpenFile(path, ModeRead)
}

func Create(path string) (*File, error) {
	return OpenFile(path, ModeWrite|ModeCreate|ModeTruncate)
}

func OpenFile(path string, mode OpenMode) (*File, error) {
	var req openRequest
	copyPath(req.Path[:], path)
	req.Mode = mode

	var resp openResponse
	if err := doRequest(opOpen, modeRights(mode), unsafe.Pointer(&req), unsafe.Sizeof(req), unsafe.Pointer(&resp), unsafe.Sizeof(resp)); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errOperationFailed
	}

	return &File{
		handle: resp.Handle,
		mode:   mode,
	}, nil
}

func ReadFile(path string) ([]byte, error) {
	f, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []byte
	buf := make([]byte, maxChunkSize)
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
		}
		if n == 0 {
			return out, readErr
		}
		if readErr != nil {
			return out, readErr
		}
	}
}

func WriteFile(path string, data []byte) error {
	f, err := Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for len(data) > 0 {
		chunkLen := len(data)
		if chunkLen > maxChunkSize {
			chunkLen = maxChunkSize
		}

		n, writeErr := f.Write(data[:chunkLen])
		if writeErr != nil {
			return writeErr
		}
		data = data[n:]
	}

	return f.Sync()
}

func Stat(path string) (FileInfo, error) {
	var req statRequest
	copyPath(req.Path[:], path)

	var resp statResponse
	if err := doRequest(opStat, runtime.RightRead, unsafe.Pointer(&req), unsafe.Sizeof(req), unsafe.Pointer(&resp), unsafe.Sizeof(resp)); err != nil {
		return FileInfo{}, err
	}
	if !resp.Success {
		return FileInfo{}, errOperationFailed
	}

	return FileInfo{
		Name:     cleanPath(path),
		Exists:   resp.Exists,
		IsDir:    resp.IsDir,
		Readable: resp.Readable,
		Writable: resp.Writable,
		Size:     resp.Size,
		ModTime:  resp.ModTime,
	}, nil
}

func ReadDir(path string) ([]FileInfo, error) {
	var req listRequest
	copyPath(req.Path[:], path)
	req.MaxItems = maxListItems

	var resp listResponse
	if err := doRequest(opList, runtime.RightRead, unsafe.Pointer(&req), unsafe.Sizeof(req), unsafe.Pointer(&resp), unsafe.Sizeof(resp)); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, errOperationFailed
	}

	items := make([]FileInfo, 0, resp.NumItems)
	for i := uint16(0); i < resp.NumItems && i < maxListItems; i++ {
		item := resp.Items[i]
		items = append(items, FileInfo{
			Name:    trimNull(item.Name[:]),
			IsDir:   item.IsDir,
			Size:    item.Size,
			ModTime: item.ModTime,
			Exists:  true,
		})
	}

	return items, nil
}

func Mkdir(path string) error {
	var req pathRequest
	copyPath(req.Path[:], path)
	return doBoolRequest(opMkdir, runtime.RightReadWrite, unsafe.Pointer(&req), unsafe.Sizeof(req))
}

func Remove(path string) error {
	var req pathRequest
	copyPath(req.Path[:], path)
	return doBoolRequest(opRemove, runtime.RightReadWrite, unsafe.Pointer(&req), unsafe.Sizeof(req))
}

func Rename(oldPath, newPath string) error {
	var req renameRequest
	copyPath(req.OldPath[:], oldPath)
	copyPath(req.NewPath[:], newPath)
	return doBoolRequest(opRename, runtime.RightReadWrite, unsafe.Pointer(&req), unsafe.Sizeof(req))
}

func (f *File) Read(buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}

	var req readRequest
	req.Handle = f.handle
	req.Length = uint16(len(buf))
	if req.Length > maxChunkSize {
		req.Length = maxChunkSize
	}

	var resp readResponse
	if err := doRequest(opRead, runtime.RightRead, unsafe.Pointer(&req), unsafe.Sizeof(req), unsafe.Pointer(&resp), unsafe.Sizeof(resp)); err != nil {
		return 0, err
	}
	if !resp.Success {
		return 0, errOperationFailed
	}

	n := int(resp.BytesRead)
	copy(buf, resp.Data[:n])
	if n == 0 && resp.EOF {
		return 0, nil
	}
	return n, nil
}

func (f *File) Write(buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}

	var req writeRequest
	req.Handle = f.handle
	req.Length = uint16(len(buf))
	if req.Length > maxChunkSize {
		req.Length = maxChunkSize
	}
	copy(req.Data[:], buf[:int(req.Length)])

	var resp writeResponse
	if err := doRequest(opWrite, runtime.RightReadWrite, unsafe.Pointer(&req), unsafe.Sizeof(req), unsafe.Pointer(&resp), unsafe.Sizeof(resp)); err != nil {
		return 0, err
	}
	if !resp.Success {
		return 0, errOperationFailed
	}

	return int(resp.BytesWritten), nil
}

func (f *File) Sync() error {
	var req syncRequest
	req.Handle = f.handle
	return doBoolRequest(opSync, runtime.RightReadWrite, unsafe.Pointer(&req), unsafe.Sizeof(req))
}

func (f *File) Close() error {
	var req closeRequest
	req.Handle = f.handle
	return doBoolRequest(opClose, modeRights(f.mode), unsafe.Pointer(&req), unsafe.Sizeof(req))
}

func doBoolRequest(op operation, rights uint32, req unsafe.Pointer, reqSize uintptr) error {
	var resp boolResponse
	if err := doRequest(op, rights, req, reqSize, unsafe.Pointer(&resp), unsafe.Sizeof(resp)); err != nil {
		return err
	}
	if !resp.Success {
		return errOperationFailed
	}
	return nil
}

func doRequest(op operation, rights uint32, req unsafe.Pointer, reqSize uintptr, resp unsafe.Pointer, respSize uintptr) error {
	cap, err := ensureServiceCap(rights)
	if err != nil {
		return err
	}

	replyQ := kernel.xQueueCreate(1, uint32(respSize))
	if replyQ == nil {
		return errQueueCreate
	}
	defer kernel.xQueueDelete(replyQ)

	var msg message
	msg.Op = op
	msg.ReplyQ = uint32(uintptr(unsafe.Pointer(replyQ)))
	if req != nil && reqSize > 0 {
		size := int(reqSize)
		copy(msg.Payload[:], unsafe.Slice((*byte)(req), size))
	}

	if err := runtime.CapSendRaw(cap, unsafe.Pointer(&msg), unsafe.Sizeof(msg)); err != nil {
		return err
	}

	if kernel.xQueueReceive(replyQ, resp, fsOperationTimeoutTicks) != kernel.pdTRUE {
		return errTimeout
	}

	return nil
}

func ensureServiceCap(rights uint32) (runtime.CapHandle, error) {
	// Validate typed channel registry/schema before capability acquisition.
	// The returned Entry is discarded because runtime transport still uses caps.
	if _, err := channel.OpenFS(); err != nil {
		return 0, err
	}

	if rights&runtime.RightWrite != 0 {
		if readWriteServiceCap != 0 {
			return readWriteServiceCap, nil
		}
		cap, err := runtime.RequestCap("service:fs", runtime.RightReadWrite)
		if err != nil {
			return 0, err
		}
		readWriteServiceCap = cap
		return cap, nil
	}

	if readServiceCap != 0 {
		return readServiceCap, nil
	}

	cap, err := runtime.RequestCap("service:fs", runtime.RightRead)
	if err != nil {
		return 0, err
	}

	readServiceCap = cap
	return cap, nil
}

func modeRights(mode OpenMode) uint32 {
	if mode&ModeWrite != 0 || mode&ModeCreate != 0 || mode&ModeAppend != 0 || mode&ModeTruncate != 0 {
		return runtime.RightReadWrite
	}
	return runtime.RightRead
}

var (
	errQueueCreate     = &errorString{"failed to create reply queue"}
	errTimeout         = &errorString{"filesystem request timed out"}
	errOperationFailed = &errorString{"filesystem operation failed"}
)

type errorString struct {
	s string
}

func (e *errorString) Error() string { return e.s }
