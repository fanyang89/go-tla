// Package good provides a mutex-protected counter and a finite concurrent batch.
package good

import "sync"

// Counter's zero value is ready for use. It must not be copied after first use.
type Counter struct {
	mu    sync.Mutex
	value int
}

func (c *Counter) Increment() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
}

func (c *Counter) Value() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// RunBatch starts and joins two increments, then reads a protected snapshot.
// The checking harness supplies a fresh Counter with no external users.
func (c *Counter) RunBatch() int {
	var workers sync.WaitGroup
	workers.Add(2)
	for range 2 {
		go func() {
			defer workers.Done()
			c.Increment()
		}()
	}
	workers.Wait()
	return c.Value()
}
