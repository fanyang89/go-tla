// A finite caller environment for the same collector used by frontend.LoadContext.
// This models the capture lock protocol, not go/packages scheduling or parsing.
package main

import (
	"sync"

	"github.com/fanmi/go-tla/internal/sourcecapture"
)

func record(c *sourcecapture.Collector, name string, done *sync.WaitGroup) {
	defer done.Done()
	c.Record(name, []byte("package example\n"))
}

func main() {
	c := &sourcecapture.Collector{Files: make(map[string][]byte)}
	var done sync.WaitGroup
	// Explicit Add/Done matches the checked profile, not WaitGroup.Go.
	done.Add(2)
	go record(c, "one.go", &done)
	go record(c, "two.go", &done)
	done.Wait()
}
