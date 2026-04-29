//go:build tinygo

package sdcard

import (
    "unsafe"
)

// FatFS C library bindings
// Using ChaN's FatFS: http://elm-chan.org/fsw/ff/00index_e.html

// FatFS types
type FATFS struct {
    // Opaque filesystem object
    // Actual size depends on FatFS configuration
    data [512]byte
}

type FIL struct {
    // File object
    data [256]byte
}

type DIR struct {
    // Directory object
    data [128]byte
}

type FILINFO struct {
    // File information
    fsize    uint32
    fdate    uint16
    ftime    uint16
    fattrib  uint8
    fname    [13]byte  // 8.3 format
    lfname   *byte     // Long filename pointer
    lfsize   uint16
}

// FatFS result codes
const (
    FR_OK = iota
    FR_DISK_ERR
    FR_INT_ERR
    FR_NOT_READY
    FR_NO_FILE
    FR_NO_PATH
    FR_INVALID_NAME
    FR_DENIED
    FR_EXIST
    FR_INVALID_OBJECT
    FR_WRITE_PROTECTED
    FR_INVALID_DRIVE
    FR_NOT_ENABLED
    FR_NO_FILESYSTEM
    FR_MKFS_ABORTED
    FR_TIMEOUT
    FR_LOCKED
    FR_NOT_ENOUGH_CORE
    FR_TOO_MANY_OPEN_FILES
)

// FatFS file modes
const (
    FA_READ        = 0x01
    FA_WRITE       = 0x02
    FA_OPEN_EXISTING = 0x00
    FA_CREATE_NEW    = 0x04
    FA_CREATE_ALWAYS = 0x08
    FA_OPEN_ALWAYS   = 0x10
    FA_OPEN_APPEND   = 0x30
)

// Global filesystem object
var fs FATFS
var fsMounted bool

// File table (domain-local open files)
type openFile struct {
    fil      FIL
    path     [256]byte
    mode     uint8
    position uint64
    inUse    bool
}

var openFiles [32]openFile

// Mount filesystem
func mountFilesystem() error {
    if fsMounted {
        return nil
    }

    // f_mount(&fs, "", 1)  // Mount immediately
    result := f_mount(&fs, cstring(""), 1)

    if result != FR_OK {
        return fatfsError(result)
    }

    fsMounted = true
    return nil
}

// Open file
func openFile(path string, mode uint8) (FileHandle, error) {
    if !fsMounted {
        return 0, errNotMounted
    }

    // Find free file slot
    var handle FileHandle
    for i := FileHandle(0); i < 32; i++ {
        if !openFiles[i].inUse {
            handle = i
            break
        }
    }

    if openFiles[handle].inUse {
        return 0, errTooManyFiles
    }

    file := &openFiles[handle]

    // Convert mode
    fatMode := uint8(0)
    if mode & ModeReadOnly != 0 {
        fatMode |= FA_READ
    }
    if mode & ModeWriteOnly != 0 {
        fatMode |= FA_WRITE
    }
    if mode & ModeReadWrite != 0 {
        fatMode |= FA_READ | FA_WRITE
    }
    if mode & ModeCreate != 0 {
        fatMode |= FA_CREATE_ALWAYS
    }
    if mode & ModeAppend != 0 {
        fatMode |= FA_OPEN_APPEND
    }

    // Open file
    result := f_open(&file.fil, cstring(path), fatMode)

    if result != FR_OK {
        return 0, fatfsError(result)
    }

    // Store metadata
    copy(file.path[:], path)
    file.mode = mode
    file.position = 0
    file.inUse = true

    return handle, nil
}

// Close file
func closeFile(handle FileHandle) error {
    if handle >= 32 || !openFiles[handle].inUse {
        return errInvalidHandle
    }

    file := &openFiles[handle]

    // f_close(&fil)
    result := f_close(&file.fil)

    file.inUse = false

    if result != FR_OK {
        return fatfsError(result)
    }

    return nil
}

// Read from file
func readFile(handle FileHandle, buf []byte) (uint32, error) {
    if handle >= 32 || !openFiles[handle].inUse {
        return 0, errInvalidHandle
    }

    file := &openFiles[handle]

    var bytesRead uint32

    // f_read(&fil, buf, len, &br)
    result := f_read(&file.fil, unsafe.Pointer(&buf[0]), uint32(len(buf)), &bytesRead)

    if result != FR_OK {
        return 0, fatfsError(result)
    }

    file.position += uint64(bytesRead)

    return bytesRead, nil
}

