//go:build tinygo

package main

import (
	"strings"
	"unsafe"
)

// console provides line-based I/O for the REPL.
// Uses read/write over IDF VFS stdin/stdout.
//
// NOTE: Echo is handled here intentionally so tinygo monitor users see typed
// characters immediately.
type console struct {
	skipNextLF bool
}

func newConsole() *console {
	return &console{}
}

// ReadLine reads a line from the serial console with local echo.
// Miniterm doesn't echo by default, so picoceci handles it.
func (c *console) ReadLine() (string, error) {
	var sb strings.Builder

	for {
		b, err := readConsoleByte()
		if err != nil {
			return sb.String(), err
		}

		// Terminals commonly send CRLF. If the previous line ended on CR,
		// swallow the following LF so we don't emit a duplicate empty command.
		if c.skipNextLF && b == '\n' {
			c.skipNextLF = false
			continue
		}
		c.skipNextLF = false

		// Handle newline
		if b == '\n' || b == '\r' {
			if b == '\r' {
				c.skipNextLF = true
			}
			// Echo newline
			writeConsoleByte('\r')
			writeConsoleByte('\n')
			break
		}

		// Handle backspace (BS=8, DEL=127)
		if b == 8 || b == 127 {
			s := sb.String()
			if len(s) > 0 {
				sb.Reset()
				sb.WriteString(s[:len(s)-1])
				// Erase: move back, overwrite with space, move back
				writeConsoleByte(8)
				writeConsoleByte(' ')
				writeConsoleByte(8)
			}
			// Don't echo the backspace character itself
			continue
		}

		// Handle Ctrl-C (interrupt)
		if b == 3 {
			writeConsoleByte('^')
			writeConsoleByte('C')
			writeConsoleByte('\r')
			writeConsoleByte('\n')
			sb.Reset()
			return "", nil
		}

		// Handle Ctrl-D (EOF)
		if b == 4 {
			return "", &eofError{}
		}

		// Echo printable characters
		writeConsoleByte(b)
		sb.WriteByte(b)
	}

	return sb.String(), nil
}

// Write outputs bytes to the console.
func (c *console) Write(data []byte) (int, error) {
	for _, b := range data {
		writeConsoleByte(b)
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

func readConsoleByte() (byte, error) {
	var b [1]byte
	for {
		n := usbSerialJtagReadBytes(unsafe.Pointer(&b[0]), 1, 1)
		if n == 1 {
			return b[0], nil
		}
		// Keep polling cooperatively while no character is available.
		vTaskDelay(1)
	}
}

func writeConsoleByte(b byte) {
	bb := [1]byte{b}
	for {
		n := usbSerialJtagWriteBytes(unsafe.Pointer(&bb[0]), 1, 1)
		if n == 1 {
			_ = usbSerialJtagWaitTxDone(1)
			return
		}
		vTaskDelay(1)
	}
}

type ioError struct{}

func (ioError) Error() string { return "console I/O error" }
