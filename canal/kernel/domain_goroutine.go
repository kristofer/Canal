//go:build tinygo && !idf

package kernel

// spawnGoEntry launches the fallback domain entry as a goroutine.
// Used in non-IDF builds where the Go scheduler is available.
func spawnGoEntry(entry func(), name string, priority uint8) {
	go entry()
}
