package main

import (
	"testing"

	"github.com/fanmi/go-tla/internal/checker"
)

func TestCheckCounterComponents(t *testing.T) {
	for _, c := range []struct {
		variant                string
		want                   checker.Status
		locations, transitions int
	}{
		{"good", checker.Passed, 23, 27}, {"bad", checker.Deadlock, 19, 19},
	} {
		t.Run(c.variant, func(t *testing.T) {
			m := checkComponent(t, "counter/"+c.variant, c.want)
			stats := m.Statistics()
			if stats.Mutexes != 1 || stats.Channels != 0 || stats.Locations != c.locations || stats.Transitions != c.transitions {
				t.Fatalf("counter topology changed: %+v", stats)
			}
		})
	}
}
