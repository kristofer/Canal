# Capability System

Capabilities are unforgeable tokens granting specific access rights.

## Basic Usage

```go
// Request capability
cap, err := runtime.RequestCap("service:wifi", runtime.RightReadWrite)

// Use capability
err = runtime.CapSend(cap, request)
```

## Security Properties

### Unforgeable
Capabilities are kernel-managed IDs. User code cannot create or guess them.

### Transferable
Capabilities can be sent over channels to delegate authority.

### Revocable
The owner can revoke a capability at any time.

### Type-Safe
Channels enforce message types at compile time.

## Examples

### File Access
```go
file, _ := fs.Open("/logs/app.log")
// File is a capability to that specific file
```

### Network Access
```go
conn, _ := net.Dial("tcp", "example.com:80")
// conn is a capability to that connection
```

### Service Access
```go
wifi, _ := wifi.Init()
// wifi is a capability to WiFi service
```

## Permission Model

Capabilities have rights:
- Read
- Write
- Execute
- Grant (can delegate)
- Revoke (can destroy)

## Defense in Depth

Even if a domain is compromised:
- Cannot access hardware directly
- Cannot forge capabilities
- Cannot access other domains' memory
- Limited to capabilities it holds
