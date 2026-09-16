package good_test

import (
	"testing"

	"github.com/fanmi/go-tla/examples/components/workerpool/good"
)

func TestRunBatch(t *testing.T) {
	pool := &good.Pool{Jobs: make(chan int), Results: make(chan int)}
	if got := pool.RunBatch(); got != 84 {
		t.Fatalf("batch total = %d, want 84", got)
	}
}
