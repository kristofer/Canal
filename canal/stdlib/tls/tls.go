//go:build tinygo

package tls

import (
    "runtime"
    "stdlib/net"
    "domains/tls"
)

// TLS connection wrapper
type Conn struct {
    rawConn   *net.Conn
    contextID uint32
    tlsCap    runtime.CapHandle

    handshakeDone bool
}

// Dial creates a TLS connection (client)
func Dial(network, address string) (*Conn, error) {
    // Create raw TCP connection
    rawConn, err := net.Dial(network, address)
    if err != nil {
        return nil, err
    }

    // Get TLS capability
    tlsCap, err := runtime.RequestCap("service:tls", runtime.RightReadWrite)
    if err != nil {
        rawConn.Close()
        return nil, err
    }

    // Create TLS context
    contextID, err := createContext(tlsCap, 0, 0xFFFFFFFF, 0xFFFFFFFF, true)
    if err != nil {
        rawConn.Close()
        return nil, err
    }

    conn := &Conn{
        rawConn:   rawConn,
        contextID: contextID,
        tlsCap:    tlsCap,
    }

    // Perform handshake
    err = conn.Handshake()
    if err != nil {
        conn.Close()
        return nil, err
    }

    return conn, nil
}

// Handshake performs TLS handshake
func (c *Conn) Handshake() error {
    if c.handshakeDone {
        return nil
    }

    // TLS handshake loop
    for {
        // Request handshake step
        var req tls.HandshakeRequest
        req.ContextID = c.contextID
        req.InputLen = 0

        resp, err := tlsRequest(c.tlsCap, tls.OpHandshake, &req)
        if err != nil {
            return err
        }

        hsResp := (*tls.HandshakeResponse)(unsafe.Pointer(&resp[0]))

        // Send output if any
        if hsResp.OutputLen > 0 {
            _, err = c.rawConn.Write(hsResp.OutputData[:hsResp.OutputLen])
            if err != nil {
                return err
            }
        }

        // Check state
        switch hsResp.State {
        case tls.HandshakeComplete:
            c.handshakeDone = true
            return nil

        case tls.HandshakeFailed:
            return errHandshakeFailed

        case tls.HandshakeInProgress:
            // Need more data?
            if hsResp.NeedMoreData {
                // Read from network
                buf := make([]byte, 4096)
                n, err := c.rawConn.Read(buf)
                if err != nil {
                    return err
                }

                // Feed to TLS domain
                req.InputLen = uint16(n)
                copy(req.InputData[:], buf[:n])
            }
        }
    }
}

// Write encrypts and sends data
func (c *Conn) Write(data []byte) (int, error) {
    if !c.handshakeDone {
        return 0, errHandshakeNotComplete
    }

    // Encrypt via TLS domain
    var req tls.WriteRequest
    req.ContextID = c.contextID
    req.Length = uint16(len(data))
    copy(req.Plaintext[:], data)

    result, err := tlsRequest(c.tlsCap, tls.OpWrite, &req)
    if err != nil {
        return 0, err
    }

    resp := (*tls.WriteResponse)(unsafe.Pointer(&result[0]))
    if !resp.Success {
        return 0, errEncryptFailed
    }

    // Send encrypted data over network
    return c.rawConn.Write(resp.Ciphertext[:resp.Length])
}

// Read receives and decrypts data
func (c *Conn) Read(buf []byte) (int, error) {
    if !c.handshakeDone {
        return 0, errHandshakeNotComplete
    }

    // Read encrypted data from network
    ciphertext := make([]byte, len(buf)+512) // TLS overhead
    n, err := c.rawConn.Read(ciphertext)
    if err != nil {
        return 0, err
    }

    // Decrypt via TLS domain
    var req tls.ReadRequest
    req.ContextID = c.contextID
    req.Length = uint16(n)
    copy(req.Ciphertext[:], ciphertext[:n])

    result, err := tlsRequest(c.tlsCap, tls.OpRead, &req)
    if err != nil {
        return 0, err
    }

    resp := (*tls.ReadResponse)(unsafe.Pointer(&result[0]))
    if !resp.Success {
        return 0, errDecryptFailed
    }

    // Copy plaintext to buffer
    n = int(resp.Length)
    copy(buf, resp.Plaintext[:n])

    return n, nil
}

