//go:build tinygo && esp32s3

package wifi

// Message types sent over WiFi capability channel
type WiFiMessage struct {
    Op      WiFiOp
    ReplyQ  uint32  // FreeRTOS queue handle for response
    Payload [240]byte
}

type WiFiOp uint8

const (
    OpScan WiFiOp = iota
    OpConnect
    OpDisconnect
    OpStatus
    OpCreateSocket
    OpSocketSend
    OpSocketRecv
    OpSocketClose
    OpGetIP
)

// Scan request/response
type ScanRequest struct {
    MaxResults uint8
}

type ScanResult struct {
    NumAPs uint8
    APs    [16]AccessPoint
}

type AccessPoint struct {
    SSID     [32]byte
    BSSID    [6]byte
    RSSI     int8
    Channel  uint8
    AuthMode uint8
}

// Connect request/response
type ConnectRequest struct {
    SSID     [32]byte
    Password [64]byte
    Timeout  uint32  // Milliseconds
}

type ConnectResponse struct {
    Success bool
    IP      [4]byte
    Netmask [4]byte
    Gateway [4]byte
}

// Status request/response
type StatusRequest struct{}

type StatusResponse struct {
    Connected bool
    SSID      [32]byte
    IP        [4]byte
    RSSI      int8
}

// Socket operations
type SocketCreateRequest struct {
    Protocol uint8  // TCP=6, UDP=17
    Port     uint16
}

type SocketCreateResponse struct {
    SocketID uint32
    Success  bool
}

type SocketSendRequest struct {
    SocketID uint32
    DestIP   [4]byte
    DestPort uint16
    DataLen  uint16
    Data     [1400]byte
}

type SocketSendResponse struct {
    BytesSent uint16
    Success   bool
}

type SocketRecvRequest struct {
    SocketID uint32
    MaxLen   uint16
    Timeout  uint32
}

type SocketRecvResponse struct {
    DataLen  uint16
    SrcIP    [4]byte
    SrcPort  uint16
    Data     [1400]byte
}
