//go:build tinygo

package tls

import (
    "crypto/rand"
    "unsafe"
)

// Key storage - NEVER leaves TLS domain
// Keys are encrypted at rest with domain-specific key

type keyStore struct {
    keys     [16]keyEntry
    certs    [16]certEntry
    domainKey [32]byte  // Domain encryption key
}

type keyEntry struct {
    id       uint32
    label    [32]byte
    keyType  uint8  // RSA, ECDSA, Ed25519

    // Encrypted private key material
    encrypted [2048]byte
    encLen    uint16

    // Key metadata (not encrypted)
    keySize   uint16
    curve     uint8  // For EC keys

    inUse     bool
}

type certEntry struct {
    id       uint32
    label    [32]byte

    // X.509 certificate (not encrypted - public data)
    certDER  [2048]byte
    certLen  uint16

    // Certificate info (parsed)
    info     CertificateInfo

    inUse    bool
}

var store keyStore
var storeInitialized bool

// Initialize key store
func initKeyStore() error {
    if storeInitialized {
        return nil
    }

    // Generate domain encryption key
    // In production, derive from hardware unique ID + entropy
    _, err := rand.Read(store.domainKey[:])
    if err != nil {
        return err
    }

    // Wipe key material on panic
    // (Go's defer won't help here - need supervisor restart)

    storeInitialized = true
    return nil
}

// Store private key (encrypts before storing)
func storePrivateKey(label []byte, keyPEM []byte, password []byte) (uint32, error) {
    // Find free slot
    var id uint32
    for i := uint32(0); i < 16; i++ {
        if !store.keys[i].inUse {
            id = i
            break
        }
    }

    entry := &store.keys[id]

    // Parse PEM to get key type/size
    keyType, keySize, keyDER := parsePEMPrivateKey(keyPEM, password)
    if keyDER == nil {
        return 0, errInvalidKey
    }

    // Encrypt key with domain key
    encrypted := encrypt(keyDER, store.domainKey[:])

    // Store encrypted key
    copy(entry.label[:], label)
    copy(entry.encrypted[:], encrypted)
    entry.encLen = uint16(len(encrypted))
    entry.keyType = keyType
    entry.keySize = keySize
    entry.id = id
    entry.inUse = true

    // Zero out plaintext key material
    zeroBytes(keyDER)

    return id, nil
}

// Retrieve private key (decrypts)
func retrievePrivateKey(keyID uint32) ([]byte, error) {
    if keyID >= 16 || !store.keys[keyID].inUse {
        return nil, errInvalidKeyID
    }

    entry := &store.keys[keyID]

    // Decrypt key
    keyDER := decrypt(entry.encrypted[:entry.encLen], store.domainKey[:])

    return keyDER, nil
}

// Store certificate (public, no encryption needed)
func storeCertificate(label []byte, certPEM []byte) (uint32, error) {
    // Find free slot
    var id uint32
    for i := uint32(0); i < 16; i++ {
        if !store.certs[i].inUse {
            id = i
            break
        }
    }

    entry := &store.certs[id]

    // Parse PEM to DER
    certDER := parsePEMCertificate(certPEM)
    if certDER == nil {
        return 0, errInvalidCert
    }

    // Parse certificate info
    info := parseCertificateInfo(certDER)

    // Store
    copy(entry.label[:], label)
    copy(entry.certDER[:], certDER)
    entry.certLen = uint16(len(certDER))
    entry.info = info
    entry.id = id
    entry.inUse = true

    return id, nil
}

// Retrieve certificate
func retrieveCertificate(certID uint32) ([]byte, error) {
    if certID >= 16 || !store.certs[certID].inUse {
        return nil, errInvalidCertID
    }

    entry := &store.certs[certID]
    return entry.certDER[:entry.certLen], nil
}

// Encryption helpers (AES-GCM)
func encrypt(plaintext, key []byte) []byte {
    // Use mbedTLS AES-GCM
    // In production: proper IV, auth tag, etc.
    return aesGCMEncrypt(plaintext, key)
}

func decrypt(ciphertext, key []byte) []byte {
    return aesGCMDecrypt(ciphertext, key)
}

// Zero memory (security critical!)
func zeroBytes(buf []byte) {
    for i := range buf {
        buf[i] = 0
    }
}

// Prevent compiler optimization of zero
//go:noinline
func zeroMemory(ptr unsafe.Pointer, len uintptr) {
    // Assembly: memset with memory barrier
    volatile.StorePointer(&ptr, ptr)
    for i := uintptr(0); i < len; i++ {
        *(*byte)(unsafe.Pointer(uintptr(ptr) + i)) = 0
    }
}
