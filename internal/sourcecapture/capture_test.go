package sourcecapture

import (
	"bytes"
	"strconv"
	"sync"
	"testing"
)

func TestRecordCopiesAndReplaces(t *testing.T) {
	c := &Collector{Files: make(map[string][]byte)}
	for _, tc := range []struct {
		name  string
		input []byte
	}{
		{"nil", nil},
		{"empty", make([]byte, 0, 32)},
		{"content", []byte("package example\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := bytes.Clone(tc.input)
			c.Record(tc.name, tc.input)
			got, ok := c.Files[tc.name]
			if !ok || !bytes.Equal(got, want) || (got == nil) != (tc.input == nil) {
				t.Fatalf("copy mismatch: got=%v want=%v", got, want)
			}
			if len(tc.input) != 0 {
				tc.input[0] = '!'
				if !bytes.Equal(got, want) {
					t.Fatal("retained caller's byte storage")
				}
			}
		})
	}
	first, last := []byte("first"), []byte("last")
	c.Record("same", first)
	c.Record("same", last)
	first[0], last[0] = '!', '!'
	if string(c.Files["same"]) != "last" {
		t.Fatal("replacement did not retain its own copy")
	}
}

func TestConcurrentRecords(t *testing.T) {
	c := &Collector{Files: make(map[string][]byte)}
	var done sync.WaitGroup
	for i := range 64 {
		done.Go(func() {
			name := strconv.Itoa(i)
			data := []byte("source-" + name)
			c.Record(name, data)
			data[0] = '!'
		})
	}
	done.Wait()
	if len(c.Files) != 64 {
		t.Fatalf("lost records: %d", len(c.Files))
	}
	for i := range 64 {
		name := strconv.Itoa(i)
		if string(c.Files[name]) != "source-"+name {
			t.Fatalf("wrong source for %s", name)
		}
	}
	// Like LoadContext, consumers read/clear only after parser callbacks finish.
	clear(c.Files)
	if len(c.Files) != 0 {
		t.Fatal("clear failed")
	}
}
