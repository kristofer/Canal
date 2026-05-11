//go:build tinygo

package main

import "stdlib/fs"

// Filesystem adapter for picoceci running as a Canal domain.
//
// This bridges picoceci's module loader to Canal's capability-based
// filesystem client so imports can be served by the SD card service.

func ReadFile(path string) ([]byte, error) {
	return fs.ReadFile(path)
}

func FileExists(path string) bool {
	info, err := fs.Stat(path)
	if err != nil {
		return false
	}
	return info.Exists
}

func ListDir(path string) ([]string, error) {
	items, err := fs.ReadDir(path)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}

	return names, nil
}
