package boundedlog

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
)

var _ io.Writer = (*Writer)(nil)

type outputFunc func([]byte) (int, error)

func (f outputFunc) Write(p []byte) (int, error) { return f(p) }

func TestWritePolicy(t *testing.T) {
	limitErr := errors.New("limit")
	ioErr := errors.New("output failed")
	for _, tc := range []struct {
		name, input   string
		budget        int64
		short         bool
		outputErr     error
		wantN         int
		wantRemaining int64
		wantErr       error
	}{
		{"below-limit", "ab", 3, false, nil, 2, 1, nil},
		{"exact-limit", "abc", 3, false, nil, 3, 0, nil},
		{"truncated", "abc", 2, false, nil, 2, 0, limitErr},
		{"zero-limit", "a", 0, false, nil, 0, 0, limitErr},
		{"empty", "", 0, false, nil, 0, 0, nil},
		{"short-success", "abc", 4, true, nil, 1, 3, limitErr},
		{"output-error", "abc", 3, true, ioErr, 1, 2, ioErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			var causes []error
			w := &Writer{Remaining: tc.budget, LimitError: limitErr}
			w.Output = outputFunc(func(p []byte) (int, error) {
				if w.mu.TryLock() {
					w.mu.Unlock()
					t.Error("output called without writer lock")
				}
				if tc.short {
					p = p[:1]
				}
				n, _ := output.Write(p)
				return n, tc.outputErr
			})
			w.Cancel = func(err error) {
				if w.mu.TryLock() {
					w.mu.Unlock()
					t.Error("cancellation called without writer lock")
				}
				causes = append(causes, err)
			}
			n, err := w.Write([]byte(tc.input))
			if n != tc.wantN || !errors.Is(err, tc.wantErr) || w.Remaining != tc.wantRemaining || !errors.Is(w.Err, tc.wantErr) {
				t.Fatalf("n=%d err=%v remaining=%d last=%v", n, err, w.Remaining, w.Err)
			}
			if output.String() != tc.input[:tc.wantN] {
				t.Fatal("wrong output prefix")
			}
			if tc.wantErr == nil && len(causes) != 0 || tc.wantErr != nil && (len(causes) != 1 || !errors.Is(causes[0], tc.wantErr)) {
				t.Fatalf("wrong cancellation causes: %v", causes)
			}
		})
	}
}

func TestLaterSuccessDoesNotClearRecordedError(t *testing.T) {
	failure := errors.New("first write failed")
	calls := 0
	w := &Writer{Remaining: 4, LimitError: errors.New("limit"), Cancel: func(error) {}}
	w.Output = outputFunc(func(p []byte) (int, error) {
		calls++
		if calls == 1 {
			return 0, failure
		}
		return len(p), nil
	})
	if _, err := w.Write([]byte("x")); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if n, err := w.Write([]byte("x")); n != 1 || err != nil {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if !errors.Is(w.Err, failure) || w.Remaining != 3 {
		t.Fatal("success reset previous error or wrong budget")
	}
}

func TestConcurrentWritesShareOneBudget(t *testing.T) {
	const budget = 251
	var output bytes.Buffer
	var written atomic.Int64
	var cancellations atomic.Int64
	limitErr := errors.New("limit")
	w := &Writer{Output: &output, Remaining: budget, LimitError: limitErr, Cancel: func(err error) {
		if !errors.Is(err, limitErr) {
			t.Errorf("unexpected cancellation: %v", err)
		}
		cancellations.Add(1)
	}}
	var done sync.WaitGroup
	for range 64 {
		done.Go(func() {
			n, err := w.Write([]byte("abcdefgh"))
			written.Add(int64(n))
			if err != nil && !errors.Is(err, limitErr) {
				t.Errorf("unexpected write error: %v", err)
			}
		})
	}
	done.Wait()
	if output.Len() != budget || written.Load() != budget || w.Remaining != 0 || cancellations.Load() == 0 || !errors.Is(w.Err, limitErr) {
		t.Fatalf("length=%d written=%d remaining=%d cancellations=%d error=%v", output.Len(), written.Load(), w.Remaining, cancellations.Load(), w.Err)
	}
}
