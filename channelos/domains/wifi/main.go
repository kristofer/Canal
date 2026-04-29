//go:build tinygo && esp32s3

package main

import (
    "unsafe"
    "runtime"
    "kernel"
    "domains/wifi"
)

// WiFi service domain - handles all WiFi operations
// Runs with elevated privileges to access WiFi hardware

func main() {
    println("[WiFi] Service starting...")

    // Initialize WiFi driver
    err := wifi.driverInit()
    if err != nil {
        println("[WiFi] Failed to initialize driver:", err.Error())
        return
    }

    println("[WiFi] Driver initialized")

    // Request capability to WiFi hardware
    hwCap, err := runtime.RequestCap("device:wifi", runtime.RightReadWrite)
    if err != nil {
        println("[WiFi] Failed to get hardware capability")
        return
    }

    println("[WiFi] Hardware capability acquired")

    // Expose WiFi service capability
    serviceCap, err := runtime.ExposeCap("service:wifi", runtime.RightReadWrite)
    if err != nil {
        println("[WiFi] Failed to expose service")
        return
    }

    println("[WiFi] Service exposed, ready for requests")

    // Main service loop
    var msg wifi.WiFiMessage
    requestCount := uint32(0)

    for {
        // Receive request from capability channel
        err := runtime.CapRecv(serviceCap, &msg)
        if err != nil {
            println("[WiFi] Recv error:", err.Error())
            continue
        }

        requestCount++

        // Dispatch based on operation
        switch msg.Op {
        case wifi.OpScan:
            handleScan(&msg)

        case wifi.OpConnect:
            handleConnect(&msg)

        case wifi.OpDisconnect:
            handleDisconnect(&msg)

        case wifi.OpStatus:
            handleStatus(&msg)

        case wifi.OpCreateSocket:
            handleCreateSocket(&msg)

        case wifi.OpSocketSend:
            handleSocketSend(&msg)

        case wifi.OpSocketRecv:
            handleSocketRecv(&msg)

        case wifi.OpSocketClose:
            handleSocketClose(&msg)

        case wifi.OpGetIP:
            handleGetIP(&msg)

        default:
            println("[WiFi] Unknown op:", msg.Op)
        }

        // Periodic GC (every 100 requests)
        if requestCount % 100 == 0 {
            runtime.GC()
            println("[WiFi] GC complete, requests:", requestCount)
        }
    }
}

func handleScan(msg *wifi.WiFiMessage) {
    println("[WiFi] Scan request")

    var req wifi.ScanRequest
    unmarshal(msg.Payload[:], &req)

    // Perform scan
    result := wifi.driverScan(req.MaxResults)

    println("[WiFi] Found", result.NumAPs, "access points")

    // Send response
    sendResponse(msg.ReplyQ, &result)
}

func handleConnect(msg *wifi.WiFiMessage) {
    var req wifi.ConnectRequest
    unmarshal(msg.Payload[:], &req)

    ssid := nullTermString(req.SSID[:])
    println("[WiFi] Connect request to", ssid)

    // Perform connection
    resp := wifi.driverConnect(req.SSID[:], req.Password[:], req.Timeout)

    if resp.Success {
        println("[WiFi] Connected! IP:", ipString(resp.IP))
    } else {
        println("[WiFi] Connection failed")
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleDisconnect(msg *wifi.WiFiMessage) {
    println("[WiFi] Disconnect request")

    // Call esp_wifi_disconnect()
    wifi.disconnectWiFi()

    var resp [1]byte
    resp[0] = 1 // Success
    sendResponse(msg.ReplyQ, &resp)
}

func handleStatus(msg *wifi.WiFiMessage) {
    resp := wifi.driverStatus()

    if resp.Connected {
        println("[WiFi] Status: connected to", nullTermString(resp.SSID[:]))
    } else {
        println("[WiFi] Status: disconnected")
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleCreateSocket(msg *wifi.WiFiMessage) {
    var req wifi.SocketCreateRequest
    unmarshal(msg.Payload[:], &req)

    protoName := "TCP"
    if req.Protocol != 6 {
        protoName = "UDP"
    }

    println("[WiFi] Create socket:", protoName, "port", req.Port)

    resp := wifi.driverCreateSocket(req.Protocol, req.Port)

    if resp.Success {
        println("[WiFi] Socket created, ID:", resp.SocketID)
    } else {
        println("[WiFi] Socket creation failed")
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleSocketSend(msg *wifi.WiFiMessage) {
    var req wifi.SocketSendRequest
    unmarshal(msg.Payload[:], &req)

    println("[WiFi] Send on socket", req.SocketID, "len:", req.DataLen)

    resp := wifi.driverSocketSend(req.SocketID, req.DestIP, req.DestPort,
        req.Data[:req.DataLen])

    sendResponse(msg.ReplyQ, &resp)
}

func handleSocketRecv(msg *wifi.WiFiMessage) {
    var req wifi.SocketRecvRequest
    unmarshal(msg.Payload[:], &req)

    resp := wifi.driverSocketRecv(req.SocketID, req.MaxLen, req.Timeout)

    if resp.DataLen > 0 {
        println("[WiFi] Received", resp.DataLen, "bytes")
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleSocketClose(msg *wifi.WiFiMessage) {
    var socketID uint32
    unmarshal(msg.Payload[:], &socketID)

    println("[WiFi] Close socket", socketID)

    success := wifi.driverSocketClose(socketID)

    var resp [1]byte
    if success {
        resp[0] = 1
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleGetIP(msg *wifi.WiFiMessage) {
    status := wifi.driverStatus()
    sendResponse(msg.ReplyQ, &status.IP)
}

// Send response back via reply queue
func sendResponse(replyQ uint32, data interface{}) {
    buf := marshal(data)

    // Send to FreeRTOS queue
    kernel.xQueueSend(
        kernel.QueueHandle_t(unsafe.Pointer(uintptr(replyQ))),
        unsafe.Pointer(&buf[0]),
        kernel.portMAX_DELAY,
    )
}

// Helpers
func unmarshal(buf []byte, v interface{}) {
    // Simple memcpy for now
    dst := (*[240]byte)(unsafe.Pointer(v))
    copy(dst[:], buf)
}

func marshal(v interface{}) []byte {
    // Simple memcpy
    src := (*[240]byte)(unsafe.Pointer(&v))
    buf := make([]byte, 240)
    copy(buf, src[:])
    return buf
}

func nullTermString(b []byte) string {
    for i, c := range b {
        if c == 0 {
            return string(b[:i])
        }
    }
    return string(b)
}

func ipString(ip [4]byte) string {
    // Simple IP to string (would use proper formatting)
    return "192.168.1.123" // Placeholder
}
