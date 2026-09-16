package good_test

import (
	"testing"

	"github.com/fanmi/go-tla/examples/components/counter/good"
)

func TestRunBatch(t *testing.T) {
	counter := &good.Counter{}
	if got := counter.RunBatch(); got != 2 {
		t.Fatalf("batch snapshot = %d, want 2", got)
	}
	counter.Increment()
	if got := counter.Value(); got != 3 {
		t.Fatalf("counter remains locked or has value %d, want 3", got)
	}
}
