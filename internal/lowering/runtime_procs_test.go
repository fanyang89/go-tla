package lowering

import (
	"go/constant"
	"go/types"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"github.com/fanmi/go-tla/internal/tla"
	"golang.org/x/tools/go/ssa"
)

const runtimeImports = `package main;import("runtime";"sync");var _ sync.Mutex;`

func TestRuntimeProcsProfile(t *testing.T) {
	for _, n := range []int{0, 1, 2, 1024} {
		p := testutil.Load(t, runtimeImports+`var sem=make(chan int,runtime.GOMAXPROCS(0));func main(){sem<-1;<-sem}`)
		m, err := Lower(p, Options{RuntimeProcs: n})
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			if !m.HasErrors() {
				t.Fatal("unbound runtime capacity accepted")
			}
			continue
		}
		if m.HasErrors() || len(m.Channels) != 1 || m.Channels[0].Capacity != n {
			t.Fatalf("profile %d: %+v %+v", n, m.Channels, m.Diagnostics)
		}
		found := false
		for _, s := range m.Assumptions {
			if strings.HasPrefix(s, "Conditional runtime environment:") {
				found = true
			}
		}
		if !found {
			t.Fatal("environment assumption missing")
		}
	}
}

func TestRuntimeProcsRefusals(t *testing.T) {
	for name, source := range map[string]string{
		"name-collision":      `func GOMAXPROCS(n int)int{return 2};func main(){_=runtime.GOMAXPROCS(0);c:=make(chan int,GOMAXPROCS(0));_=c}`,
		"other-init":          `func init(){println("effect")};func main(){_=runtime.GOMAXPROCS(0)}`,
		"setter":              `func hidden(){runtime.GOMAXPROCS(3)};func main(){_=runtime.GOMAXPROCS(0)}`,
		"reset":               `func hidden(){runtime.SetDefaultGOMAXPROCS()};func main(){_=runtime.GOMAXPROCS(0)}`,
		"escape":              `var setting=runtime.GOMAXPROCS;func main(){_=setting(0)}`,
		"defer":               `func main(){defer runtime.GOMAXPROCS(0)}`,
		"argument":            `func arg()int{return 0};func main(){_=runtime.GOMAXPROCS(arg())}`,
		"negative-query":      `func main(){_=runtime.GOMAXPROCS(-1)}`,
		"arithmetic-capacity": `func main(){c:=make(chan int,runtime.GOMAXPROCS(0)+1);_=c}`,
	} {
		t.Run(name, func(t *testing.T) {
			p := testutil.Load(t, runtimeImports+source)
			m, err := Lower(p, Options{RuntimeProcs: 2})
			if err != nil {
				t.Fatal(err)
			}
			if !m.HasErrors() {
				t.Fatal("unproved environment accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved environment executable")
			}
		})
	}
	for _, opts := range []Options{{RuntimeProcs: -1}, {RuntimeProcs: 1025}, {RuntimeProcs: 2, TrustedCalls: []string{"runtime.GOMAXPROCS"}}} {
		if opts.Validate() == nil {
			t.Fatal("invalid profile options accepted")
		}
	}
}

func TestRuntimeProcsGlobalProofFreshness(t *testing.T) {
	for _, broken := range []string{"setting", "query"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, runtimeImports+`var sem=make(chan int,runtime.GOMAXPROCS(0));func main(){sem<-1;<-sem}`)
			opts := Options{RuntimeProcs: 2}
			b := &builder{p: p, opts: opts, m: &behavior.Model{Metadata: modelMetadata(opts)}, names: map[string]int{}}
			var creation *ssa.MakeChan
			for _, bb := range p.Roots[0].Func("init").Blocks {
				for _, i := range bb.Instrs {
					if allocation, ok := i.(*ssa.MakeChan); ok {
						creation = allocation
					}
				}
			}
			if creation == nil || !b.consumeOpenGlobal(creation) {
				t.Fatal("runtime creation proof missing")
			}
			proof := b.proveGlobalCreation(creation)
			if broken == "setting" {
				b.opts.RuntimeProcs = 3
				b.m.Metadata = modelMetadata(b.opts)
			} else {
				creation.Size.(*ssa.Call).Common().Args[0] = ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Int])
			}
			if id := b.identity(&frame{}, proof.global, map[ssa.Value]bool{}); id != "invalid" || !b.m.HasErrors() {
				t.Fatal("stale runtime creation proof accepted")
			}
		})
	}
}

func TestRuntimeProcsProofFreshness(t *testing.T) {
	for _, broken := range []string{"argument", "graph", "slice", "callee-slice", "metadata", "signature", "source", "setter"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, runtimeImports+`func hidden(){_=runtime.GOMAXPROCS(0)};func main(){c:=make(chan int,runtime.GOMAXPROCS(0));c<-1;<-c}`)
			opts := Options{RuntimeProcs: 2}
			main, _ := p.Main()
			b := &builder{p: p, opts: opts, m: &behavior.Model{Metadata: modelMetadata(opts)}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
			plan := b.plan(main)
			var site *ssa.Call
			for c := range plan.Calls {
				if c.Common().StaticCallee() == b.runtimeGetter() {
					site = c.(*ssa.Call)
				}
			}
			if site == nil || !b.consumeRuntimeQuery(&frame{f: main, plan: plan}, site) {
				t.Fatal("baseline runtime query refused")
			}
			switch broken {
			case "argument":
				site.Common().Args[0] = ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Int])
			case "graph":
				p.Calls.Nodes[main].Out = nil
			case "slice":
				delete(plan.Slice.Data, site.Common().Args[0])
			case "callee-slice":
				delete(plan.Slice.Data, site.Common().Value)
			case "metadata":
				b.m.Metadata.Options["startup.GOMAXPROCS"] = []string{"3"}
			case "signature":
				f := b.runtimeGetter()
				f.Signature = types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewVar(0, nil, "n", types.Typ[types.String])), f.Signature.Results(), false)
			case "source":
				f := b.runtimeGetter()
				file := p.Fset.Position(f.Pos()).Filename
				delete(p.Sources, file)
			case "setter":
				f := p.Roots[0].Func("hidden")
				delete(p.Calls.Nodes, f) // Still inventoried through the complete SSA program.
				for _, bb := range f.Blocks {
					for _, i := range bb.Instrs {
						if c, ok := i.(*ssa.Call); ok {
							c.Common().Args[0] = ssa.NewConst(constant.MakeInt64(3), types.Typ[types.Int])
						}
					}
				}
			}
			if b.consumeRuntimeQuery(&frame{f: main, plan: plan}, site) {
				t.Fatal("stale runtime query accepted")
			}
		})
	}
}
