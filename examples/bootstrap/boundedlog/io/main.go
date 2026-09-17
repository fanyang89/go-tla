// This finite environment models blocking output explicitly, not as pure I/O.
package main

import (
	"sync"

	"github.com/fanmi/go-tla/internal/boundedlog"
)

type gatedWriter chan int

func (g gatedWriter) Write(p []byte) (int, error) {
	<-g
	return len(p), nil
}

type limitError int

func (limitError) Error() string { return "log limit" }

func write(w *boundedlog.Writer, done *sync.WaitGroup) {
	defer done.Done()
	_, _ = w.Write([]byte("x"))
}

func release(g gatedWriter, done *sync.WaitGroup) {
	defer done.Done()
	g <- 1
	g <- 1
}

func main() {
	gate := make(gatedWriter)
	notifications := make(chan int, 2)
	w := &boundedlog.Writer{
		Output: gate, Remaining: 2, LimitError: limitError(0),
		Cancel: func(error) { notifications <- 1 },
	}
	var done sync.WaitGroup
	// Explicit Add/Done matches the checked profile, not WaitGroup.Go.
	done.Add(3)
	go write(w, &done)
	go write(w, &done)
	go release(gate, &done)
	done.Wait()
}
