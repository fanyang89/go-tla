package effects

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/frontend"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

func TestScalarSprintVariants(t *testing.T) {
	for _, name := range []string{"Sprint", "Sprintln"} {
		for _, tc := range []struct {
			declarations, args string
			want               bool
		}{
			{"", `"x",2,true,nil`, true},
			{"", ``, true},
			{`type N int;func(n N)String()string{panic("callback")};`, `N(1)`, false},
			{"var shared=[]any{1};", `shared...`, false},
			{"", `[]byte("x")`, false},
		} {
			t.Run(name+"/"+tc.args, func(t *testing.T) {
				p := testutil.Load(t, `package main;import "fmt";`+tc.declarations+`func main(){_=fmt.`+name+`(`+tc.args+`)}`)
				main, _ := p.Main()
				a := New(p, nil)
				found := false
				for _, bb := range main.Blocks {
					for _, i := range bb.Instrs {
						if c, ok := i.(*ssa.Call); ok {
							found = true
							if (a.ProveScalarFormat(c) != nil) != tc.want || (a.Call(c).Kind == ScalarFormat) != tc.want {
								t.Fatal("scalar formatting boundary mismatch")
							}
						}
					}
				}
				if !found {
					t.Fatal("format call missing")
				}
			})
		}
	}
}

func TestScalarSprintWrapperFreshness(t *testing.T) {
	for _, name := range []string{"Sprint", "Sprintln"} {
		for _, broken := range []string{"method", "graph", "escape", "arity"} {
			t.Run(name+"/"+broken, func(t *testing.T) {
				p := testutil.Load(t, `package main;import "fmt";var shared []any;func main(){a:=[]any{1};_=fmt.`+name+`(a...);shared=nil}`)
				main, _ := p.Main()
				a := New(p, nil)
				var site *ssa.Call
				for _, bb := range main.Blocks {
					for _, i := range bb.Instrs {
						if c, ok := i.(*ssa.Call); ok {
							site = c
						}
					}
				}
				if site == nil || a.ProveScalarFormat(site) == nil {
					t.Fatal("baseline missing")
				}
				f := p.CallTarget(site)
				switch broken {
				case "method":
					method := f.Blocks[0].Instrs[1].(*ssa.Call)
					other := "Sprint"
					if name == other {
						other = "Sprintln"
					}
					replacement := f.Pkg.Func(other).Blocks[0].Instrs[1].(*ssa.Call).Common().StaticCallee()
					method.Common().Value = replacement
					callgraph.AddEdge(p.Calls.Nodes[f], method, p.Calls.CreateNode(replacement))
					if p.CallTarget(method) != replacement {
						t.Fatal("replacement graph missing")
					}
				case "graph":
					p.Calls.Nodes[f].Out = nil
				case "escape":
					for _, bb := range main.Blocks {
						for _, i := range bb.Instrs {
							if s, ok := i.(*ssa.Store); ok {
								if _, global := s.Addr.(*ssa.Global); global {
									s.Val = site.Common().Args[0]
								}
							}
						}
					}
				case "arity":
					site.Common().Args = append(site.Common().Args, site.Common().Args[0])
				}
				if a.ProveScalarFormat(site) != nil {
					t.Fatal("stale variant accepted")
				}
			})
		}
	}
}

func TestActualCheckerWorkerFormatting(t *testing.T) {
	ps, err := frontend.Load("../checker", ".")
	if err != nil {
		t.Fatal(err)
	}
	p := frontend.BuildSSA(ps)
	frontend.BuildCallGraph(p)
	a := New(p, nil)
	run := p.Roots[0].Func("Run")
	count := 0
	for _, bb := range run.Blocks {
		for _, i := range bb.Instrs {
			site, ok := i.(*ssa.Call)
			if ok && site.Common().StaticCallee() != nil && site.Common().StaticCallee().String() == "fmt.Sprint" {
				if a.ProveScalarFormat(site) == nil {
					t.Fatal("actual checker worker formatting refused")
				}
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("worker formatting sites=%d", count)
	}
}

func TestScalarSprintNativeBoundary(t *testing.T) {
	p := testutil.Load(t, `package main;import "fmt";var calls int;type N int;func(n N)String()string{calls++;return "callback"};func main(){a:=fmt.Sprint("worker=",2);b:=fmt.Sprintln(2,true,nil);c:=fmt.Sprint(N(1));d:=fmt.Sprintln(N(2));fmt.Printf("%q|%q|%q|%q|%d",a,b,c,d,calls)}`)
	main, _ := p.Main()
	a := New(p, nil)
	accepted, refused := 0, 0
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if c, ok := i.(*ssa.Call); ok && c.Common().StaticCallee() != nil {
				switch c.Common().StaticCallee().Name() {
				case "Sprint", "Sprintln":
					if a.ProveScalarFormat(c) != nil {
						accepted++
					} else {
						refused++
					}
				case "Printf":
					if a.ProveScalarFormat(c) != nil {
						t.Fatal("output I/O admitted")
					}
				}
			}
		}
	}
	if accepted != 2 || refused != 2 {
		t.Fatalf("boundary=%d/%d", accepted, refused)
	}
	cmd := exec.CommandContext(t.Context(), "go", "run", ".")
	cmd.Dir = filepath.Dir(p.Fset.Position(main.Pos()).Filename)
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != `"worker=2"|"2 true <nil>\n"|"callback"|"callback\n"|2` {
		t.Fatalf("native spacing/callback mismatch: %s", out)
	}
}
