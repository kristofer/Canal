# Network-Accessible picoceci Interpreter

The WiFi domain now hosts a network-accessible picoceci interpreter, allowing you to connect to your ESP32-S3 device over TCP/IP and interact with the picoceci REPL remotely.

## Features

- **TCP Shell Access**: Connect to picoceci interpreter over WiFi using nc (netcat) or similar
- **WiFi Configuration**: SSID and password configured at build time via environment variables
- **Logging Separation**: USB Serial/JTAG now dedicated to logging messages only
- **Network I/O**: Full REPL interaction over TCP socket with line editing support

## Building

**Important:** You must rebuild the kernel first to include WiFi/networking support:

```bash
rm -rf build/idf-app/build && make build
```

Then build and flash the WiFi domain with your network credentials:

```bash
make flash-wifi WIFI_SSID=YourNetworkName WIFI_PASSWORD=YourPassword PORT=/dev/cu.usbmodem11201
```

Or for a complete clean build:

```bash
rm -rf build/idf-app/build && \
  make flash PORT=/dev/cu.usbmodem11201 && \
  make flash-wifi WIFI_SSID=YourNetworkName WIFI_PASSWORD=YourPassword PORT=/dev/cu.usbmodem11201
```

Optional: Customize the TCP port (default is 2323):

```bash
make flash-wifi WIFI_SSID=YourNetworkName WIFI_PASSWORD=YourPassword WIFI_PORT=2323
```

## Usage

1. **Flash the WiFi domain** with your credentials
2. **Monitor the serial output** to see the assigned IP address:

   ```bash
   make monitor
   ```

3. **Connect via nc (netcat)** from your computer:

   ```bash
   nc <device-ip> 2323
   ```

   Or use netcat:

   ```bash
   nc <device-ip> 2323
   ```

## Example Session

```
$ make monitor
[WiFi] Domain 1 starting from flash
[WiFi] picoceci network interpreter v0.1.0-dev
[WiFi] Initializing WiFi...
[WiFi] Connecting to MyNetwork...
[WiFi] Waiting for IP address...
[WiFi] Connected!
[TCP] Creating server socket...
[TCP] Server listening on port 2323
[WiFi] Ready! Connect with: nc 192.168.1.100 2323
[WiFi] USB serial is now for logging only
[TCP] Waiting for client connection...
```

Then from another terminal:

```
$ nc (netcat) 192.168.1.100 2323
Trying 192.168.1.100...
Connected to 192.168.1.100.
Escape character is '^]'.

=== picoceci over TCP ===

[picoceci] Ready v0.1.0-dev
  tip: type '---' to enter/exit paste mode for multi-line programs

> 1 + 2
=> 3
>
```

## Architecture

The implementation merges the picoceci interpreter into the WiFi domain:

- **WiFi domain**: Handles network connectivity and TCP server
- **TCP Console**: Implements consoleIO interface for network I/O
- **REPL**: Full picoceci interpreter running over TCP connection
- **USB Serial**: Dedicated to system logging only

## Files Modified/Created

- `canal/Makefile` - Added WIFI_SSID, WIFI_PASSWORD, WIFI_PORT variables
- `canal/domains/wifi/cmd/esp32s3/main.go` - WiFi domain orchestration
- `canal/domains/wifi/cmd/esp32s3/esp_idf.go` - ESP-IDF WiFi and socket bindings
- `canal/domains/wifi/cmd/esp32s3/network.go` - WiFi connection and TCP server
- `canal/domains/wifi/cmd/esp32s3/console.go` - Network console implementation
- `canal/domains/wifi/cmd/esp32s3/interpreter.go` - picoceci REPL logic

## Troubleshooting

**WiFi won't connect:**

- Check SSID and password are correct
- Ensure your network is 2.4GHz (ESP32-S3 doesn't support 5GHz)
- Check serial monitor for error messages

**Can't connect via nc (netcat):**

- Verify device IP address from serial monitor
- Ensure device and computer are on same network
- Check firewall settings

**Connection drops:**

- WiFi signal strength may be weak
- Try moving device closer to access point
- Check router logs

## Future Enhancements

- mDNS/Bonjour discovery (connect via hostname instead of IP)
- TLS/SSL support for encrypted connections
- Multiple simultaneous client connections
- WebSocket support for browser-based REPL
- WiFi configuration via web portal (no rebuild needed)
