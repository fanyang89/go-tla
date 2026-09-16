package good_test

import (
	"testing"

	"github.com/fanmi/go-tla/examples/components/pipeline/good"
)

func TestRun(t *testing.T) {
	for _, capacity := range []int{0, 1, 2} {
		input, output := make(chan int, capacity), make(chan int, capacity)
		if got := good.Run(input, output); got != 84 {
			t.Fatalf("capacity %d: total=%d, want 84", capacity, got)
		}
	}
}
