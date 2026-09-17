package lowering

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/frontend"
)

func TestRuntimeProcsNativeCapacity(t *testing.T) {
	dir := t.TempDir()
	source := `package main;import("fmt";"runtime";"sync");var _ sync.Mutex;var c=make(chan int,runtime.GOMAXPROCS(0));func main(){fmt.Println(cap(c))}`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.26\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{1, 2, 3} {
		cmd := exec.CommandContext(t.Context(), "go", "run", ".")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), fmt.Sprintf("GOMAXPROCS=%d", n))
		native, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("native: %v: %s", err, native)
		}
		ps, err := frontend.Load(dir, ".")
		if err != nil {
			t.Fatal(err)
		}
		p := frontend.BuildSSA(ps)
		frontend.BuildCallGraph(p)
		m, err := Lower(p, Options{RuntimeProcs: n})
		if err != nil {
			t.Fatal(err)
		}
		// fmt's unrelated effects may refuse full emission; this compares the exact
		// source-located channel capacity, not a claimed proof of native printing.
		found := false
		for _, ch := range m.Channels {
			if ch.Source.Package == "fixture" {
				found = true
				if fmt.Sprint(ch.Capacity) != strings.TrimSpace(string(native)) {
					t.Fatal("profile capacity differs from native startup")
				}
			}
		}
		if !found {
			t.Fatal("profile channel missing")
		}
	}
}