// Close closes TLS connection
func (c *Conn) Close() error {
    // Close TLS context
    tlsRequest(c.tlsCap, tls.OpClose, &c.contextID)

    // Close raw connection
    return c.rawConn.Close()
}

// LoadCertificate loads a certificate into TLS domain
func LoadCertificate(label string, certPEM []byte) (uint32, error) {
    tlsCap, err := runtime.RequestCap("service:tls", runtime.RightReadWrite)
    if err != nil {
        return 0, err
    }

    var req tls.LoadCertificateRequest
    copy(req.Label[:], label)
    copy(req.CertPEM[:], certPEM)
    req.CertLen = uint16(len(certPEM))

    result, err := tlsRequest(tlsCap, tls.OpLoadCertificate, &req)
    if err != nil {
        return 0, err
    }

    resp := (*tls.LoadCertificateResponse)(unsafe.Pointer(&result[0]))
    if !resp.Success {
        return 0, errLoadFailed
    }

    return resp.CertID, nil
}

// LoadPrivateKey loads a private key into TLS domain
func LoadPrivateKey(label string, keyPEM, password []byte) (uint32, error) {
    tlsCap, err := runtime.RequestCap("service:tls", runtime.RightReadWrite)
    if err != nil {
        return 0, err
    }

    var req tls.LoadPrivateKeyRequest
    copy(req.Label[:], label)
    copy(req.KeyPEM[:], keyPEM)
    req.KeyLen = uint16(len(keyPEM))
    if password != nil {
        copy(req.Password[:], password)
    }

    result, err := tlsRequest(tlsCap, tls.OpLoadPrivateKey, &req)
    if err != nil {
        return 0, err
    }

    // IMPORTANT: Zero the request (contained private key)
    zeroBytes(req.KeyPEM[:])

    resp := (*tls.LoadPrivateKeyResponse)(unsafe.Pointer(&result[0]))
    if !resp.Success {
        return 0, errLoadFailed
    }

    return resp.KeyID, nil
}

// Helper functions
func createContext(cap runtime.CapHandle, role uint8, certID, keyID uint32, verify bool) (uint32, error) {
    var req tls.CreateContextRequest
    req.Role = role
    req.CertID = certID
    req.KeyID = keyID
    req.VerifyPeer = verify
    req.MinVersion = 3 // TLS 1.2

    result, err := tlsRequest(cap, tls.OpCreateContext, &req)
    if err != nil {
        return 0, err
    }

    resp := (*tls.CreateContextResponse)(unsafe.Pointer(&result[0]))
    if !resp.Success {
        return 0, errContextFailed
    }

    return resp.ContextID, nil
}

func tlsRequest(cap runtime.CapHandle, op tls.TLSOp, reqData interface{}) ([]byte, error) {
    // Same pattern as WiFi requests
    replyQ := kernel.xQueueCreate(1, 2048)
    defer kernel.vQueueDelete(replyQ)

    var msg tls.TLSMessage
    msg.Op = op
    msg.ReplyQ = uint32(uintptr(unsafe.Pointer(replyQ)))

    if reqData != nil {
        src := (*[2048]byte)(unsafe.Pointer(&reqData))
        copy(msg.Payload[:], src[:])
    }

    runtime.CapSend(cap, &msg)

    var response [2048]byte
    kernel.xQueueReceive(replyQ, unsafe.Pointer(&response[0]), 10000)

    return response[:], nil
}

// Errors
var (
    errHandshakeFailed      = &errorString{"handshake failed"}
    errHandshakeNotComplete = &errorString{"handshake not complete"}
    errEncryptFailed        = &errorString{"encryption failed"}
    errDecryptFailed        = &errorString{"decryption failed"}
    errLoadFailed           = &errorString{"load failed"}
    errContextFailed        = &errorString{"context creation failed"}
)
