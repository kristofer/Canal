// domains/logger/main.go
//go:build tinygo

package main

import (
    "stdlib/fs"
    "time"
)

func main() {
    println("[Logger] Starting...")

    // Open log file (will be created if doesn't exist)
    logFile, err := fs.OpenMode("/logs/system.log",
        fs.ModeReadWrite | fs.ModeCreate | fs.ModeAppend)
    if err != nil {
        println("[Logger] Failed to open log:", err.Error())
        return
    }
    defer logFile.Close()

    println("[Logger] Log file opened")

    // Write log entries
    for {
        timestamp := time.Now().Format("2006-01-02 15:04:05")
        entry := timestamp + " [INFO] System running\n"

        _, err := logFile.Write([]byte(entry))
        if err != nil {
            println("[Logger] Write error:", err.Error())
        } else {
            // Flush to disk
            logFile.Sync()
        }

        time.Sleep(10 * time.Second)
    }
}
