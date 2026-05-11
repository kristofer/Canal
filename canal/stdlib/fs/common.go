package fs

import "path"

const (
	maxPathLen    = 192
	maxChunkSize  = 224
	maxListItems  = 16
	fsPayloadSize = 512
)

type OpenMode uint8

const (
	ModeRead OpenMode = 1 << iota
	ModeWrite
	ModeCreate
	ModeAppend
	ModeTruncate
)

type operation uint8

const (
	opOpen operation = iota
	opClose
	opRead
	opWrite
	opSeek
	opSync
	opStat
	opList
	opMkdir
	opRemove
	opRename
)

type message struct {
	Op      operation
	ReplyQ  uint32
	Payload [fsPayloadSize]byte
}

type openRequest struct {
	Path [maxPathLen]byte
	Mode OpenMode
}

type openResponse struct {
	Handle  uint32
	Size    uint64
	Success bool
}

type closeRequest struct {
	Handle uint32
}

type readRequest struct {
	Handle uint32
	Length uint16
}

type readResponse struct {
	BytesRead uint16
	EOF       bool
	Success   bool
	Data      [maxChunkSize]byte
}

type writeRequest struct {
	Handle uint32
	Length uint16
	Data   [maxChunkSize]byte
}

type writeResponse struct {
	BytesWritten uint16
	Success      bool
}

type syncRequest struct {
	Handle uint32
}

type statRequest struct {
	Path [maxPathLen]byte
}

type statResponse struct {
	Exists   bool
	IsDir    bool
	Readable bool
	Writable bool
	Success  bool
	Size     uint64
	ModTime  uint64
}

type listRequest struct {
	Path     [maxPathLen]byte
	MaxItems uint16
}

type listEntry struct {
	Name    [maxPathLen]byte
	IsDir   bool
	Size    uint64
	ModTime uint64
}

type listResponse struct {
	NumItems uint16
	Success  bool
	Items    [maxListItems]listEntry
}

type pathRequest struct {
	Path [maxPathLen]byte
}

type renameRequest struct {
	OldPath [maxPathLen]byte
	NewPath [maxPathLen]byte
}

type boolResponse struct {
	Success bool
}

type FileInfo struct {
	Name     string
	Exists   bool
	IsDir    bool
	Readable bool
	Writable bool
	Size     uint64
	ModTime  uint64
}

func cleanPath(name string) string {
	if name == "" {
		return "/"
	}
	if name[0] != '/' {
		name = "/" + name
	}
	return path.Clean(name)
}

func copyPath(dst []byte, name string) {
	if len(dst) == 0 {
		return
	}
	cleaned := cleanPath(name)
	n := copy(dst, cleaned)
	if n >= len(dst) {
		dst[len(dst)-1] = 0
		return
	}
	dst[n] = 0
}

func trimNull(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
