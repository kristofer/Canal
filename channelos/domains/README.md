# System Service Domains

Each domain is an isolated service running in its own memory space.

## Core Domains

- **wifi/** - WiFi hardware management
- **tls/** - TLS/crypto operations
- **sdcard/** - Filesystem service
- **http-server/** - HTTP server example
- **logger/** - Logging service
- **led-blinker/** - Simple example

## Implementation

Each domain has:
- `main.go` - Service entry point
- `protocol.go` - Message types
- Supporting files for domain logic

## TODO

Copy full implementations from conversation.
Search for "domains/wifi/main.go", etc.
