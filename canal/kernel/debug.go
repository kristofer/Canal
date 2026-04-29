//go:build tinygo

package kernel

import "machine"

func debugWrite(data []byte) {
	for _, b := range data {
		machine.Serial.WriteByte(b)
	}
}
