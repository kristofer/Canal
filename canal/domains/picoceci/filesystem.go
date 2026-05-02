//go:build tinygo

package main

// Filesystem adapter for picoceci running as a Canal domain.
//
// This bridges picoceci's module loader to Canal's capability-based
// filesystem. Currently a stub until Canal's sdcard domain is complete.
//
// TODO: Wire to stdlib/fs once Canal's filesystem capabilities are ready:
//   - Request "fs:/sdcard" capability
//   - Use capability to read module files
//   - Integrate with picoceci module.Resolver

// ReadFile reads a file from the filesystem via Canal capabilities.
// Currently returns an error as Canal's FS is still being implemented.
func ReadFile(path string) ([]byte, error) {
	// TODO: Implement using Canal stdlib/fs:
	//
	// f, err := fs.Open(path)
	// if err != nil {
	//     return nil, err
	// }
	// defer f.Close()
	//
	// buf := make([]byte, 4096)
	// n, err := f.Read(buf)
	// if err != nil {
	//     return nil, err
	// }
	// return buf[:n], nil

	println("[picoceci] ReadFile: " + path + " (not yet implemented)")
	return nil, errNoFS
}

// FileExists checks if a file exists.
// Currently always returns false.
func FileExists(path string) bool {
	// TODO: Implement using Canal capabilities
	return false
}

// ListDir lists files in a directory.
// Currently returns empty.
func ListDir(path string) ([]string, error) {
	// TODO: Implement using Canal capabilities
	return nil, errNoFS
}
