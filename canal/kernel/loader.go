//go:build tinygo && esp32s3 && idf

package kernel

import (
	"unsafe"
)

type staticError string

func (e staticError) Error() string { return string(e) }

const (
	errFlashReadHeader      staticError = "loader: flash read header failed"
	errBadELFMagic          staticError = "loader: bad ELF magic"
	errNotELF32             staticError = "loader: not a 32-bit ELF"
	errNoProgramHeaders     staticError = "loader: no program headers"
	errZeroEntry            staticError = "loader: zero entry point"
	errEntryInIRAM          staticError = "loader: entry in IRAM — build domain with -ldflags \"-e domain_entry\""
	errEntryNotXIP          staticError = "loader: entry not in flash XIP region"
	errReadProgramHeader    staticError = "loader: flash read program header failed"
	errReadSegmentData      staticError = "loader: flash read segment failed"
	errInvalidSegmentSizes  staticError = "loader: invalid segment size (filesz > memsz)"
	errWritableInFlash      staticError = "loader: writable segment in flash-mapped readonly region"
	errBSSInFlash           staticError = "loader: unsupported BSS in flash-mapped segment"
	errNoDomainRAMWindow    staticError = "loader: no RAM window configured for domain"
	errRAMSegmentOutOfRange staticError = "loader: RAM PT_LOAD segment outside domain window"
	errMmapExecFailed       staticError = "loader: spi_flash_mmap INST failed"
	errEntryOutsideXIPSeg   staticError = "loader: entry point not covered by any XIP segment"
)

//export canal_flash_read
func canal_flash_read(offset uint32, out unsafe.Pointer, length uint32) int32

//export canal_domain_ram
func canal_domain_ram(name *byte, sizeOut *uint32) unsafe.Pointer

//export canal_mmap_exec
func canal_mmap_exec(flashOffset uint32, size uint32, handleOut *uint32) uint32

//export canal_munmap_exec
func canal_munmap_exec(handle uint32)

// ELF32 magic
var elfMagic = [4]byte{0x7F, 'E', 'L', 'F'}

