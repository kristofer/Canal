package channel

import "errors"

// ServiceID is the numeric identifier of a named service channel.
type ServiceID uint8

const (
	ServiceFS   ServiceID = 1
	ServiceWiFi ServiceID = 2
	ServiceTLS  ServiceID = 3
)

// SchemaVersion tags a service channel's message schema for compatibility checks.
type SchemaVersion uint8

const SchemaV1 SchemaVersion = 1

// Entry describes one registered service channel.
type Entry struct {
	ID     ServiceID
	Name   string
	Schema SchemaVersion
}

var registry = [3]Entry{
	{ServiceFS, "fs", SchemaV1},
	{ServiceWiFi, "wifi", SchemaV1},
	{ServiceTLS, "tls", SchemaV1},
}

// Lookup returns the entry for the named service, or false if unknown.
func Lookup(name string) (Entry, bool) {
	for _, e := range registry {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}

// LookupByID returns the entry for the given ServiceID, or false if unknown.
func LookupByID(id ServiceID) (Entry, bool) {
	for _, e := range registry {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// Validate returns an error if serviceID is not registered or schema mismatches.
func Validate(serviceID ServiceID, schema SchemaVersion) error {
	e, ok := LookupByID(serviceID)
	if !ok {
		return ErrUnknownService
	}
	if e.Schema != schema {
		return ErrSchemaMismatch
	}
	return nil
}

var (
	ErrUnknownService = errors.New("channel: unknown service")
	ErrSchemaMismatch = errors.New("channel: schema version mismatch")
)
