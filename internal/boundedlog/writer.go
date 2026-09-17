// Package boundedlog serializes bounded log writes and reports errors through a
// caller-supplied cancellation function. It has no process or context dependency.
package boundedlog

import "sync"

// Writer is the checker runner's shared output writer. Initialize Output, Cancel,
// LimitError (non-nil) and Remaining (nonnegative) before use. Output must obey the
// io.Writer contract. Do not change configuration or copy Writer after first use.
// Read Remaining and Err only after every Write call has completed.
//
// Output.Write and Cancel run while the mutex is held, as in the original runner.
// They must not synchronously re-enter this writer. Their blocking behavior is
// part of the caller's environment, not made safe by this type.
type Writer struct {
	mu         sync.Mutex
	Output     interface{ Write([]byte) (int, error) }
	Remaining  int64
	Cancel     func(error)
	LimitError error
	Err        error
}

// Write preserves the runner's original truncation/error policy: a nil-error
// short write is reported using LimitError, even if caused by the output sink.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	data := p
	if int64(len(data)) > w.Remaining {
		data = data[:w.Remaining]
	}
	n, err := w.Output.Write(data)
	w.Remaining -= int64(n)
	if err == nil && n != len(p) {
		err = w.LimitError
	}
	if err != nil {
		w.Err = err
		w.Cancel(err)
	}
	return n, err
}
