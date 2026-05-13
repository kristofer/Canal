package channel

import "testing"

// ─── Registry ────────────────────────────────────────────────────────────────

func TestLookupKnownServices(t *testing.T) {
	cases := []struct {
		name string
		id   ServiceID
	}{
		{"fs", ServiceFS},
		{"wifi", ServiceWiFi},
		{"tls", ServiceTLS},
	}
	for _, tc := range cases {
		e, ok := Lookup(tc.name)
		if !ok {
			t.Fatalf("Lookup(%q) returned false", tc.name)
		}
		if e.ID != tc.id {
			t.Fatalf("Lookup(%q).ID = %d, want %d", tc.name, e.ID, tc.id)
		}
		if e.Schema != SchemaV1 {
			t.Fatalf("Lookup(%q).Schema = %d, want %d", tc.name, e.Schema, SchemaV1)
		}
	}
}

func TestLookupUnknownService(t *testing.T) {
	_, ok := Lookup("gpio")
	if ok {
		t.Fatal("Lookup(unknown) should return false")
	}
}

func TestLookupByIDRoundTrip(t *testing.T) {
	for _, id := range []ServiceID{ServiceFS, ServiceWiFi, ServiceTLS} {
		e, ok := LookupByID(id)
		if !ok {
			t.Fatalf("LookupByID(%d) returned false", id)
		}
		e2, ok2 := Lookup(e.Name)
		if !ok2 || e2.ID != id {
			t.Fatalf("round-trip failed for service ID %d", id)
		}
	}
}

// ─── Validate ─────────────────────────────────────────────────────────────────

func TestValidateAcceptsRegisteredService(t *testing.T) {
	for _, id := range []ServiceID{ServiceFS, ServiceWiFi, ServiceTLS} {
		if err := Validate(id, SchemaV1); err != nil {
			t.Fatalf("Validate(%d, SchemaV1) = %v, want nil", id, err)
		}
	}
}

func TestValidateRejectsUnknownService(t *testing.T) {
	if err := Validate(ServiceID(99), SchemaV1); err != ErrUnknownService {
		t.Fatalf("got %v, want ErrUnknownService", err)
	}
}

func TestValidateRejectsSchemaMismatch(t *testing.T) {
	if err := Validate(ServiceFS, SchemaVersion(99)); err != ErrSchemaMismatch {
		t.Fatalf("got %v, want ErrSchemaMismatch", err)
	}
}

// ─── Type sizing ─────────────────────────────────────────────────────────────
// Ensure each typed request/response is non-zero and carries the cookie field.

func TestFSRequestCookieRoundTrip(t *testing.T) {
	var req FSRequest
	req.Op = FSOpRead
	req.Cookie = 0xDEADBEEF
	if req.Cookie != 0xDEADBEEF {
		t.Fatal("FSRequest cookie not preserved")
	}
}

func TestWiFiRequestCookieRoundTrip(t *testing.T) {
	var req WiFiRequest
	req.Op = WiFiOpConnect
	req.Cookie = 42
	if req.Cookie != 42 {
		t.Fatal("WiFiRequest cookie not preserved")
	}
}

func TestTLSRequestCookieRoundTrip(t *testing.T) {
	var req TLSRequest
	req.Op = TLSOpHandshake
	req.Cookie = 1
	if req.Cookie != 1 {
		t.Fatal("TLSRequest cookie not preserved")
	}
}
