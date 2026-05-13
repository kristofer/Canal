package channel

import "errors"

var (
	ErrOpenFailed = errors.New("channel: open failed")
	ErrSendFailed = errors.New("channel: send failed")
	ErrRecvFailed = errors.New("channel: receive failed")
)

// Open resolves a named channel and validates the requested schema.
// Unknown names and schema mismatches both return ErrOpenFailed.
func Open(name string, schema SchemaVersion) (Entry, error) {
	entry, ok := Lookup(name)
	if !ok {
		return Entry{}, ErrOpenFailed
	}
	if err := Validate(entry.ID, schema); err != nil {
		return Entry{}, ErrOpenFailed
	}
	return entry, nil
}

// OpenFS resolves the typed fs service channel entry.
func OpenFS() (Entry, error) {
	return Open("fs", SchemaV1)
}

// OpenWiFi resolves the typed wifi service channel entry.
func OpenWiFi() (Entry, error) {
	return Open("wifi", SchemaV1)
}

// OpenTLS resolves the typed tls service channel entry.
func OpenTLS() (Entry, error) {
	return Open("tls", SchemaV1)
}

// ValidateSend checks that msg matches the registered request type for serviceID.
// It returns deterministic ErrSendFailed on mismatch.
func ValidateSend(serviceID ServiceID, msg any) error {
	if !validSendType(serviceID, msg) {
		return ErrSendFailed
	}
	return nil
}

// ValidateRecv checks that msg matches the registered response type for serviceID.
// It returns deterministic ErrRecvFailed on mismatch.
func ValidateRecv(serviceID ServiceID, msg any) error {
	if !validRecvType(serviceID, msg) {
		return ErrRecvFailed
	}
	return nil
}

func validSendType(serviceID ServiceID, msg any) bool {
	switch serviceID {
	case ServiceFS:
		switch msg.(type) {
		case FSRequest, *FSRequest:
			return true
		}
	case ServiceWiFi:
		switch msg.(type) {
		case WiFiRequest, *WiFiRequest:
			return true
		}
	case ServiceTLS:
		switch msg.(type) {
		case TLSRequest, *TLSRequest:
			return true
		}
	}
	return false
}

func validRecvType(serviceID ServiceID, msg any) bool {
	switch serviceID {
	case ServiceFS:
		switch msg.(type) {
		case FSResponse, *FSResponse:
			return true
		}
	case ServiceWiFi:
		switch msg.(type) {
		case WiFiResponse, *WiFiResponse:
			return true
		}
	case ServiceTLS:
		switch msg.(type) {
		case TLSResponse, *TLSResponse:
			return true
		}
	}
	return false
}
