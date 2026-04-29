// domains/https-client/main.go
//go:build tinygo

package main

import (
    "stdlib/wifi"
    "stdlib/tls"
    "time"
)

func main() {
    println("[HTTPS] Client starting...")

    // Connect to WiFi
    wifi.Init()
    wifi.Connect("MyNetwork", "MyPassword", 30000)

    time.Sleep(2 * time.Second)

    // Make HTTPS request
    conn, err := tls.Dial("tcp", "api.github.com:443")
    if err != nil {
        println("[HTTPS] Dial error:", err.Error())
        return
    }
    defer conn.Close()

    println("[HTTPS] Connected via TLS!")

    // Send HTTP request
    request := "GET /users/github HTTP/1.1\r\n" +
        "Host: api.github.com\r\n" +
        "User-Agent: ChannelOS/1.0\r\n" +
        "Connection: close\r\n" +
        "\r\n"

    _, err = conn.Write([]byte(request))
    if err != nil {
        println("[HTTPS] Write error:", err.Error())
        return
    }

    println("[HTTPS] Request sent")

    // Read response
    buf := make([]byte, 4096)
    n, err := conn.Read(buf)
    if err != nil {
        println("[HTTPS] Read error:", err.Error())
        return
    }

    println("[HTTPS] Response:")
    println(string(buf[:n]))
}
