package channel

// ─── FS ──────────────────────────────────────────────────────────────────────

// FSRequestOp is the operation selector for FS channel messages.
type FSRequestOp uint8

const (
	FSOpOpen   FSRequestOp = iota
	FSOpClose
	FSOpRead
	FSOpWrite
	FSOpSync
	FSOpStat
	FSOpList
	FSOpMkdir
	FSOpRemove
	FSOpRename
)

// FSRequest is a typed message sent to the fs service channel.
type FSRequest struct {
	Op      FSRequestOp
	Cookie  uint64
	Payload [512]byte
}

// FSResponse is a typed message received from the fs service channel.
type FSResponse struct {
	Cookie  uint64
	Status  uint8
	Payload [512]byte
}

// ─── WiFi ─────────────────────────────────────────────────────────────────────

// WiFiRequestOp is the operation selector for WiFi channel messages.
type WiFiRequestOp uint8

const (
	WiFiOpScan         WiFiRequestOp = iota
	WiFiOpConnect
	WiFiOpDisconnect
	WiFiOpStatus
	WiFiOpGetIP
	WiFiOpSocketCreate
	WiFiOpSocketSend
	WiFiOpSocketRecv
	WiFiOpSocketClose
)

// WiFiRequest is a typed message sent to the wifi service channel.
type WiFiRequest struct {
	Op      WiFiRequestOp
	Cookie  uint64
	Payload [240]byte
}

// WiFiResponse is a typed message received from the wifi service channel.
type WiFiResponse struct {
	Cookie  uint64
	Status  uint8
	Payload [240]byte
}

// ─── TLS ─────────────────────────────────────────────────────────────────────

// TLSRequestOp is the operation selector for TLS channel messages.
type TLSRequestOp uint8

const (
	TLSOpLoadCertificate TLSRequestOp = iota
	TLSOpLoadPrivateKey
	TLSOpCreateContext
	TLSOpDestroyContext
	TLSOpHandshake
	TLSOpWrite
	TLSOpRead
	TLSOpClose
)

// TLSRequest is a typed message sent to the tls service channel.
type TLSRequest struct {
	Op      TLSRequestOp
	Cookie  uint64
	Payload [2048]byte
}

// TLSResponse is a typed message received from the tls service channel.
type TLSResponse struct {
	Cookie  uint64
	Status  uint8
	Payload [2048]byte
}
