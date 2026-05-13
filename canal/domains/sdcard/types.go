//go:build tinygo

package main

type FileHandle uint16

const (
	ModeReadOnly  uint8 = 1 << 0
	ModeWriteOnly uint8 = 1 << 1
	ModeReadWrite uint8 = 1 << 2
	ModeCreate    uint8 = 1 << 3
	ModeAppend    uint8 = 1 << 4
)

type StatResponse struct {
	Exists   bool
	IsDir    bool
	Readable bool
	Writable bool
	Success  bool
	Size     uint64
	ModTime  uint64
}

type ListEntry struct {
	Name    [192]byte
	IsDir   bool
	Size    uint64
	ModTime uint64
}

type ListResponse struct {
	NumItems uint16
	Success  bool
	Items    [32]ListEntry
}

//export vTaskDelay
func vTaskDelay(xTicksToDelay uint32)
