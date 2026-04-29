//go:build tinygo

package tls

// TLS operations exposed via capability
type TLSMessage struct {
    Op       TLSOp
    SessionID uint32
    ReplyQ   uint32
    Payload  [2048]byte  // Larger for certs/keys
}

type TLSOp uint8

const (
    OpLoadCertificate TLSOp = iota
    OpLoadPrivateKey
    OpCreateContext
    OpDestroyContext
    OpHandshake
    OpWrite
    OpRead
    OpClose
    OpGetPeerCert
    OpVerifyPeer
)

// Load certificate request
type LoadCertificateRequest struct {
    Label     [32]byte   // Human-readable label
    CertPEM   [2048]byte // PEM-encoded certificate
    CertLen   uint16
}

type LoadCertificateResponse struct {
    CertID  uint32
    Success bool
}

// Load private key request (never leaves TLS domain!)
type LoadPrivateKeyRequest struct {
    Label     [32]byte
    KeyPEM    [2048]byte  // PEM-encoded private key
    KeyLen    uint16
    Password  [64]byte    // Optional password
}

type LoadPrivateKeyResponse struct {
    KeyID   uint32
    Success bool
}

// Create TLS context
type CreateContextRequest struct {
    Role       uint8  // 0=client, 1=server
    CertID     uint32 // Certificate to use
    KeyID      uint32 // Private key to use
    VerifyPeer bool   // Require peer certificate
    MinVersion uint8  // TLS 1.2, 1.3, etc.
}

type CreateContextResponse struct {
    ContextID uint32
    Success   bool
}

// TLS handshake (non-blocking state machine)
type HandshakeRequest struct {
    ContextID    uint32
    InputData    [2048]byte
    InputLen     uint16
    WantRead     bool  // Does handshake need more input?
}

type HandshakeResponse struct {
    State        HandshakeState
    OutputData   [2048]byte
    OutputLen    uint16
    NeedMoreData bool
}

type HandshakeState uint8

const (
    HandshakeInProgress HandshakeState = iota
    HandshakeComplete
    HandshakeFailed
)

// Encrypt data (plaintext → ciphertext)
type WriteRequest struct {
    ContextID uint32
    Plaintext [1400]byte
    Length    uint16
}

type WriteResponse struct {
    Ciphertext [1500]byte  // Slightly larger (TLS overhead)
    Length     uint16
    Success    bool
}

// Decrypt data (ciphertext → plaintext)
type ReadRequest struct {
    ContextID  uint32
    Ciphertext [1500]byte
    Length     uint16
}

type ReadResponse struct {
    Plaintext [1400]byte
    Length    uint16
    Success   bool
}

// Get peer certificate (for verification)
type GetPeerCertRequest struct {
    ContextID uint32
}

type GetPeerCertResponse struct {
    CertPEM   [2048]byte
    CertLen   uint16
    CommonName [64]byte
    Verified  bool
}

// Certificate info
type CertificateInfo struct {
    Subject    [128]byte
    Issuer     [128]byte
    NotBefore  uint64  // Unix timestamp
    NotAfter   uint64
    PublicKey  [512]byte
    Verified   bool
}
