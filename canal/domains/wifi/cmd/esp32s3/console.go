//go:build tinygo && esp32s3

package main

import (
	"strings"
	"unsafe"
)

// consoleIO defines the interface for REPL I/O
type consoleIO interface {
	ReadLine() (string, error)
	Write(data []byte) (int, error)
	Print(s string)
	Println(s string)
}

// tcpConsole implements consoleIO for TCP socket connections
type tcpConsole struct {
	fd           int32
	skipNextLF   bool
	recvBuf      [1]byte
	sendBuf      [256]byte
}

func newTCPConsole(fd int32) *tcpConsole {
	return &tcpConsole{fd: fd}
}

// ReadLine reads a line from the TCP socket with local echo
func (c *tcpConsole) ReadLine() (string, error) {
	var sb strings.Builder

	for {
		b, err := c.readByte()
		if err != nil {
			return sb.String(), err
		}

		// Handle CRLF
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
			c.writeByte('\r')
			c.writeByte('\n')
			break
		}

		// Handle backspace (BS=8, DEL=127)
		if b == 8 || b == 127 {
			s := sb.String()
			if len(s) > 0 {
				sb.Reset()
				sb.WriteString(s[:len(s)-1])
				// Erase: move back, overwrite with space, move back
				c.writeByte(8)
				c.writeByte(' ')
				c.writeByte(8)
			}
			continue
		}

		// Handle Ctrl-C (interrupt)
		if b == 3 {
			c.writeByte('^')
			c.writeByte('C')
			c.writeByte('\r')
			c.writeByte('\n')
			sb.Reset()
			return "", nil
		}

		// Handle Ctrl-D (EOF)
		if b == 4 {
			return "", &eofError{}
		}

		// Echo printable characters
		c.writeByte(b)
		sb.WriteByte(b)
	}

	return sb.String(), nil
}

// Write outputs bytes to the TCP socket
func (c *tcpConsole) Write(data []byte) (int, error) {
	totalSent := 0
	for totalSent < len(data) {
		chunk := data[totalSent:]
		if len(chunk) > len(c.sendBuf) {
			chunk = chunk[:len(c.sendBuf)]
		}

		n := lwipSend(c.fd, unsafe.Pointer(&chunk[0]), int32(len(chunk)), 0)
		if n <= 0 {
			return totalSent, &ioError{}
		}
		totalSent += int(n)
	}
	return totalSent, nil
}

// Print outputs a string to the TCP socket
func (c *tcpConsole) Print(s string) {
	c.Write([]byte(s))
}

// Println outputs a string with newline to the TCP socket
func (c *tcpConsole) Println(s string) {
	c.Write([]byte(s + "\r\n"))
}

// readByte reads a single byte from the TCP socket
func (c *tcpConsole) readByte() (byte, error) {
	for {
		n := lwipRecv(c.fd, unsafe.Pointer(&c.recvBuf[0]), 1, 0)
		if n == 1 {
			return c.recvBuf[0], nil
		}
		if n < 0 {
			return 0, &ioError{}
		}
		// Keep polling
		vTaskDelay(1)
	}
}

// writeByte writes a single byte to the TCP socket
func (c *tcpConsole) writeByte(b byte) {
	bb := [1]byte{b}
	lwipSend(c.fd, unsafe.Pointer(&bb[0]), 1, 0)
}

type ioError struct{}

func (ioError) Error() string { return "I/O error" }
