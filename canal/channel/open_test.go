package channel

import "testing"

func TestOpenKnownChannel(t *testing.T) {
	entry, err := Open("fs", SchemaV1)
	if err != nil {
		t.Fatalf("Open(fs): %v", err)
	}
	if entry.ID != ServiceFS {
		t.Fatalf("Open(fs).ID = %d, want %d", entry.ID, ServiceFS)
	}
}

func TestOpenUnknownChannelDeterministicError(t *testing.T) {
	if _, err := Open("nope", SchemaV1); err != ErrOpenFailed {
		t.Fatalf("got %v, want ErrOpenFailed", err)
	}
}

func TestOpenSchemaMismatchDeterministicError(t *testing.T) {
	if _, err := Open("fs", SchemaVersion(99)); err != ErrOpenFailed {
		t.Fatalf("got %v, want ErrOpenFailed", err)
	}
}

func TestOpenHelpers(t *testing.T) {
	if e, err := OpenFS(); err != nil || e.ID != ServiceFS {
		t.Fatalf("OpenFS() = (%+v, %v)", e, err)
	}
	if e, err := OpenWiFi(); err != nil || e.ID != ServiceWiFi {
		t.Fatalf("OpenWiFi() = (%+v, %v)", e, err)
	}
	if e, err := OpenTLS(); err != nil || e.ID != ServiceTLS {
		t.Fatalf("OpenTLS() = (%+v, %v)", e, err)
	}
}

func TestValidateSendAcceptsExpectedTypes(t *testing.T) {
	tests := []struct {
		service ServiceID
		msg     any
	}{
		{ServiceFS, FSRequest{}},
		{ServiceFS, &FSRequest{}},
		{ServiceWiFi, WiFiRequest{}},
		{ServiceWiFi, &WiFiRequest{}},
		{ServiceTLS, TLSRequest{}},
		{ServiceTLS, &TLSRequest{}},
	}

	for _, tc := range tests {
		if err := ValidateSend(tc.service, tc.msg); err != nil {
			t.Fatalf("ValidateSend(%d, %T) = %v, want nil", tc.service, tc.msg, err)
		}
	}
}

func TestValidateSendRejectsMismatchedType(t *testing.T) {
	if err := ValidateSend(ServiceFS, WiFiRequest{}); err != ErrSendFailed {
		t.Fatalf("got %v, want ErrSendFailed", err)
	}
	if err := ValidateSend(ServiceID(77), FSRequest{}); err != ErrSendFailed {
		t.Fatalf("got %v, want ErrSendFailed", err)
	}
}

func TestValidateRecvAcceptsExpectedTypes(t *testing.T) {
	tests := []struct {
		service ServiceID
		msg     any
	}{
		{ServiceFS, FSResponse{}},
		{ServiceFS, &FSResponse{}},
		{ServiceWiFi, WiFiResponse{}},
		{ServiceWiFi, &WiFiResponse{}},
		{ServiceTLS, TLSResponse{}},
		{ServiceTLS, &TLSResponse{}},
	}

	for _, tc := range tests {
		if err := ValidateRecv(tc.service, tc.msg); err != nil {
			t.Fatalf("ValidateRecv(%d, %T) = %v, want nil", tc.service, tc.msg, err)
		}
	}
}

func TestValidateRecvRejectsMismatchedType(t *testing.T) {
	if err := ValidateRecv(ServiceWiFi, TLSResponse{}); err != ErrRecvFailed {
		t.Fatalf("got %v, want ErrRecvFailed", err)
	}
	if err := ValidateRecv(ServiceID(88), WiFiResponse{}); err != ErrRecvFailed {
		t.Fatalf("got %v, want ErrRecvFailed", err)
	}
}
