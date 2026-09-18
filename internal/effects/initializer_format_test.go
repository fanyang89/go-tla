package effects

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fanmi/go-tla/internal/frontend"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestInitializerFormatComposition(t *testing.T) {
	for _, tc := range []struct {
		name, decl, body string
		want             bool
	}{
		{"sprintf", "", `values[i]=fmt.Sprintf("v%d",i)`, true},
		{"sprint", "", `values[i]=fmt.Sprint(i)`, true},
		{"sprintln", "", `values[i]=fmt.Sprintln(i)`, true},
		{"empty", "", `values[i]=fmt.Sprintf("constant")`, true},
		{"helper", `func text(i int)string{return fmt.Sprintf("%d",i)}`, `values[i]=text(i)`, true},
		{"named", `type N int;func(n N)String()string{return "custom"}`, `values[i]=fmt.Sprint(N(i))`, false},
		{"argument-output", `func arg(i int)int{println(i);return i}`, `values[i]=fmt.Sprint(arg(i))`, false},
		{"io", "", `fmt.Println(i)`, false},
		{"shared-args", `var args=[]any{1}`, `values[i]=fmt.Sprint(args...)`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "fmt";var values [40]string;`+tc.decl+"\n"+`func fill(){for i:=0;i<40;i++{`+tc.body+`}};func main(){}`)
			a := New(p, nil)
			proof := a.ProveInitializerDataEffects(p.Roots[0].Func("fill"))
			if (proof != nil) != tc.want {
				t.Fatalf("proof=%+v want=%v", proof, tc.want)
			}
			if proof != nil && a.ProveInitializerData(p.Roots[0].Func("fill")) {
				t.Fatal("boolean proof discarded format assumptions")
			}
			if proof != nil && !slices.Contains(proof.ModeledOperations, ScalarFormatModel) {
				t.Fatal("format assumption missing")
			}
		})
	}
}

func TestInitializerFormatFreshGraph(t *testing.T) {
	p := testutil.Load(t, `package main;import "fmt";var values [40]string;func fill(){for i:=0;i<40;i++{values[i]=fmt.Sprintf("%d",i)}};func main(){}`)
	a := New(p, nil)
	f := p.Roots[0].Func("fill")
	if a.ProveInitializerDataEffects(f) == nil {
		t.Fatal("fixture missing")
	}
	var target *ssa.Function
	for _, bb := range f.Blocks {
		for _, i := range bb.Instrs {
			if c, ok := i.(*ssa.Call); ok {
				if fn := c.Common().StaticCallee(); fn != nil && fn.Name() == "Sprintf" {
					target = fn
				}
			}
		}
	}
	if target == nil {
		t.Fatal("format call missing")
	}
	p.Calls.Nodes[target].Out = nil
	if a.ProveInitializerDataEffects(f) != nil {
		t.Fatal("stale transitive format graph accepted")
	}
}

func TestNativeInitializerFormatting(t *testing.T) {
	p := testutil.Load(t, `package main;import("fmt";"strconv");var a,b,c [40]string;func init(){for i:=0;i<40;i++{a[i]=fmt.Sprintf("v%d",i);b[i]=fmt.Sprint(i);c[i]=fmt.Sprintln(i)}};func main(){for i:=0;i<40;i++{s:=strconv.Itoa(i);if a[i]!="v"+s||b[i]!=s||c[i]!=s+"\n"{panic("table mismatch")}}}`)
	if New(p, nil).ProveInitializerDataEffects(p.Roots[0].Func("init#1")) == nil {
		t.Fatal("native fixture lacks proof")
	}
	main, _ := p.Main()
	cmd := exec.CommandContext(t.Context(), "go", "run", ".")
	cmd.Dir = filepath.Dir(p.Fset.Position(main.Pos()).Filename)
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native: %v: %s", err, out)
	}
}

func TestActualVersionTableInitialization(t *testing.T) {
	loaded, err := frontend.LoadContext(t.Context(), "../frontend", ".")
	if err != nil {
		t.Fatal(err)
	}
	p := frontend.BuildSSA(loaded)
	frontend.BuildCallGraph(p)
	a := New(p, nil)
	found := false
	for f := range p.Calls.Nodes {
		if f != nil && f.Pkg != nil && f.Pkg.Pkg.Path() == "golang.org/x/tools/internal/stdlib" && f.Name() == "init#1" {
			found = true
			proof := a.ProveInitializerDataEffects(f)
			if proof == nil || !slices.Contains(proof.ModeledOperations, ScalarFormatModel) {
				t.Fatal("actual version table lacks composed proof")
			}
		}
	}
	if !found {
		t.Fatal("actual initializer missing")
	}
}
