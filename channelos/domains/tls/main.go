//go:build tinygo

package main

import (
    "runtime"
    "domains/tls"
)

func main() {
    println("[TLS] Service starting...")

    // Initialize key store
    err := tls.initKeyStore()
    if err != nil {
        println("[TLS] Keystore init failed:", err.Error())
        return
    }

    println("[TLS] Keystore initialized")

    // Expose TLS service capability
    serviceCap, err := runtime.ExposeCap("service:tls", runtime.RightReadWrite)
    if err != nil {
        println("[TLS] Failed to expose service")
        return
    }

    println("[TLS] Service ready")

    // IMPORTANT: No network capability!
    // TLS domain NEVER touches the network directly
    // It only encrypts/decrypts data via capability messages

    // Main service loop
    var msg tls.TLSMessage
    requestCount := uint32(0)

    for {
        err := runtime.CapRecv(serviceCap, &msg)
        if err != nil {
            println("[TLS] Recv error:", err.Error())
            continue
        }

        requestCount++

        switch msg.Op {
        case tls.OpLoadCertificate:
            handleLoadCertificate(&msg)

        case tls.OpLoadPrivateKey:
            handleLoadPrivateKey(&msg)

        case tls.OpCreateContext:
            handleCreateContext(&msg)

        case tls.OpHandshake:
            handleHandshake(&msg)

        case tls.OpWrite:
            handleWrite(&msg)

        case tls.OpRead:
            handleRead(&msg)

        case tls.OpClose:
            handleClose(&msg)

        case tls.OpGetPeerCert:
            handleGetPeerCert(&msg)

        default:
            println("[TLS] Unknown op:", msg.Op)
        }

        // Aggressive GC - we handle crypto material
        if requestCount % 50 == 0 {
            runtime.GC()

            // Also scrub freed memory (paranoid)
            runtime.ScrubFreedMemory()
        }
    }
}

func handleLoadCertificate(msg *tls.TLSMessage) {
    var req tls.LoadCertificateRequest
    unmarshal(msg.Payload[:], &req)

    label := nullTermString(req.Label[:])
    println("[TLS] Load certificate:", label)

    certID, err := tls.storeCertificate(req.Label[:], req.CertPEM[:req.CertLen])

    var resp tls.LoadCertificateResponse
    if err == nil {
        resp.CertID = certID
        resp.Success = true
        println("[TLS] Certificate loaded, ID:", certID)
    } else {
        println("[TLS] Certificate load failed")
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleLoadPrivateKey(msg *tls.TLSMessage) {
    var req tls.LoadPrivateKeyRequest
    unmarshal(msg.Payload[:], &req)

    label := nullTermString(req.Label[:])
    println("[TLS] Load private key:", label)

    // CRITICAL: Private key never leaves this function scope!
    keyID, err := tls.storePrivateKey(req.Label[:], req.KeyPEM[:req.KeyLen], req.Password[:])

    // Zero the request payload (contains private key!)
    zeroBytes(msg.Payload[:])
    zeroBytes(req.KeyPEM[:])

    var resp tls.LoadPrivateKeyResponse
    if err == nil {
        resp.KeyID = keyID
        resp.Success = true
        println("[TLS] Private key loaded, ID:", keyID)
    } else {
        println("[TLS] Private key load failed")
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleCreateContext(msg *tls.TLSMessage) {
    var req tls.CreateContextRequest
    unmarshal(msg.Payload[:], &req)

    role := "client"
    if req.Role == 1 {
        role = "server"
    }

    println("[TLS] Create context:", role, "cert:", req.CertID, "key:", req.KeyID)

    contextID, err := tls.createContext(req.Role, req.CertID, req.KeyID, req.VerifyPeer)

    var resp tls.CreateContextResponse
    if err == nil {
        resp.ContextID = contextID
        resp.Success = true
        println("[TLS] Context created, ID:", contextID)
    } else {
        println("[TLS] Context creation failed")
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleHandshake(msg *tls.TLSMessage) {
    var req tls.HandshakeRequest
    unmarshal(msg.Payload[:], &req)

    println("[TLS] Handshake step for context", req.ContextID)

    resp, err := tls.doHandshake(req.ContextID, req.InputData[:req.InputLen])

    if err != nil {
        println("[TLS] Handshake error:", err.Error())
        resp.State = tls.HandshakeFailed
    } else {
        switch resp.State {
        case tls.HandshakeComplete:
            println("[TLS] Handshake complete!")
        case tls.HandshakeInProgress:
            println("[TLS] Handshake in progress...")
        case tls.HandshakeFailed:
            println("[TLS] Handshake failed")
        }
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleWrite(msg *tls.TLSMessage) {
    var req tls.WriteRequest
    unmarshal(msg.Payload[:], &req)

    println("[TLS] Encrypt", req.Length, "bytes for context", req.ContextID)

    resp, err := tls.tlsWrite(req.ContextID, req.Plaintext[:req.Length])

    if err != nil {
        println("[TLS] Encrypt error:", err.Error())
        resp.Success = false
    } else {
        println("[TLS] Encrypted to", resp.Length, "bytes")
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleRead(msg *tls.TLSMessage) {
    var req tls.ReadRequest
    unmarshal(msg.Payload[:], &req)

    println("[TLS] Decrypt", req.Length, "bytes for context", req.ContextID)

    resp, err := tls.tlsRead(req.ContextID, req.Ciphertext[:req.Length])

    if err != nil {
        println("[TLS] Decrypt error:", err.Error())
        resp.Success = false
    } else {
        println("[TLS] Decrypted to", resp.Length, "bytes")
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleClose(msg *tls.TLSMessage) {
    var contextID uint32
    unmarshal(msg.Payload[:], &contextID)

    println("[TLS] Close context", contextID)

    err := tls.destroyContext(contextID)

    var resp [1]byte
    if err == nil {
        resp[0] = 1
    }

    sendResponse(msg.ReplyQ, &resp)
}

func handleGetPeerCert(msg *tls.TLSMessage) {
    var req tls.GetPeerCertRequest
    unmarshal(msg.Payload[:], &req)

    println("[TLS] Get peer cert for context", req.ContextID)

    resp, err := tls.getPeerCertificate(req.ContextID)

    if err != nil {
        println("[TLS] Get peer cert failed")
    } else {
        println("[TLS] Peer cert retrieved, CN:", nullTermString(resp.CommonName[:]))
    }

    sendResponse(msg.ReplyQ, &resp)
}

// Helpers (same as WiFi domain)
func sendResponse(replyQ uint32, data interface{}) { /* ... */ }
func unmarshal(buf []byte, v interface{}) { /* ... */ }
func nullTermString(b []byte) string { /* ... */ }
func zeroBytes(b []byte) { /* ... */ }
