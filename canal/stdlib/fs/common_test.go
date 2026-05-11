package fs

import "testing"

func TestCleanPath(t *testing.T) {
	tests := map[string]string{
		"":                 "/",
		"logs/app.log":     "/logs/app.log",
		"/logs/./app.log":  "/logs/app.log",
		"/logs/../app.log": "/app.log",
	}

	for input, want := range tests {
		if got := cleanPath(input); got != want {
			t.Fatalf("cleanPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTrimNull(t *testing.T) {
	if got := trimNull([]byte{'a', 'b', 0, 'c'}); got != "ab" {
		t.Fatalf("trimNull returned %q", got)
	}
}

func TestCopyPathTerminatesAndCleans(t *testing.T) {
	var dst [maxPathLen]byte
	copyPath(dst[:], "programs/../main.pc")

	if got := trimNull(dst[:]); got != "/main.pc" {
		t.Fatalf("copyPath wrote %q", got)
	}
}
