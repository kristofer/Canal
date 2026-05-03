//go:build tinygo && esp32s3 && idf

package kernel

import (
	"errors"
	"unsafe"
)

// ESP32-S3 DCache flash base — for data reads from flash.
// ICache (0x42000000) is instruction-fetch only; data loads require DCache.
// Physical flash offset N → virtual 0x3C000000 + N.
const flashXIPBase uint32 = 0x3C000000

// ELF32 magic
var elfMagic = [4]byte{0x7F, 'E', 'L', 'F'}

type elf32Header struct {
	Magic     [4]byte
	Class     uint8  // 1 = 32-bit
	Data      uint8  // 1 = little-endian
	Version   uint8
	_         [9]byte
	Type      uint16 // 2 = ET_EXEC
	Machine   uint16 // 0x5E = Xtensa
	_         uint32
	Entry     uint32 // virtual entry point
	PHOff     uint32 // program header table offset in file
	SHOff     uint32
	Flags     uint32
	EHSize    uint16
	PHEntSize uint16
	PHNum     uint16
	SHEntSize uint16
	SHNum     uint16
	SHStrNdx  uint16
}

type elf32ProgramHeader struct {
	Type   uint32 // 1 = PT_LOAD
	Offset uint32 // segment offset in file
	VAddr  uint32 // virtual load address
	PAddr  uint32
	FileSz uint32 // bytes in file image
	MemSz  uint32 // bytes in memory (FileSz..MemSz zeroed = BSS)
	Flags  uint32 // PF_X=1 PF_W=2 PF_R=4
	Align  uint32
}

const ptLOAD uint32 = 1
const pfWrite uint32 = 2 // ELF PF_W flag — segment is writable

// flashBytes returns a read-only slice of flash bytes at the given offset
// using the ESP32-S3 DCache window (0x3C000000).  This window provides
// data-side (load/store) access to flash; the separate ICache window
// (0x42000000) is instruction-fetch-only and is used for XIP text execution.
func flashBytes(offset uint32, length uint32) []byte {
	ptr := unsafe.Pointer(uintptr(flashXIPBase + offset))
	return (*[1 << 24]byte)(ptr)[:length:length]
}

// LoadDomain parses a 32-bit ELF from the given flash partition offset,
// copies all PT_LOAD segments to their virtual addresses in RAM, and
// returns the virtual entry point address.
func LoadDomain(partitionOffset uint32) (entryPoint uint32, err error) {
	// Read ELF header from memory-mapped flash.
	hdrBytes := flashBytes(partitionOffset, uint32(unsafe.Sizeof(elf32Header{})))
	hdr := (*elf32Header)(unsafe.Pointer(&hdrBytes[0]))

	if hdr.Magic != elfMagic {
		return 0, errors.New("loader: bad ELF magic")
	}
	if hdr.Class != 1 {
		return 0, errors.New("loader: not a 32-bit ELF")
	}
	if hdr.PHNum == 0 {
		return 0, errors.New("loader: no program headers")
	}

	println("[Loader] ELF at", partitionOffset, "entry:", hdr.Entry, "phdrs:", hdr.PHNum)

	phBase := partitionOffset + hdr.PHOff
	phSize := uint32(hdr.PHEntSize)

	for i := uint16(0); i < hdr.PHNum; i++ {
		phBytes := flashBytes(phBase+uint32(i)*phSize, uint32(unsafe.Sizeof(elf32ProgramHeader{})))
		ph := (*elf32ProgramHeader)(unsafe.Pointer(&phBytes[0]))

		if ph.Type != ptLOAD || ph.MemSz == 0 {
			continue
		}

		// Skip non-writable segments (XIP text/read-only data).
		// Code executes directly from flash via the ICache; attempting to
		// write to a flash-mapped virtual address causes a CPU fault.
		if ph.Flags&pfWrite == 0 {
			continue
		}

		println("[Loader] LOAD vaddr:", ph.VAddr, "filesz:", ph.FileSz, "memsz:", ph.MemSz)

		dst := (*[1 << 24]byte)(unsafe.Pointer(uintptr(ph.VAddr)))

		// Copy initialised data from flash image.
		if ph.FileSz > 0 {
			src := flashBytes(partitionOffset+ph.Offset, ph.FileSz)
			copy(dst[:ph.FileSz], src)
		}

		// Zero the BSS region (MemSz > FileSz).
		for j := ph.FileSz; j < ph.MemSz; j++ {
			dst[j] = 0
		}
	}

	return hdr.Entry, nil
}