// Write to file
func writeFile(handle FileHandle, data []byte) (uint32, error) {
    if handle >= 32 || !openFiles[handle].inUse {
        return 0, errInvalidHandle
    }

    file := &openFiles[handle]

    var bytesWritten uint32

    // f_write(&fil, data, len, &bw)
    result := f_write(&file.fil, unsafe.Pointer(&data[0]), uint32(len(data)), &bytesWritten)

    if result != FR_OK {
        return 0, fatfsError(result)
    }

    file.position += uint64(bytesWritten)

    return bytesWritten, nil
}

// Seek in file
func seekFile(handle FileHandle, offset int64, whence uint8) (uint64, error) {
    if handle >= 32 || !openFiles[handle].inUse {
        return 0, errInvalidHandle
    }

    file := &openFiles[handle]

    var newPos uint64

    switch whence {
    case 0: // SEEK_SET
        newPos = uint64(offset)
    case 1: // SEEK_CUR
        newPos = uint64(int64(file.position) + offset)
    case 2: // SEEK_END
        // Get file size
        size := f_size(&file.fil)
        newPos = uint64(int64(size) + offset)
    }

    // f_lseek(&fil, pos)
    result := f_lseek(&file.fil, uint32(newPos))

    if result != FR_OK {
        return 0, fatfsError(result)
    }

    file.position = newPos

    return newPos, nil
}

// Sync file (flush buffers)
func syncFile(handle FileHandle) error {
    if handle >= 32 || !openFiles[handle].inUse {
        return errInvalidHandle
    }

    file := &openFiles[handle]

    // f_sync(&fil)
    result := f_sync(&file.fil)

    if result != FR_OK {
        return fatfsError(result)
    }

    return nil
}

// Get file info
func statPath(path string) (StatResponse, error) {
    var resp StatResponse

    if !fsMounted {
        return resp, errNotMounted
    }

    var fno FILINFO

    // f_stat(path, &fno)
    result := f_stat(cstring(path), &fno)

    if result == FR_NO_FILE || result == FR_NO_PATH {
        resp.Exists = false
        resp.Success = true
        return resp, nil
    }

    if result != FR_OK {
        return resp, fatfsError(result)
    }

    resp.Exists = true
    resp.Size = uint64(fno.fsize)
    resp.ModTime = fatTimeToUnix(fno.fdate, fno.ftime)
    resp.IsDir = (fno.fattrib & 0x10) != 0
    resp.Readable = true
    resp.Writable = (fno.fattrib & 0x01) == 0 // Not read-only
    resp.Success = true

    return resp, nil
}

// List directory
func listDirectory(path string, maxItems uint16) (ListResponse, error) {
    var resp ListResponse

    if !fsMounted {
        return resp, errNotMounted
    }

    var dir DIR
    var fno FILINFO

    // f_opendir(&dir, path)
    result := f_opendir(&dir, cstring(path))
    if result != FR_OK {
        return resp, fatfsError(result)
    }
    defer f_closedir(&dir)

    // Read entries
    count := uint16(0)
    for count < maxItems && count < 32 {
        // f_readdir(&dir, &fno)
        result := f_readdir(&dir, &fno)
        if result != FR_OK {
            break
        }

        // End of directory?
        if fno.fname[0] == 0 {
            break
        }

        // Add to results
        info := &resp.Items[count]
        copy(info.Name[:], fno.fname[:])
        info.Size = uint64(fno.fsize)
        info.ModTime = fatTimeToUnix(fno.fdate, fno.ftime)
        info.IsDir = (fno.fattrib & 0x10) != 0

        count++
    }

    resp.NumItems = count
    resp.Success = true

    return resp, nil
}

// Create directory
func makeDirectory(path string) error {
    if !fsMounted {
        return errNotMounted
    }

    // f_mkdir(path)
    result := f_mkdir(cstring(path))

    if result != FR_OK {
        return fatfsError(result)
    }

    return nil
}

// Remove file or directory
func removePath(path string) error {
    if !fsMounted {
        return errNotMounted
    }

    // f_unlink(path)
    result := f_unlink(cstring(path))

    if result != FR_OK {
        return fatfsError(result)
    }

    return nil
}

