//go:build tinygo && esp32s3 && !idf

package kernel

import "errors"

// LoadDomain is a stub for non-IDF builds.
// Flash reading requires the IDF cache/MMU setup; without it we always
// signal "no domain" so bootDomains() falls back to goroutine spawning.
func LoadDomain(partitionOffset uint32) (entryPoint uint32, err error) {
	return 0, errors.New("loader: IDF required for flash loading")
}
