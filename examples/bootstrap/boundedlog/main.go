// This finite caller environment imports the same Writer used by checker.Run.
// It verifies the lock/callback protocol, not os/exec, file I/O or context teardown.
package main

import (
	"sync"

	"github.com/fanmi/go-tla/internal/boundedlog"
)

type sink struct{}

func (sink) Write(p []byte) (int, error) { return len(p), nil }

type limitError int

func (limitError) Error() string { return "log limit" }

func write(w *boundedlog.Writer, done *sync.WaitGroup) {
	defer done.Done()
	_, _ = w.Write([]byte("x"))
}

func main() {
	// Scalar/error predicates are abstract: either write may report an error.
	// Each can call Cancel once, so capacity two cannot obstruct either callback.
	notifications := make(chan int, 2)
	w := &boundedlog.Writer{
		Output: sink{}, Remaining: 2, LimitError: limitError(0),
		Cancel: func(error) { notifications <- 1 },
	}
	var done sync.WaitGroup
	// Explicit Add/Done is intentional: WaitGroup.Go is outside the checked profile.
	done.Add(2)
	go write(w, &done)
	go write(w, &done)
	done.Wait()
}
