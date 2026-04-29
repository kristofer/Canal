//go:build tinygo && esp32s3

package main

import "kernel"

func main() {
	kernel.Boot()
}
