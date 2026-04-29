//go:build tinygo

package hal

// MemoryProtection is the hardware abstraction for memory isolation
type MemoryProtection interface {
	Init() error
	ConfigureDomain(config *DomainMemoryConfig) error
	SwitchContext(domainID uint16) error
	CheckAccess(addr uintptr, perms Permissions) bool
	Map(virt, phys uintptr, size uint32, perms Permissions) error
	Unmap(virt uintptr, size uint32) error
}

// Domain memory configuration
type DomainMemoryConfig struct {
	DomainID  uint16
	CodeVirt  uintptr
	CodePhys  uintptr
	CodeSize  uint32
	DataVirt  uintptr
	DataPhys  uintptr
	DataSize  uint32
	HeapVirt  uintptr
	HeapPhys  uintptr
	HeapSize  uint32
	StackVirt uintptr
	StackPhys uintptr
	StackSize uint32
}

// Memory permissions (architecture-independent)
type Permissions uint8

const (
	PermNone    Permissions = 0
	PermRead    Permissions = 1 << 0
	PermWrite   Permissions = 1 << 1
	PermExecute Permissions = 1 << 2
	PermUser    Permissions = 1 << 3
)

// Get the appropriate memory protection implementation
func NewMemoryProtection() MemoryProtection {
	return newMemoryProtection()
}

// Fallback implementation used when no architecture-specific backend is linked.
type noMemoryProtection struct{}

func newMemoryProtection() MemoryProtection { return &noMemoryProtection{} }

func (m *noMemoryProtection) Init() error                                      { return nil }
func (m *noMemoryProtection) ConfigureDomain(config *DomainMemoryConfig) error { return nil }
func (m *noMemoryProtection) SwitchContext(domainID uint16) error              { return nil }
func (m *noMemoryProtection) CheckAccess(addr uintptr, perms Permissions) bool { return true }
func (m *noMemoryProtection) Map(virt, phys uintptr, size uint32, perms Permissions) error {
	return nil
}
func (m *noMemoryProtection) Unmap(virt uintptr, size uint32) error { return nil }

// Fault handler callback
type FaultHandler func(addr uintptr, domainID uint16, faultType FaultType)

type FaultType uint8

const (
	FaultRead FaultType = iota
	FaultWrite
	FaultExecute
)

var faultHandler FaultHandler

func RegisterFaultHandler(handler FaultHandler) {
	faultHandler = handler
}

// Errors
var (
	ErrNotSupported   = &errorString{"not supported"}
	ErrInvalidDomain  = &errorString{"invalid domain"}
	ErrOutOfResources = &errorString{"out of resources"}
)

type errorString struct{ s string }

func (e *errorString) Error() string { return e.s }
