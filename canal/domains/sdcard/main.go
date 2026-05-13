//go:build tinygo && esp32s3

package main

import "unsafe"

type queueHandle unsafe.Pointer
type baseType int32

type fsOperation uint8

const (
	pdTRUE        baseType = 1
	qSendBack     baseType = 0
	portMAX_DELAY uint32   = 0xFFFFFFFF

	fsMaxPathLen   = 192
	fsMaxChunkSize = 224
	fsMaxListItems = 16
	fsPayloadSize  = 512

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

const (
	fsModeRead     uint8 = 1 << 0
	fsModeWrite    uint8 = 1 << 1
	fsModeCreate   uint8 = 1 << 2
	fsModeTruncate uint8 = 1 << 4
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

type fsMkdirRequest struct {
	Path [fsMaxPathLen]byte
}

type fsRemoveRequest struct {
	Path [fsMaxPathLen]byte
}

type fsRenameRequest struct {
	OldPath [fsMaxPathLen]byte
	NewPath [fsMaxPathLen]byte
}

type fsSeekRequest struct {
	Handle uint32
	Offset int64
	Whence uint8
}

type fsSeekResponse struct {
	NewOffset uint64
	Success   bool
}

type fsBoolResponse struct {
	Success bool
}

type permission struct {
	domainID   uint16
	prefix     string
	read       bool
	write      bool
	create     bool
	deletePerm bool
}

var permissions [16]permission
var permissionCount int
var serviceMsg fsMessage

//export xQueueGenericSend
func xQueueGenericSend(xQueue queueHandle, pvItemToQueue unsafe.Pointer, xTicksToWait uint32, xCopyPosition baseType) baseType

//export xQueueReceive
func xQueueReceive(xQueue queueHandle, pvBuffer unsafe.Pointer, xTicksToWait uint32) baseType

//export xQueueGenericCreate
func xQueueGenericCreate(uxQueueLength uint32, uxItemSize uint32, ucQueueType baseType) queueHandle

//export canal_set_fs_service_queue
func canal_set_fs_service_queue(queue unsafe.Pointer)

//export canal_sdcard_init
func canal_sdcard_init() int32

//export domain_entry
func domain_entry(param unsafe.Pointer) {
	_ = param
	println("[SDCard] Service starting...")

	addPermission(1, "/", true, false, false, false)
	addPermission(2, "/", true, true, true, false)

	serviceQ := xQueueGenericCreate(8, uint32(unsafe.Sizeof(fsMessage{})), 0)
	if serviceQ == nil {
		println("[SDCard] Failed to create service queue")
		for {
			vTaskDelay(10000)
		}
	}

	for {
		if ret := canal_sdcard_init(); ret != 0 {
			println("[SDCard] SPI/SD init failed")
			vTaskDelay(3000)
			continue
		}

		if err := mountFilesystem(); err != nil {
			println("[SDCard] Mount failed")
			vTaskDelay(3000)
			continue
		}

		println("[SDCard] Filesystem mounted")
		canal_set_fs_service_queue(unsafe.Pointer(serviceQ))
		println("[SDCard] service:fs registered")
		break
	}

	for {
		if xQueueReceive(serviceQ, unsafe.Pointer(&serviceMsg), portMAX_DELAY) != pdTRUE {
			continue
		}
		handleFSRequest(&serviceMsg)
	}
}

func main() {
	domain_entry(nil)
}

func initSPI() error {
	// Handled inline in domain_entry via canal_sdcard_init().
	return nil
}

func handleFSRequest(msg *fsMessage) {
	switch msg.Op {
	case fsOpOpen:
		handleOpen(msg)
	case fsOpClose:
		handleClose(msg)
	case fsOpRead:
		handleRead(msg)
	case fsOpWrite:
		handleWrite(msg)
	case fsOpSeek:
		handleSeek(msg)
	case fsOpSync:
		handleSync(msg)
	case fsOpStat:
		handleStat(msg)
	case fsOpList:
		handleList(msg)
	case fsOpMkdir:
		handleMkdir(msg)
	case fsOpRemove:
		handleRemove(msg)
	case fsOpRename:
		handleRename(msg)
	default:
		resp := fsBoolResponse{Success: false}
		sendReply(msg.ReplyQ, unsafe.Pointer(&resp), unsafe.Sizeof(resp))
	}
}

func handleOpen(msg *fsMessage) {
	req := (*fsOpenRequest)(unsafe.Pointer(&msg.Payload[0]))
	path := trimNull(req.Path[:])
	mode := mapMode(req.Mode)
	h, err := openFile(path, mode)
	resp := fsOpenResponse{Success: err == nil, Handle: uint32(h)}
	if err == nil {
		if st, statErr := statPath(path); statErr == nil {
			resp.Size = st.Size
		}
	}
	sendReply(msg.ReplyQ, unsafe.Pointer(&resp), unsafe.Sizeof(resp))
}

func handleClose(msg *fsMessage) {
	req := (*fsCloseRequest)(unsafe.Pointer(&msg.Payload[0]))
	err := closeFile(FileHandle(req.Handle))
	resp := fsBoolResponse{Success: err == nil}
	sendReply(msg.ReplyQ, unsafe.Pointer(&resp), unsafe.Sizeof(resp))
}

func handleRead(msg *fsMessage) {
	req := (*fsReadRequest)(unsafe.Pointer(&msg.Payload[0]))
	resp := fsReadResponse{}
	length := int(req.Length)
	if length > fsMaxChunkSize {
		length = fsMaxChunkSize
	}
	n, err := readFile(FileHandle(req.Handle), resp.Data[:length])
	if err == nil {
		resp.BytesRead = uint16(n)
		resp.EOF = uint32(length) != n
		resp.Success = true
	}
	sendReply(msg.ReplyQ, unsafe.Pointer(&resp), unsafe.Sizeof(resp))
}

func handleWrite(msg *fsMessage) {
	req := (*fsWriteRequest)(unsafe.Pointer(&msg.Payload[0]))
	resp := fsWriteResponse{}
	length := int(req.Length)
	if length > fsMaxChunkSize {
		length = fsMaxChunkSize
	}
	n, err := writeFile(FileHandle(req.Handle), req.Data[:length])
	if err == nil {
		resp.BytesWritten = uint16(n)
		resp.Success = true
	}
	sendReply(msg.ReplyQ, unsafe.Pointer(&resp), unsafe.Sizeof(resp))
}

func handleSeek(msg *fsMessage) {
	req := (*fsSeekRequest)(unsafe.Pointer(&msg.Payload[0]))
	resp := fsSeekResponse{}
	off, err := seekFile(FileHandle(req.Handle), req.Offset, req.Whence)
	if err == nil {
		resp.NewOffset = off
		resp.Success = true
	}
	sendReply(msg.ReplyQ, unsafe.Pointer(&resp), unsafe.Sizeof(resp))
}

func handleSync(msg *fsMessage) {
	req := (*fsSyncRequest)(unsafe.Pointer(&msg.Payload[0]))
	err := syncFile(FileHandle(req.Handle))
	resp := fsBoolResponse{Success: err == nil}
	sendReply(msg.ReplyQ, unsafe.Pointer(&resp), unsafe.Sizeof(resp))
}

func handleStat(msg *fsMessage) {
	req := (*fsStatRequest)(unsafe.Pointer(&msg.Payload[0]))
	path := trimNull(req.Path[:])
	st, err := statPath(path)
	resp := fsStatResponse{Success: false}
	if err == nil {
		resp.Exists = st.Exists
		resp.IsDir = st.IsDir
		resp.Readable = st.Readable
		resp.Writable = st.Writable
		resp.Success = st.Success
		resp.Size = st.Size
		resp.ModTime = st.ModTime
	}
	sendReply(msg.ReplyQ, unsafe.Pointer(&resp), unsafe.Sizeof(resp))
}

func handleList(msg *fsMessage) {
	req := (*fsListRequest)(unsafe.Pointer(&msg.Payload[0]))
	path := trimNull(req.Path[:])
	lr, err := listDirectory(path, req.MaxItems)
	resp := fsListResponse{Success: false}
	if err == nil {
		resp.NumItems = lr.NumItems
		if resp.NumItems > fsMaxListItems {
			resp.NumItems = fsMaxListItems
		}
		resp.Success = true
		for i := uint16(0); i < resp.NumItems; i++ {
			copy(resp.Items[i].Name[:], lr.Items[i].Name[:])
			resp.Items[i].IsDir = lr.Items[i].IsDir
			resp.Items[i].Size = lr.Items[i].Size
			resp.Items[i].ModTime = lr.Items[i].ModTime
		}
	}
	sendReply(msg.ReplyQ, unsafe.Pointer(&resp), unsafe.Sizeof(resp))
}

func handleMkdir(msg *fsMessage) {
	req := (*fsMkdirRequest)(unsafe.Pointer(&msg.Payload[0]))
	err := makeDirectory(trimNull(req.Path[:]))
	resp := fsBoolResponse{Success: err == nil}
	sendReply(msg.ReplyQ, unsafe.Pointer(&resp), unsafe.Sizeof(resp))
}

func handleRemove(msg *fsMessage) {
	req := (*fsRemoveRequest)(unsafe.Pointer(&msg.Payload[0]))
	err := removePath(trimNull(req.Path[:]))
	resp := fsBoolResponse{Success: err == nil}
	sendReply(msg.ReplyQ, unsafe.Pointer(&resp), unsafe.Sizeof(resp))
}

func handleRename(msg *fsMessage) {
	req := (*fsRenameRequest)(unsafe.Pointer(&msg.Payload[0]))
	err := renamePath(trimNull(req.OldPath[:]), trimNull(req.NewPath[:]))
	resp := fsBoolResponse{Success: err == nil}
	sendReply(msg.ReplyQ, unsafe.Pointer(&resp), unsafe.Sizeof(resp))
}

func sendReply(replyQ uint32, resp unsafe.Pointer, size uintptr) {
	q := queueHandle(unsafe.Pointer(uintptr(replyQ)))
	xQueueGenericSend(q, resp, portMAX_DELAY, qSendBack)
}

func mapMode(fsMode uint8) uint8 {
	mode := uint8(0)
	if fsMode&fsModeRead != 0 {
		mode |= ModeReadOnly
	}
	if fsMode&fsModeWrite != 0 {
		mode |= ModeWriteOnly
	}
	if fsMode&fsModeRead != 0 && fsMode&fsModeWrite != 0 {
		mode |= ModeReadWrite
	}
	if fsMode&fsModeCreate != 0 {
		mode |= ModeCreate
	}
	if fsMode&fsModeTruncate != 0 {
		mode |= ModeCreate
	}
	return mode
}

func addPermission(domainID uint16, prefix string, read, write, create, deletePerm bool) {
	if permissionCount >= len(permissions) {
		return
	}
	permissions[permissionCount] = permission{
		domainID:   domainID,
		prefix:     prefix,
		read:       read,
		write:      write,
		create:     create,
		deletePerm: deletePerm,
	}
	permissionCount++
}

func checkPermission(domainID uint16, path string, needRead, needWrite, needCreate, needDelete bool) bool {
	for i := 0; i < permissionCount; i++ {
		p := permissions[i]
		if p.domainID != domainID {
			continue
		}
		if !hasPrefix(path, p.prefix) {
			continue
		}
		if needRead && !p.read {
			continue
		}
		if needWrite && !p.write {
			continue
		}
		if needCreate && !p.create {
			continue
		}
		if needDelete && !p.deletePerm {
			continue
		}
		return true
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	if len(prefix) == 0 {
		return true
	}
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if s[i] != prefix[i] {
			return false
		}
	}
	return true
}

func trimNull(b []byte) string {
	for i := range b {
		if b[i] == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