type elf32Header struct {
	Magic     [4]byte
	Class     uint8 // 1 = 32-bit
	Data      uint8 // 1 = little-endian
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

const (
	pfExec  uint32 = 1 << 0
	pfWrite uint32 = 1 << 1
	pfRead  uint32 = 1 << 2
)

func isFlashMappedAddr(addr uint32) bool {
	if addr >= 0x3C000000 && addr < 0x3E000000 {
		return true
	}
	if addr >= 0x42000000 && addr < 0x44000000 {
		return true
	}
	return false
}

func domainRAMWindow(name string) (base uint32, size uint32, ok bool) {
	// Ask the C bridge for this domain's statically allocated internal DRAM buffer.
	// The buffers are declared as static arrays in bridge_main.c so IDF's linker
	// places them at fixed addresses in .dram0.bss. We read those addresses at
	// runtime instead of hardcoding them, avoiding conflicts with the IDF heap.
	var sz uint32
	ptr := canal_domain_ram(cstring(name), &sz)
	if ptr == nil || sz == 0 {
		return 0, 0, false
	}
	return uint32(uintptr(ptr)), sz, true
}

// LoadDomain parses a 32-bit ELF from the given flash partition offset,
// copies all PT_LOAD segments to their virtual addresses in RAM, and
// returns the virtual entry point address.
func LoadDomain(partitionOffset uint32, domainName string) (entryPoint uint32, err error) {
	ramBase, ramSize, ok := domainRAMWindow(domainName)
	if !ok {
		return 0, errNoDomainRAMWindow
	}
	ramLimit := ramBase + ramSize

	// Read ELF header directly from flash through IDF to avoid MMU mapping limits.
	var hdr elf32Header
	if canal_flash_read(partitionOffset, unsafe.Pointer(&hdr), uint32(unsafe.Sizeof(hdr))) != 0 {
		return 0, errFlashReadHeader
	}

	if hdr.Magic != elfMagic {
		return 0, errBadELFMagic
	}
	if hdr.Class != 1 {
		return 0, errNotELF32
	}
	if hdr.PHNum == 0 {
		return 0, errNoProgramHeaders
	}
	if hdr.Entry == 0 {
		return 0, errZeroEntry
	}

	// Reject firmware ELFs whose entry point falls in kernel IRAM (0x40000000–0x4FFFFFFF).
	// Domain binaries must be built with -ldflags "-e domain_entry" so their entry is in
	// the flash XIP window (0x42000000+). An IRAM entry means this is a full firmware
	// image, not a domain binary — loading it would overwrite the running kernel.
	if hdr.Entry >= 0x40000000 && hdr.Entry < 0x42000000 {
		println("[Loader] rejected: entry", hdr.Entry, "is in IRAM (firmware image, not domain)")
		return 0, errEntryInIRAM
	}
	if !isFlashMappedAddr(hdr.Entry) {
		return 0, errEntryNotXIP
	}

	println("[Loader] ELF at", partitionOffset, "entry:", hdr.Entry, "phdrs:", hdr.PHNum)

	phBase := partitionOffset + hdr.PHOff
	phSize := uint32(hdr.PHEntSize)

	for i := uint16(0); i < hdr.PHNum; i++ {
		var ph elf32ProgramHeader
		if canal_flash_read(phBase+uint32(i)*phSize, unsafe.Pointer(&ph), uint32(unsafe.Sizeof(ph))) != 0 {
			return 0, errReadProgramHeader
		}

		if ph.Type != ptLOAD || ph.MemSz == 0 {
			continue
		}
		if ph.FileSz > ph.MemSz {
			return 0, errInvalidSegmentSizes
		}

		println("[Loader] LOAD vaddr:", ph.VAddr, "filesz:", ph.FileSz, "memsz:", ph.MemSz)

		// XIP text/rodata segments already execute directly from flash mapping.
		if isFlashMappedAddr(ph.VAddr) {
			// NOLOAD padding sections (e.g. .text_dummy) have filesz=0 — nothing
			// to copy or validate, the XIP mapping already covers them.
			if ph.FileSz == 0 {
				println("[Loader] XIP NOLOAD segment; skipping")
				continue
			}
			if (ph.Flags & pfWrite) != 0 {
				return 0, errWritableInFlash
			}
			if ph.MemSz != ph.FileSz {
				return 0, errBSSInFlash
			}
			println("[Loader] XIP segment; skipping RAM copy")
			continue
		}

		// IRAM range (0x40000000–0x40FFFFFF) is owned entirely by the kernel.
		// Domain ELFs may have an .iram segment (interrupt stubs, etc.) linked
		// there by TinyGo's linker script. Skip it: the kernel's version is
		// already loaded and the domain must not overwrite it.
		if ph.VAddr >= 0x40000000 && ph.VAddr < 0x41000000 {
			println("[Loader] IRAM segment; owned by kernel, skipping")
			continue
		}

		segEnd := ph.VAddr + ph.MemSz
		if segEnd < ph.VAddr {
			return 0, errRAMSegmentOutOfRange
		}
		if ph.VAddr < ramBase || segEnd > ramLimit {
			return 0, errRAMSegmentOutOfRange
		}

		dst := (*[1 << 16]byte)(unsafe.Pointer(uintptr(ph.VAddr)))

		if ph.FileSz > 0 {
			if canal_flash_read(partitionOffset+ph.Offset, unsafe.Pointer(&dst[0]), ph.FileSz) != 0 {
				return 0, errReadSegmentData
			}
		}

		for j := ph.FileSz; j < ph.MemSz; j++ {
			dst[j] = 0
		}
	}

	return hdr.Entry, nil
}

// LoadDomainMapped is like LoadDomain but additionally maps the domain's
// executable flash partition into the IROM XIP window via spi_flash_mmap and
// returns the corrected (mapped) entry point address and the mmap handle.
// The caller must call canal_munmap_exec(handle) when the domain exits.
func LoadDomainMapped(partitionOffset uint32, partitionSize uint32, domainName string) (entryPoint uint32, mmapHandle uint32, err error) {
	ramBase, ramSize, ok := domainRAMWindow(domainName)
	if !ok {
		return 0, 0, errNoDomainRAMWindow
	}
	ramLimit := ramBase + ramSize

	var hdr elf32Header
	if canal_flash_read(partitionOffset, unsafe.Pointer(&hdr), uint32(unsafe.Sizeof(hdr))) != 0 {
		return 0, 0, errFlashReadHeader
	}
	if hdr.Magic != elfMagic {
		return 0, 0, errBadELFMagic
	}
	if hdr.Class != 1 {
		return 0, 0, errNotELF32
	}
	if hdr.PHNum == 0 {
		return 0, 0, errNoProgramHeaders
	}
	if hdr.Entry == 0 {
		return 0, 0, errZeroEntry
	}
	if hdr.Entry >= 0x40000000 && hdr.Entry < 0x42000000 {
		println("[Loader] rejected: entry", hdr.Entry, "is in IRAM (firmware image, not domain)")
		return 0, 0, errEntryInIRAM
	}
	if !isFlashMappedAddr(hdr.Entry) {
		return 0, 0, errEntryNotXIP
	}

	println("[Loader] ELF at", partitionOffset, "entry:", hdr.Entry, "phdrs:", hdr.PHNum)

	phBase := partitionOffset + hdr.PHOff
	phSize := uint32(hdr.PHEntSize)

	// Find the IROM segment that covers the entry point and load RAM segments.
	// We need the IROM segment's file offset and linked VAddr to compute the
	// real entry after remapping.
	var iromFileOff uint32 // file offset of the IROM (.text) segment
	var iromVAddr uint32   // linked VAddr of the IROM segment
	var iromFileSz uint32  // file size of the IROM segment
	iromFound := false

	for i := uint16(0); i < hdr.PHNum; i++ {
		var ph elf32ProgramHeader
		if canal_flash_read(phBase+uint32(i)*phSize, unsafe.Pointer(&ph), uint32(unsafe.Sizeof(ph))) != 0 {
			return 0, 0, errReadProgramHeader
		}
		if ph.Type != ptLOAD || ph.MemSz == 0 {
			continue
		}
		if ph.FileSz > ph.MemSz {
			return 0, 0, errInvalidSegmentSizes
		}

		println("[Loader] LOAD vaddr:", ph.VAddr, "filesz:", ph.FileSz, "memsz:", ph.MemSz)

		if isFlashMappedAddr(ph.VAddr) {
			if ph.FileSz == 0 {
				println("[Loader] XIP NOLOAD segment; skipping")
				continue
			}
			if (ph.Flags & pfWrite) != 0 {
				return 0, 0, errWritableInFlash
			}
			if ph.MemSz != ph.FileSz {
				return 0, 0, errBSSInFlash
			}
			// Check if this IROM segment covers the entry point.
			if !iromFound && hdr.Entry >= ph.VAddr && hdr.Entry < ph.VAddr+ph.FileSz {
				iromFileOff = ph.Offset
				iromVAddr = ph.VAddr
				iromFileSz = ph.FileSz
				iromFound = true
				println("[Loader] IROM segment covers entry; file_off:", ph.Offset)
			}
			println("[Loader] XIP segment; will be remapped")
			continue
		}

		if ph.VAddr >= 0x40000000 && ph.VAddr < 0x41000000 {
			println("[Loader] IRAM segment; owned by kernel, skipping")
			continue
		}

		// RAM segment — copy/zero into domain buffer.
		segEnd := ph.VAddr + ph.MemSz
		if segEnd < ph.VAddr || ph.VAddr < ramBase || segEnd > ramLimit {
			return 0, 0, errRAMSegmentOutOfRange
		}
		dst := (*[1 << 16]byte)(unsafe.Pointer(uintptr(ph.VAddr)))
		if ph.FileSz > 0 {
			if canal_flash_read(partitionOffset+ph.Offset, unsafe.Pointer(&dst[0]), ph.FileSz) != 0 {
				return 0, 0, errReadSegmentData
			}
		}
		for j := ph.FileSz; j < ph.MemSz; j++ {
			dst[j] = 0
		}
	}

	if !iromFound {
		return 0, 0, errEntryOutsideXIPSeg
	}

	// Map the domain's flash partition as executable (IROM).
	// spi_flash_mmap requires the offset to be on a page boundary (64KB).
	// partitionOffset is always page-aligned (0x100000, 0x180000, etc.).
	// We map from the start of the partition; the IROM file content within it
	// starts at iromFileOff. The mapping covers the whole partition.
	var handle uint32
	mappedBase := canal_mmap_exec(partitionOffset, partitionSize, &handle)
	if mappedBase == 0 {
		return 0, 0, errMmapExecFailed
	}
	println("[Loader] IROM mapped at virtual", mappedBase, "handle:", handle)

	// Compute the real entry: mappedBase + file_offset_of_entry.
	// file_offset_of_entry = iromFileOff + (entry_linked_vaddr - iromVAddr)
	entryFileOffset := iromFileOff + (hdr.Entry - iromVAddr)
	realEntry := mappedBase + entryFileOffset
	println("[Loader] real entry:", realEntry, "(linked:", hdr.Entry, ")")
	_ = iromFileSz

	return realEntry, handle, nil
}
