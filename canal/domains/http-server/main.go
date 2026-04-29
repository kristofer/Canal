//go:build tinygo && esp32s3

package main

import (
    "stdlib/wifi"
    "stdlib/net"
    "time"
)

// HTTP server domain - serves web pages
// Uses WiFi capability for networking

func main() {
    println("[HTTP] Server starting...")

    // Initialize WiFi
    err := wifi.Init()
    if err != nil {
        println("[HTTP] WiFi init failed:", err.Error())
        return
    }

    // Scan for networks
    println("[HTTP] Scanning...")
    aps, _ := wifi.Scan()
    for _, ap := range aps {
        println("  Found:", ap.SSID, "RSSI:", ap.RSSI)
    }

    // Connect to WiFi
    println("[HTTP] Connecting to WiFi...")
    err = wifi.Connect("MyNetwork", "MyPassword", 30000)
    if err != nil {
        println("[HTTP] Connect failed:", err.Error())
        return
    }

    // Wait for connection
    time.Sleep(2 * time.Second)

    status, _ := wifi.Status()
    if !status.Connected {
        println("[HTTP] Not connected")
        return
    }

    println("[HTTP] Connected! IP:", status.IP)

    // Create TCP listener
    listener, err := net.Listen("tcp", ":80")
    if err != nil {
        println("[HTTP] Listen failed:", err.Error())
        return
    }

    println("[HTTP] Listening on port 80")

    // Accept loop
    requestCount := 0
    for {
        conn, err := listener.Accept()
        if err != nil {
            println("[HTTP] Accept error:", err.Error())
            continue
        }

        requestCount++
        println("[HTTP] Request", requestCount)

        // Handle in goroutine
        go handleConnection(conn)
    }
}

func handleConnection(conn *net.Conn) {
    defer conn.Close()

    // Read HTTP request
    buf := make([]byte, 1024)
    n, err := conn.Read(buf)
    if err != nil {
        return
    }

    request := string(buf[:n])
    println("[HTTP] Request:", request[:min(len(request), 50)])

    // Parse request (simplified)
    path := parseHTTPPath(request)

    // Route
    var response string
    switch path {
    case "/":
        response = htmlResponse(indexHTML)
    case "/api/status":
        response = jsonResponse(getStatus())
    case "/api/led":
        response = jsonResponse(toggleLED())
    default:
        response = htmlResponse(notFoundHTML)
    }

    // Send response
    conn.Write([]byte(response))
}

func parseHTTPPath(request string) string {
    // Simple parser: "GET /path HTTP/1.1"
    // Would use proper HTTP parser in production
    return "/"
}

func htmlResponse(html string) string {
    return "HTTP/1.1 200 OK\r\n" +
        "Content-Type: text/html\r\n" +
        "Content-Length: " + itoa(len(html)) + "\r\n" +
        "\r\n" +
        html
}

func jsonResponse(json string) string {
    return "HTTP/1.1 200 OK\r\n" +
        "Content-Type: application/json\r\n" +
        "Content-Length: " + itoa(len(json)) + "\r\n" +
        "\r\n" +
        json
}

func getStatus() string {
    status, _ := wifi.Status()

    return `{"connected":` + btoa(status.Connected) +
        `,"ssid":"` + status.SSID + `"` +
        `,"ip":"` + status.IP + `"` +
        `,"rssi":` + itoa(int(status.RSSI)) + `}`
}

func toggleLED() string {
    // Would send message to LED domain via capability
    return `{"led":"toggled"}`
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func itoa(n int) string {
    // Simple int to string
    if n == 0 {
        return "0"
    }

    negative := n < 0
    if negative {
        n = -n
    }

    buf := [20]byte{}
    i := len(buf) - 1

    for n > 0 {
        buf[i] = byte('0' + n%10)
        n /= 10
        i--
    }

    if negative {
        buf[i] = '-'
        i--
    }

    return string(buf[i+1:])
}

func btoa(b bool) string {
    if b {
        return "true"
    }
    return "false"
}

const indexHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Canal ESP32-S3</title>
    <style>
        body { font-family: monospace; padding: 20px; }
        .status { background: #f0f0f0; padding: 10px; margin: 10px 0; }
        button { padding: 10px 20px; font-size: 16px; }
    </style>
</head>
<body>
    <h1>Canal on ESP32-S3</h1>
    <div class="status" id="status">Loading...</div>
    <button onclick="refresh()">Refresh Status</button>
    <button onclick="toggleLED()">Toggle LED</button>

    <script>
        function refresh() {
            fetch('/api/status')
                .then(r => r.json())
                .then(data => {
                    document.getElementById('status').innerHTML =
                        'Connected: ' + data.connected + '<br>' +
                        'SSID: ' + data.ssid + '<br>' +
                        'IP: ' + data.ip + '<br>' +
                        'RSSI: ' + data.rssi + ' dBm';
                });
        }

        function toggleLED() {
            fetch('/api/led');
        }

        refresh();
        setInterval(refresh, 5000);
    </script>
</body>
</html>`

const notFoundHTML = `<!DOCTYPE html>
<html>
<body>
    <h1>404 Not Found</h1>
</body>
</html>`
