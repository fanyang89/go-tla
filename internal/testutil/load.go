// Package testutil loads small, real Go modules for analysis-pass tests.
package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fanmi/go-tla/internal/frontend"
)

func Load(t *testing.T, source string) *frontend.Program {
	t.Helper()
	dir := t.TempDir()
	for name, text := range map[string]string{"go.mod": "module fixture\n\ngo 1.26\n", "main.go": source} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ps, err := frontend.Load(dir, ".")
	if err != nil {
		t.Fatal(err)
	}
	p := frontend.BuildSSA(ps)
	frontend.BuildCallGraph(p)
	return p
}
