//go:build tinygo && esp32s3

package main

import (
    "runtime"
    "domains/sdcard"
)

func main() {
    println("[SDCard] Service starting...")

    // Initialize SPI hardware
    err := sdcard.initSPI()
    if err != nil {
        println("[SDCard] SPI init failed:", err.Error())
        return
    }

    println("[SDCard] SPI initialized")

    // Request hardware capability
    spiCap, err := runtime.RequestCap("device:spi2", runtime.RightReadWrite)
    if err != nil {
        println("[SDCard] Failed to get SPI capability")
        return
    }

    println("[SDCard] SPI capability acquired")

    // Mount filesystem
    err = sdcard.mountFilesystem()
    if err != nil {
        println("[SDCard] Mount failed:", err.Error())
        return
    }

    println("[SDCard] Filesystem mounted")

    // Set up permissions
    // Logger domain: can write to /logs
    sdcard.addPermission(3, "/logs", true, true, true, false)

    // HTTP server: can read /www
    sdcard.addPermission(4, "/www", true, false, false, false)

    // Config manager: can read/write /etc
    sdcard.addPermission(5, "/etc", true, true, true, false)

    println("[SDCard] Permissions configured")

    // Expose filesystem service capability
    serviceCap, err := runtime.ExposeCap("service:fs", runtime.RightReadWrite)
    if err != nil {
        println("[SDCard] Failed to expose service")
        return
    }

    println("[SDCard] Service ready")

    // Main service loop
    var msg sdcard.FSMessage
    requestCount := uint32(0)

    for {
        err := runtime.CapRecv(serviceCap, &msg)
        if err != nil {
            println("[SDCard] Recv error:", err.Error())
            continue
        }

        requestCount++

        // Extract domain ID from message
        domainID := msg.DomainID

        // Dispatch operation
        switch msg.Op {
        case sdcard.OpMount:
            handleMount(&msg, domainID)

        case sdcard.OpOpen:
            handleOpen(&msg, domainID)

        case sdcard.OpClose:
            handleClose(&msg, domainID)

        case sdcard.OpRead:
            handleRead(&msg, domainID)

        case sdcard.OpWrite:
            handleWrite(&msg, domainID)

        case sdcard.OpSeek:
            handleSeek(&msg, domainID)

        case sdcard.OpSync:
            handleSync(&msg, domainID)

        case sdcard.OpStat:
            handleStat(&msg, domainID)

        case sdcard.OpList:
            handleList(&msg, domainID)

        case sdcard.OpMkdir:
            handleMkdir(&msg, domainID)

        case sdcard.OpRemove:
            handleRemove(&msg, domainID)

        case sdcard.OpRename:
            handleRename(&msg, domainID)

        case sdcard.OpTruncate:
            handleTruncate(&msg, domainID)

        default:
            println("[SDCard] Unknown op:", msg.Op)
        }

        // Periodic stats
        if requestCount % 100 == 0 {
            println("[SDCard] Requests:", requestCount)
            runtime.GC()
        }
    }
}

func handleMount(msg *sdcard.FSMessage, domainID kernel.DomainID) {
    var req sdcard.MountRequest
    unmarshal(msg.Payload[:], &req)

    println("[SDCard] Mount request from domain", domainID)

    // Already mounted in main(), just return info
    var resp sdcard.MountResponse
    resp.Success = true
    resp.TotalBytes = 8 * 1024 * 1024 * 1024 // 8GB (would query card)
    resp.FreeBytes = 4 * 1024 * 1024 * 1024  // 4GB (would query filesystem)
    resp.ClusterSize = 4096

    sendResponse(msg.ReplyQ, &resp)
}

