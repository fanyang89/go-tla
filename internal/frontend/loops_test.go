package frontend_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fanmi/go-tla/internal/frontend"
)

func TestLoopNormalizationDoesNotModifySource(t *testing.T) {
	dir := t.TempDir()
	source := []byte("package main\nfunc main(){for range 2 {_ = 1}}\nfunc after(){for range 3 {_ = 2}}\n")
	for name, data := range map[string][]byte{"go.mod": []byte("module fixture\n\ngo 1.26.2\n"), "main.go": source} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := frontend.LoadContext(t.Context(), dir, ".")
	if err != nil {
		t.Fatal(err)
	}
	p := frontend.BuildSSA(loaded)
	main, _ := p.Main()
	for _, fn := range []string{"main", "after"} {
		f := main.Pkg.Func(fn)
		notes := p.LoopProofs(f)
		if len(notes) != 1 || notes[0].Reason != "" {
			t.Fatalf("lost function proof for %s: %+v", fn, notes)
		}
		line := 2
		if fn == "after" {
			line = 3
		}
		if notes[0].Source.Line != line || p.Fset.Position(f.Pos()).Line != line {
			t.Fatal("line directives corrupted subsequent function ownership")
		}
	}
	got, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(source) {
		t.Fatal("normalization modified input source")
	}
}
