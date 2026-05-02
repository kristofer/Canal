//go:build tinygo

package main

import (
	"machine"
	"strings"
	"time"
)

// console provides line-based I/O for the REPL.
// Uses machine.Serial which is USB CDC on ESP32-S3.
type console struct{}

func newConsole() *console {
	return &console{}
}

// ReadLine reads a line from the serial console with echo and backspace support.
func (c *console) ReadLine() (string, error) {
	var sb strings.Builder

	for {
		// Wait for input with a small yield to prevent tight spinning
		for machine.Serial.Buffered() == 0 {
			time.Sleep(time.Millisecond)
		}

		b, err := machine.Serial.ReadByte()
		if err != nil {
			return sb.String(), err
		}

		// Echo the character
		machine.Serial.WriteByte(b)

		// Handle newline
		if b == '\n' || b == '\r' {
			if b == '\r' {
				machine.Serial.WriteByte('\n')
			}
			break
		}

		// Handle backspace (BS=8, DEL=127)
		if b == 8 || b == 127 {
			s := sb.String()
			if len(s) > 0 {
				sb.Reset()
				sb.WriteString(s[:len(s)-1])
				// Erase character on terminal: backspace, space, backspace
				machine.Serial.WriteByte(8)
				machine.Serial.WriteByte(' ')
				machine.Serial.WriteByte(8)
			}
			continue
		}

		// Handle Ctrl-C (interrupt)
		if b == 3 {
			println("^C")
			sb.Reset()
			return "", nil
		}

		// Handle Ctrl-D (EOF)
		if b == 4 {
			return "", &eofError{}
		}

		sb.WriteByte(b)
	}

	return sb.String(), nil
}

// Write outputs bytes to the console.
func (c *console) Write(data []byte) (int, error) {
	for _, b := range data {
		machine.Serial.WriteByte(b)
	}
	return len(data), nil
}

// Print outputs a string to the console.
func (c *console) Print(s string) {
	c.Write([]byte(s))
}

// Println outputs a string with newline to the console.
func (c *console) Println(s string) {
	c.Write([]byte(s + "\n"))
}

type eofError struct{}

func (eofError) Error() string { return "EOF" }
