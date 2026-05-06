//go:build tinygo && esp32s3 && idf

package kernel

func allowInKernelFallback() bool {
	return false
}