func handleOpen(msg *sdcard.FSMessage, domainID kernel.DomainID) {
    var req sdcard.OpenRequest
    unmarshal(msg.Payload[:], &req)

    path := nullTermString(req.Path[:])
    println("[SDCard] Open:", path, "from domain", domainID)

    // Check permissions
    needRead := (req.Mode & sdcard.ModeReadOnly) != 0 || (req.Mode & sdcard.ModeReadWrite) != 0
    needWrite := (req.Mode & sdcard.ModeWriteOnly) != 0 || (req.Mode & sdcard.ModeReadWrite) != 0
    needCreate := (req.Mode & sdcard.ModeCreate) != 0

    if !sdcard.checkPermission(domainID, path, needRead, needWrite, needCreate, false) {
        println("[SDCard] Permission denied")
        var resp sdcard.OpenResponse
        resp.Success = false
        sendResponse(msg.ReplyQ, &resp)
        return
    }

    // Open file
    handle, err := sdcard.openFile(path, req.Mode)

    var resp sdcard.OpenResponse
    if err == nil {
        resp.Handle = handle
        resp.Success = true

        // Get size
        stat, _ := sdcard.statPath(path)
        resp.Size = stat.Size

        println("[SDCard] Opened, handle:", handle)
    } else {
        println("[SDCard] Open failed:", err.Error())
        resp.Success = false
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleClose(msg *sdcard.FSMessage, domainID kernel.DomainID) {
    var req sdcard.CloseRequest
    unmarshal(msg.Payload[:], &req)

    println("[SDCard] Close handle", req.Handle, "from domain", domainID)

    err := sdcard.closeFile(req.Handle)

    var resp sdcard.CloseResponse
    resp.Success = (err == nil)

    sendResponse(msg.ReplyQ, &resp)
}

func handleRead(msg *sdcard.FSMessage, domainID kernel.DomainID) {
    var req sdcard.ReadRequest
    unmarshal(msg.Payload[:], &req)

    println("[SDCard] Read", req.Length, "bytes from handle", req.Handle)

    var resp sdcard.ReadResponse

    bytesRead, err := sdcard.readFile(req.Handle, resp.Data[:req.Length])

    if err == nil {
        resp.BytesRead = bytesRead
        resp.EOF = (bytesRead < req.Length)
        resp.Success = true
    } else {
        println("[SDCard] Read error:", err.Error())
        resp.Success = false
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleWrite(msg *sdcard.FSMessage, domainID kernel.DomainID) {
    var req sdcard.WriteRequest
    unmarshal(msg.Payload[:], &req)

    println("[SDCard] Write", req.Length, "bytes to handle", req.Handle)

    var resp sdcard.WriteResponse

    bytesWritten, err := sdcard.writeFile(req.Handle, req.Data[:req.Length])

    if err == nil {
        resp.BytesWritten = bytesWritten
        resp.Success = true
    } else {
        println("[SDCard] Write error:", err.Error())
        resp.Success = false
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleSeek(msg *sdcard.FSMessage, domainID kernel.DomainID) {
    var req sdcard.SeekRequest
    unmarshal(msg.Payload[:], &req)

    println("[SDCard] Seek handle", req.Handle, "offset", req.Offset)

    var resp sdcard.SeekResponse

    newOffset, err := sdcard.seekFile(req.Handle, req.Offset, req.Whence)

    if err == nil {
        resp.NewOffset = newOffset
        resp.Success = true
    } else {
        resp.Success = false
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleSync(msg *sdcard.FSMessage, domainID kernel.DomainID) {
    var handle sdcard.FileHandle
    unmarshal(msg.Payload[:], &handle)

    println("[SDCard] Sync handle", handle)

    err := sdcard.syncFile(handle)

    var resp [1]byte
    if err == nil {
        resp[0] = 1
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleStat(msg *sdcard.FSMessage, domainID kernel.DomainID) {
    var req sdcard.StatRequest
    unmarshal(msg.Payload[:], &req)

    path := nullTermString(req.Path[:])
    println("[SDCard] Stat:", path, "from domain", domainID)

    // Check read permission
    if !sdcard.checkPermission(domainID, path, true, false, false, false) {
        println("[SDCard] Permission denied")
        var resp sdcard.StatResponse
        resp.Success = false
        sendResponse(msg.ReplyQ, &resp)
        return
    }

    resp, err := sdcard.statPath(path)

    if err != nil {
        println("[SDCard] Stat error:", err.Error())
        resp.Success = false
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleList(msg *sdcard.FSMessage, domainID kernel.DomainID) {
    var req sdcard.ListRequest
    unmarshal(msg.Payload[:], &req)

    path := nullTermString(req.Path[:])
    println("[SDCard] List:", path, "from domain", domainID)

    // Check read permission
    if !sdcard.checkPermission(domainID, path, true, false, false, false) {
        println("[SDCard] Permission denied")
        var resp sdcard.ListResponse
        resp.Success = false
        sendResponse(msg.ReplyQ, &resp)
        return
    }

    resp, err := sdcard.listDirectory(path, req.MaxItems)

    if err != nil {
        println("[SDCard] List error:", err.Error())
        resp.Success = false
    } else {
        println("[SDCard] Found", resp.NumItems, "items")
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleMkdir(msg *sdcard.FSMessage, domainID kernel.DomainID) {
    var req sdcard.MkdirRequest
    unmarshal(msg.Payload[:], &req)

    path := nullTermString(req.Path[:])
    println("[SDCard] Mkdir:", path, "from domain", domainID)

    // Check create permission
    if !sdcard.checkPermission(domainID, path, false, false, true, false) {
        println("[SDCard] Permission denied")
        var resp sdcard.MkdirResponse
        resp.Success = false
        sendResponse(msg.ReplyQ, &resp)
        return
    }

    err := sdcard.makeDirectory(path)

    var resp sdcard.MkdirResponse
    resp.Success = (err == nil)

    if err != nil {
        println("[SDCard] Mkdir error:", err.Error())
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleRemove(msg *sdcard.FSMessage, domainID kernel.DomainID) {
    var req sdcard.RemoveRequest
    unmarshal(msg.Payload[:], &req)

    path := nullTermString(req.Path[:])
    println("[SDCard] Remove:", path, "from domain", domainID)

    // Check delete permission
    if !sdcard.checkPermission(domainID, path, false, false, false, true) {
        println("[SDCard] Permission denied")
        var resp sdcard.RemoveResponse
        resp.Success = false
        sendResponse(msg.ReplyQ, &resp)
        return
    }

    err := sdcard.removePath(path)

    var resp sdcard.RemoveResponse
    resp.Success = (err == nil)

    if err != nil {
        println("[SDCard] Remove error:", err.Error())
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleRename(msg *sdcard.FSMessage, domainID kernel.DomainID) {
    var req sdcard.RenameRequest
    unmarshal(msg.Payload[:], &req)

    oldPath := nullTermString(req.OldPath[:])
    newPath := nullTermString(req.NewPath[:])
    println("[SDCard] Rename:", oldPath, "->", newPath)

    // Check permissions on both paths
    canOld := sdcard.checkPermission(domainID, oldPath, false, true, false, false)
    canNew := sdcard.checkPermission(domainID, newPath, false, false, true, false)

    if !canOld || !canNew {
        println("[SDCard] Permission denied")
        var resp sdcard.RenameResponse
        resp.Success = false
        sendResponse(msg.ReplyQ, &resp)
        return
    }

    err := sdcard.renamePath(oldPath, newPath)

    var resp sdcard.RenameResponse
    resp.Success = (err == nil)

    sendResponse(msg.ReplyQ, &resp)
}

func handleTruncate(msg *sdcard.FSMessage, domainID kernel.DomainID) {
    var req sdcard.TruncateRequest
    unmarshal(msg.Payload[:], &req)

    println("[SDCard] Truncate handle", req.Handle, "to", req.Size)

    err := sdcard.truncateFile(req.Handle, req.Size)

    var resp sdcard.TruncateResponse
    resp.Success = (err == nil)

    sendResponse(msg.ReplyQ, &resp)
}

// Helpers
func sendResponse(replyQ uint32, data interface{}) { /* ... */ }
func unmarshal(buf []byte, v interface{}) { /* ... */ }
func nullTermString(b []byte) string { /* ... */ }
