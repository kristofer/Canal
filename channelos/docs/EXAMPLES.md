# Example Applications

## hello-world

Minimal domain demonstrating basic functionality.

Location: `examples/hello-world/`

## https-client

TLS connection example showing secure communication.

Location: `examples/https-client/`

## sensor-logger

Reads I2C sensor and logs to SD card.

Location: `examples/sensor-logger/`

## Building Examples

```bash
cd examples/hello-world
tinygo build -target=esp32s3 -o hello.elf main.go
```

## Creating New Examples

1. Create directory under `examples/`
2. Write `main.go`
3. Add README.md
4. Test and document
