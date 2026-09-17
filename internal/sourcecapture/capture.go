// Package sourcecapture records source bytes supplied by concurrent parser calls.
package sourcecapture

import (
	"slices"
	"sync"
)

// Collector serializes Record calls. Initialize Files before use, do not copy a
// Collector after first use, and access Files directly only after all calls finish.
// Callers must not mutate src concurrently with Record.
type Collector struct {
	mu    sync.Mutex
	Files map[string][]byte
}

// Record retains a byte copy, preserving nil versus non-nil empty input. Slice
// capacity and backing-storage retention are not part of the capture contract.
func (c *Collector) Record(filename string, src []byte) {
	c.mu.Lock()
	c.Files[filename] = slices.Clone(src)
	c.mu.Unlock()
}
