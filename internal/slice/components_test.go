package slice_test

import (
	"testing"

	cslice "github.com/fanmi/go-tla/internal/slice"
	"github.com/fanmi/go-tla/internal/testutil"
)

func TestCyclicComponents(t *testing.T) {
	for _, c := range []struct {
		name, body string
		count      int
	}{
		{"acyclic", `ch:=make(chan int);close(ch)`, 0},
		{"single", `ch:=make(chan int);for range ch {}`, 1},
		{"sequential", `ch:=make(chan int);for range ch {};for range ch {}`, 2},
		{"nested", `ch:=make(chan int);for range ch {for range ch {}}`, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := testutil.Load(t, "package main;func main(){"+c.body+"}")
			f, err := p.Main()
			if err != nil {
				t.Fatal(err)
			}
			components := cslice.CyclicComponents(f)
			if len(components) != c.count {
				t.Fatalf("components=%d, want %d", len(components), c.count)
			}
			seen := map[int]bool{}
			for _, component := range components {
				for _, block := range component {
					if seen[block.Index] {
						t.Fatal("overlapping SCCs")
					}
					seen[block.Index] = true
				}
			}
		})
	}
}
