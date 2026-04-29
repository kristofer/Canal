//go:build tinygo

package wifi

import (
    "runtime"
    "unsafe"
    "kernel"
    "domains/wifi"
)

// High-level WiFi API for user applications

var wifiCap runtime.CapHandle
var initialized bool

// Initialize WiFi API
func Init() error {
    if initialized {
        return nil
    }

    // Request capability to WiFi service
    cap, err := runtime.RequestCap("service:wifi", runtime.RightReadWrite)
    if err != nil {
        return err
    }

    wifiCap = cap
    initialized = true
    return nil
}

// Scan for access points
func Scan() ([]AccessPoint, error) {
    if !initialized {
        return nil, errNotInitialized
    }

    // Prepare request
    var req wifi.ScanRequest
    req.MaxResults = 16

    // Send to WiFi service
    result, err := wifiRequest(wifi.OpScan, &req)
    if err != nil {
        return nil, err
    }

    // Parse result
    scanResult := (*wifi.ScanResult)(unsafe.Pointer(&result[0]))

    aps := make([]AccessPoint, scanResult.NumAPs)
    for i := uint8(0); i < scanResult.NumAPs; i++ {
        aps[i] = AccessPoint{
            SSID:    string(trimNull(scanResult.APs[i].SSID[:])),
            RSSI:    scanResult.APs[i].RSSI,
            Channel: scanResult.APs[i].Channel,
        }
    }

    return aps, nil
}

// Connect to access point
func Connect(ssid, password string, timeout uint32) error {
    if !initialized {
        return errNotInitialized
    }

    var req wifi.ConnectRequest
    copy(req.SSID[:], ssid)
    copy(req.Password[:], password)
    req.Timeout = timeout

    result, err := wifiRequest(wifi.OpConnect, &req)
    if err != nil {
        return err
    }

    resp := (*wifi.ConnectResponse)(unsafe.Pointer(&result[0]))
    if !resp.Success {
        return errConnectFailed
    }

    return nil
}

// Disconnect from AP
func Disconnect() error {
    if !initialized {
        return errNotInitialized
    }

    _, err := wifiRequest(wifi.OpDisconnect, nil)
    return err
}

// Get connection status
func Status() (Status, error) {
    if !initialized {
        return Status{}, errNotInitialized
    }

    result, err := wifiRequest(wifi.OpStatus, nil)
    if err != nil {
        return Status{}, err
    }

    resp := (*wifi.StatusResponse)(unsafe.Pointer(&result[0]))

    return Status{
        Connected: resp.Connected,
        SSID:      string(trimNull(resp.SSID[:])),
        IP:        ipToString(resp.IP),
        RSSI:      resp.RSSI,
    }, nil
}

// Get IP address
func GetIP() ([4]byte, error) {
    if !initialized {
        return [4]byte{}, errNotInitialized
    }

    result, err := wifiRequest(wifi.OpGetIP, nil)
    if err != nil {
        return [4]byte{}, err
    }

    var ip [4]byte
    copy(ip[:], result[:4])
    return ip, nil
}

// Send WiFi request and wait for response
func wifiRequest(op wifi.WiFiOp, reqData interface{}) ([]byte, error) {
    // Create reply queue
    replyQ := kernel.xQueueCreate(1, 240)
    if replyQ == nil {
        return nil, errQueueCreate
    }
    defer kernel.vQueueDelete(replyQ)

    // Prepare message
    var msg wifi.WiFiMessage
    msg.Op = op
    msg.ReplyQ = uint32(uintptr(unsafe.Pointer(replyQ)))

    if reqData != nil {
        src := (*[240]byte)(unsafe.Pointer(&reqData))
        copy(msg.Payload[:], src[:])
    }

    // Send to WiFi service
    err := runtime.CapSend(wifiCap, &msg)
    if err != nil {
        return nil, err
    }

    // Wait for response
    var response [240]byte
    result := kernel.xQueueReceive(replyQ, unsafe.Pointer(&response[0]), 10000)
    if result != kernel.pdTRUE {
        return nil, errTimeout
    }

    return response[:], nil
}

// Public types
type AccessPoint struct {
    SSID    string
    RSSI    int8
    Channel uint8
}

type Status struct {
    Connected bool
    SSID      string
    IP        string
    RSSI      int8
}

// Errors
var (
    errNotInitialized = &errorString{"wifi not initialized"}
    errConnectFailed  = &errorString{"connection failed"}
    errQueueCreate    = &errorString{"failed to create queue"}
    errTimeout        = &errorString{"request timeout"}
)

type errorString struct {
    s string
}

func (e *errorString) Error() string { return e.s }

// Helpers
func trimNull(b []byte) []byte {
    for i, c := range b {
        if c == 0 {
            return b[:i]
        }
    }
    return b
}

func ipToString(ip [4]byte) string {
    // Would use proper formatting
    return "192.168.1.123"
}
