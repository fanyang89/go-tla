// This environment is intentionally unsupported and must not be run natively.
// Cancellation re-enters the same writer while its mutex remains held.
package main

import "github.com/fanmi/go-tla/internal/boundedlog"

type sink struct{}

func (sink) Write(p []byte) (int, error) { return len(p), nil }

type limitError int

func (limitError) Error() string { return "log limit" }

func main() {
	var w *boundedlog.Writer
	w = &boundedlog.Writer{
		Output: sink{}, Remaining: 0, LimitError: limitError(0),
		Cancel: func(error) { _, _ = w.Write([]byte("again")) },
	}
	_, _ = w.Write([]byte("x"))
}
