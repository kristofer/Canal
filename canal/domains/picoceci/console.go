//go:build tinygo

package main

import (
	"machine"
	"strings"
	"time"
)

// console provides line-based I/O for the REPL.
// Uses machine.Serial which is USB CDC on ESP32-S3.
//
// NOTE: Echo is handled by the terminal (miniterm with local echo) or
// Canal's serial layer. This console does NOT echo to avoid double characters.
type console struct{}

func newConsole() *console {
	return &console{}
}

// ReadLine reads a line from the serial console with local echo.
// Miniterm doesn't echo by default, so picoceci handles it.
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

		// Handle newline
		if b == '\n' || b == '\r' {
			// Echo newline
			machine.Serial.WriteByte('\r')
			machine.Serial.WriteByte('\n')
			// If we got \r, consume any following \n (handles \r\n sequence)
			if b == '\r' {
				time.Sleep(time.Millisecond) // Brief wait for \n to arrive
				if machine.Serial.Buffered() > 0 {
					next, _ := machine.Serial.ReadByte()
					if next != '\n' {
						// Not a \n, put it back by processing it next iteration
						// Actually we can't put it back, so just ignore non-\n
						// This is fine since \r alone is rare
					}
				}
			}
			break
		}

		// Handle backspace (BS=8, DEL=127)
		if b == 8 || b == 127 {
			s := sb.String()
			if len(s) > 0 {
				sb.Reset()
				sb.WriteString(s[:len(s)-1])
				// Erase: move back, overwrite with space, move back
				machine.Serial.WriteByte(8)
				machine.Serial.WriteByte(' ')
				machine.Serial.WriteByte(8)
			}
			// Don't echo the backspace character itself
			continue
		}

		// Handle Ctrl-C (interrupt)
		if b == 3 {
			machine.Serial.WriteByte('^')
			machine.Serial.WriteByte('C')
			machine.Serial.WriteByte('\r')
			machine.Serial.WriteByte('\n')
			sb.Reset()
			return "", nil
		}

		// Handle Ctrl-D (EOF)
		if b == 4 {
			return "", &eofError{}
		}

		// Echo printable characters
		machine.Serial.WriteByte(b)
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
