//go:build tinygo

package main

import (
    "time"
)

func main() {
    println("[Hello] ChannelOS domain starting...")

    count := 0
    for {
        println("[Hello] Iteration:", count)
        count++
        time.Sleep(1 * time.Second)
    }
}
