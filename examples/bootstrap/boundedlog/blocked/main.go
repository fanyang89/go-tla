// This deliberately blocking environment must never be run as a native program.
// A zero budget forces a nonempty write to report LimitError; its cancellation
// callback cannot send on an unbuffered channel with no receiver.
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
	notifications := make(chan int)
	w := &boundedlog.Writer{
		Output: sink{}, Remaining: 0, LimitError: limitError(0),
		Cancel: func(error) { notifications <- 1 },
	}
	var done sync.WaitGroup
	// Explicit Add/Done matches the checked profile, not WaitGroup.Go.
	done.Add(2)
	go write(w, &done)
	go write(w, &done)
	done.Wait()
}
