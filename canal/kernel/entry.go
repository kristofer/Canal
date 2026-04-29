//go:build tinygo && esp32s3

package kernel

// Boot enters the ESP32-S3 startup path.
func Boot() {
	app_main()
}
