package frontend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/tools/go/packages"
)

func TestArrayRangeNativeEquivalence(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"collision", `package main;import "fmt";var COLLISION=99;func main(){for _,v:=range [1]int{1}{fmt.Println(v,COLLISION)}}`, "1 99\n"},
		{"snapshot", `package main;import "fmt";var a=[2]int{1,2};func main(){for i,v:=range a{a[1]=9;fmt.Println(i,v)}}`, "0 1\n1 2\n"},
		{"evaluate-once", `package main;import "fmt";var calls int;func values()[2]int{calls++;return [2]int{3,4}};func main(){for i,v:=range values(){fmt.Println(i,v)};fmt.Println(calls)}`, "0 3\n1 4\n1\n"},
		{"zero-evaluation", `package main;import "fmt";func values()[0]int{fmt.Println("evaluate");return [0]int{}};func main(){for i,v:=range values(){fmt.Println(i,v)}}`, "evaluate\n"},
		{"capture-and-defer", `package main;import "fmt";func main(){for i,v:=range [2]int{7,8}{defer func(){fmt.Println(i,v)}()}}`, "1 8\n0 7\n"},
		{"return", `package main;import "fmt";func value()int{for _,v:=range [2]int{7,8}{return v};return 0};func main(){fmt.Println(value())}`, "7\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "main.go")
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.26.2\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if tc.name == "collision" {
				name := "__gotla_array_0"
				for {
					source := strings.ReplaceAll(tc.source, "COLLISION", name)
					next := fmt.Sprintf("__gotla_array_%d", strings.Index(source, "for "))
					if next == name {
						tc.source = source
						break
					}
					name = next
				}
			}
			original := []byte(tc.source)
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			ps, err := packages.Load(&packages.Config{Context: t.Context(), Dir: dir, Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule}, ".")
			if err != nil {
				t.Fatal(err)
			}
			if packages.PrintErrors(ps) > 0 {
				t.Fatal("source failed type check")
			}
			overlay, proofs := normalizeLoops(ps, map[string][]byte{path: original})
			rewritten, ok := overlay[path]
			if !ok {
				t.Fatalf("array range not expanded: %+v", proofs)
			}
			unchanged, err := os.ReadFile(path)
			if err != nil || string(unchanged) != string(original) {
				t.Fatal("normalization modified source")
			}
			for _, source := range [][]byte{original, rewritten} {
				if err := os.WriteFile(path, source, 0600); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				cmd := exec.CommandContext(ctx, "go", "run", ".")
				cmd.Dir = dir
				out, err := cmd.CombinedOutput()
				cancel()
				if err != nil || string(out) != tc.want {
					t.Fatalf("native execution: %v output=%q want=%q\n%s", err, out, tc.want, source)
				}
			}
		})
	}
}

func TestArrayRangeRequiresPerIterationSemantics(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{"go.mod": "module fixture\n\ngo 1.21\n", "main.go": `package main;func main(){for _,v:=range [2]int{1,2}{defer func(){_=v}()}}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := LoadContext(t.Context(), dir, ".")
	if err != nil {
		t.Fatal(err)
	}
	p := BuildSSA(loaded)
	main, _ := p.Main()
	notes := p.LoopProofs(main)
	if len(notes) != 1 || notes[0].Reason == "" {
		t.Fatal("old loop variable semantics silently changed")
	}
}

func TestArrayRangeSourcePositions(t *testing.T) {
	dir := t.TempDir()
	source := []byte("package main\nfunc main(){\nfor _,v:=range [2]string{\"a\",\"b\"}{_=v}\n}\nfunc after(){for range 2{_=1}}\n")
	for name, data := range map[string][]byte{"go.mod": []byte("module fixture\n\ngo 1.26.2\n"), "main.go": source} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := LoadContext(t.Context(), dir, ".")
	if err != nil {
		t.Fatal(err)
	}
	p := BuildSSA(loaded)
	main, _ := p.Main()
	for _, name := range []string{"main", "after"} {
		f := main.Pkg.Func(name)
		notes := p.LoopProofs(f)
		if len(notes) != 1 || notes[0].Reason != "" || notes[0].Count != 2 {
			t.Fatalf("lost proof for %s: %+v", name, notes)
		}
		want := 3
		if name == "after" {
			want = 5
		}
		if notes[0].Source.Line != want {
			t.Fatal("loop source position changed")
		}
		if notes[0].ArrayRange != (name == "main") {
			t.Fatal("wrong loop kind")
		}
	}
	got, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil || string(got) != string(source) {
		t.Fatal("source modified")
	}
}
