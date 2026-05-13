//go:build tinygo && esp32s3

package main

// Capability symbol names for Canal capability requests.
// Usage: Canal capability: #fsRead
const (
	CapabilityFSRead      = "fsRead"
	CapabilityFSWrite     = "fsWrite"
	CapabilityFSReadWrite = "fsReadWrite"
)

// All known capability symbols
var knownCapabilities = []string{
	CapabilityFSRead,
	CapabilityFSWrite,
	CapabilityFSReadWrite,
}
