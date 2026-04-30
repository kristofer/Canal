//go:build tinygo && esp32s3 && !idf

package kernel

import "errors"

// LoadDomain is a stub for non-IDF development builds only.
// Flash reading requires IDF cache/MMU setup; without it we always signal
// "no domain" so bootDomains() falls back to goroutine spawning.
//
// NOTE: Production builds should use USE_IDF=1 (the default) so that the
// real ELF loader in loader.go is compiled instead of this stub.
func LoadDomain(partitionOffset uint32) (entryPoint uint32, err error) {
	return 0, errors.New("loader: IDF required for flash loading")
}