// Rename file or directory
func renamePath(oldPath, newPath string) error {
    if !fsMounted {
        return errNotMounted
    }

    // f_rename(old, new)
    result := f_rename(cstring(oldPath), cstring(newPath))

    if result != FR_OK {
        return fatfsError(result)
    }

    return nil
}

// Truncate file
func truncateFile(handle FileHandle, size uint64) error {
    if handle >= 32 || !openFiles[handle].inUse {
        return errInvalidHandle
    }

    file := &openFiles[handle]

    // Seek to size
    f_lseek(&file.fil, uint32(size))

    // f_truncate(&fil)
    result := f_truncate(&file.fil)

    if result != FR_OK {
        return fatfsError(result)
    }

    return nil
}

// Convert FAT timestamp to Unix timestamp
func fatTimeToUnix(date, time uint16) uint64 {
    // FAT date: year(7) month(4) day(5)
    // FAT time: hour(5) minute(6) second(5)*2

    year := int(date>>9) + 1980
    month := int((date >> 5) & 0x0F)
    day := int(date & 0x1F)

    hour := int(time >> 11)
    minute := int((time >> 5) & 0x3F)
    second := int((time & 0x1F) * 2)

    // Simplified Unix timestamp calculation
    // (would use proper date math in production)
    return uint64(year-1970)*365*24*3600 +
        uint64(month)*30*24*3600 +
        uint64(day)*24*3600 +
        uint64(hour)*3600 +
        uint64(minute)*60 +
        uint64(second)
}

// FatFS C function bindings
//export f_mount
func f_mount(fs *FATFS, path *byte, opt uint8) uint8

//export f_open
func f_open(fp *FIL, path *byte, mode uint8) uint8

//export f_close
func f_close(fp *FIL) uint8

//export f_read
func f_read(fp *FIL, buff unsafe.Pointer, btr uint32, br *uint32) uint8

//export f_write
func f_write(fp *FIL, buff unsafe.Pointer, btw uint32, bw *uint32) uint8

//export f_lseek
func f_lseek(fp *FIL, ofs uint32) uint8

//export f_sync
func f_sync(fp *FIL) uint8

//export f_size
func f_size(fp *FIL) uint32

//export f_stat
func f_stat(path *byte, fno *FILINFO) uint8

//export f_opendir
func f_opendir(dp *DIR, path *byte) uint8

//export f_closedir
func f_closedir(dp *DIR) uint8

//export f_readdir
func f_readdir(dp *DIR, fno *FILINFO) uint8

//export f_mkdir
func f_mkdir(path *byte) uint8

//export f_unlink
func f_unlink(path *byte) uint8

//export f_rename
func f_rename(oldPath, newPath *byte) uint8

//export f_truncate
func f_truncate(fp *FIL) uint8

// Helper
func cstring(s string) *byte {
    b := []byte(s)
    b = append(b, 0)
    return &b[0]
}

func fatfsError(code uint8) error {
    switch code {
    case FR_DISK_ERR:
        return errDiskError
    case FR_NOT_READY:
        return errNotReady
    case FR_NO_FILE:
        return errFileNotFound
    case FR_NO_PATH:
        return errPathNotFound
    case FR_DENIED:
        return errAccessDenied
    case FR_EXIST:
        return errFileExists
    case FR_WRITE_PROTECTED:
        return errWriteProtected
    case FR_TOO_MANY_OPEN_FILES:
        return errTooManyFiles
    default:
        return errUnknown
    }
}

// Errors
var (
    errNotMounted      = &errorString{"filesystem not mounted"}
    errDiskError       = &errorString{"disk error"}
    errNotReady        = &errorString{"disk not ready"}
    errFileNotFound    = &errorString{"file not found"}
    errPathNotFound    = &errorString{"path not found"}
    errAccessDenied    = &errorString{"access denied"}
    errFileExists      = &errorString{"file exists"}
    errWriteProtected  = &errorString{"write protected"}
    errTooManyFiles    = &errorString{"too many open files"}
    errInvalidHandle   = &errorString{"invalid file handle"}
    errUnknown         = &errorString{"unknown error"}
)

type errorString struct{ s string }
func (e *errorString) Error() string { return e.s }
