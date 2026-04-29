# Quick Reference

## Essential Commands

```bash
# Setup
./scripts/setup.sh esp32s3

# Build
make TARGET=esp32s3

# Flash
make flash PORT=/dev/ttyUSB0

# Monitor
make monitor

# Clean
make clean

# Help
make help
```

## File Structure

```
kernel/         - Core kernel code
runtime/        - TinyGo extensions
stdlib/         - Standard library
domains/        - System services
examples/       - Sample applications
docs/           - Documentation
build/targets/  - Build configs
```
## Adding Code

1. Find file in conversation
2. Copy implementation
3. Paste into file
4. Save
5. Build and test

## Getting Help

- `README.md` - Overview
- `IMPLEMENTATION_GUIDE.md` - What to implement
- `docs/` - Detailed documentation
- GitHub Issues - Community support
