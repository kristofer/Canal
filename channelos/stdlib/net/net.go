//go:build tinygo

package net

import (
    "runtime"
    "unsafe"
    "kernel"
    "domains/wifi"
    wifilib "stdlib/wifi"
)

// TCP/UDP connection backed by WiFi service

type Conn struct {
    socketID uint32
    protocol uint8
    wifiCap  runtime.CapHandle
}

// Dial creates a TCP connection
func Dial(network, address string) (*Conn, error) {
    // Ensure WiFi is initialized
    if err := wifilib.Init(); err != nil {
        return nil, err
    }

    // Parse address (host:port)
    host, port := parseAddress(address)

    // Resolve host to IP (simplified - would do DNS)
    ip := resolveHost(host)

    // Request WiFi capability
    wifiCap, err := runtime.RequestCap("service:wifi", runtime.RightReadWrite)
    if err != nil {
        return nil, err
    }

    // Create socket via WiFi service
    var req wifi.SocketCreateRequest
    if network == "tcp" {
        req.Protocol = 6
    } else if network == "udp" {
        req.Protocol = 17
    } else {
        return nil, errUnsupportedProtocol
    }
    req.Port = 0 // Client socket (no bind)

    result, err := wifiRequest(wifiCap, wifi.OpCreateSocket, &req)
    if err != nil {
        return nil, err
    }

    resp := (*wifi.SocketCreateResponse)(unsafe.Pointer(&result[0]))
    if !resp.Success {
        return nil, errSocketCreate
    }

    conn := &Conn{
        socketID: resp.SocketID,
        protocol: req.Protocol,
        wifiCap:  wifiCap,
    }

    // For TCP, connect now
    if network == "tcp" {
        // Send SYN via first packet
        // (in real implementation, would have explicit connect op)
    }

    return conn, nil
}

// Write data to connection
func (c *Conn) Write(data []byte) (int, error) {
    var req wifi.SocketSendRequest
    req.SocketID = c.socketID
    // req.DestIP and DestPort would be set for UDP
    req.DataLen = uint16(len(data))
    copy(req.Data[:], data)

    result, err := wifiRequest(c.wifiCap, wifi.OpSocketSend, &req)
    if err != nil {
        return 0, err
    }

    resp := (*wifi.SocketSendResponse)(unsafe.Pointer(&result[0]))
    if !resp.Success {
        return 0, errSendFailed
    }

    return int(resp.BytesSent), nil
}

// Read data from connection
func (c *Conn) Read(buf []byte) (int, error) {
    var req wifi.SocketRecvRequest
    req.SocketID = c.socketID
    req.MaxLen = uint16(len(buf))
    req.Timeout = 5000 // 5 second timeout

    result, err := wifiRequest(c.wifiCap, wifi.OpSocketRecv, &req)
    if err != nil {
        return 0, err
    }

    resp := (*wifi.SocketRecvResponse)(unsafe.Pointer(&result[0]))

    n := int(resp.DataLen)
    copy(buf, resp.Data[:n])

    return n, nil
}

// Close connection
func (c *Conn) Close() error {
    _, err := wifiRequest(c.wifiCap, wifi.OpSocketClose, &c.socketID)
    return err
}

// Helper to send WiFi request
func wifiRequest(cap runtime.CapHandle, op wifi.WiFiOp, reqData interface{}) ([]byte, error) {
    // Same as in wifi.go
    replyQ := kernel.xQueueCreate(1, 240)
    defer kernel.vQueueDelete(replyQ)

    var msg wifi.WiFiMessage
    msg.Op = op
    msg.ReplyQ = uint32(uintptr(unsafe.Pointer(replyQ)))

    if reqData != nil {
        src := (*[240]byte)(unsafe.Pointer(&reqData))
        copy(msg.Payload[:], src[:])
    }

    runtime.CapSend(cap, &msg)

    var response [240]byte
    kernel.xQueueReceive(replyQ, unsafe.Pointer(&response[0]), 10000)

    return response[:], nil
}

func parseAddress(addr string) (string, uint16) {
    // Simple parser (would be more robust)
    return "example.com", 80
}

func resolveHost(host string) [4]byte {
    // Would do DNS query via WiFi service
    return [4]byte{93, 184, 216, 34} // example.com
}

var (
    errUnsupportedProtocol = &errorString{"unsupported protocol"}
    errSocketCreate        = &errorString{"socket creation failed"}
    errSendFailed          = &errorString{"send failed"}
)

type errorString struct{ s string }
func (e *errorString) Error() string { return e.s }
