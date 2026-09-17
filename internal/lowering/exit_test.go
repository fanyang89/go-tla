package lowering

import (
	"go/constant"
	"go/types"
	"testing"

	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/discovery"
	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestExitConsumesGraphAndSlice(t *testing.T) {
	for _, broken := range []string{"none", "graph", "slice", "argument", "signature"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "os";func main(){os.Exit(3)}`)
			main, _ := p.Main()
			b := &builder{p: p, m: &behavior.Model{Outcome: "precisely-modeled"}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}}
			plan := b.plan(main)
			var site *ssa.Call
			for i, primitive := range plan.Discovery.Primitives {
				if primitive.Kind == behavior.Exit {
					site = i.(*ssa.Call)
				}
			}
			if site == nil {
				t.Fatal("actual os.Exit not recognized")
			}
			if effects.New(p, []string{"os.Exit"}).Call(site).Kind != effects.Primitive {
				t.Fatal("trust overrides termination")
			}
			if broken == "graph" {
				p.Calls.Nodes[main].Out = nil
			}
			if broken == "argument" {
				site.Common().Args[0] = ssa.NewConst(constant.MakeInt64(99), types.Typ[types.Int])
			}
			if broken == "signature" {
				target := site.Common().StaticCallee()
				target.Signature = types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewVar(0, nil, "status", types.Typ[types.String])), nil, false)
			}
			if broken == "slice" {
				delete(plan.Slice.Data, site.Common().Args[0])
			}
			ok := b.consumeExit(&frame{f: main, plan: plan}, site)
			if broken == "none" {
				if !ok || b.m.HasErrors() {
					t.Fatal("valid exit refused")
				}
			} else if ok || !b.m.HasErrors() {
				t.Fatal("unproved exit accepted")
			}
		})
	}
}

func TestExitSourceControl(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		closes       int
	}{
		{"helper-cleanup", `func cleanup(c chan int){close(c)};func stop(){os.Exit(0)};func main(){c:=make(chan int);defer cleanup(c);stop()}`, 0},
		{"worker", `func main(){blocked:=make(chan int);go func(){os.Exit(3)}();<-blocked}`, 0},
		{"argument-effect", `func status(c chan int)int{close(c);return 3};func main(){c:=make(chan int);os.Exit(status(c))}`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "os";`+tc.source)
			main, _ := p.Main()
			// Deliberately isolate function lowering to test the os.Exit control boundary.
			// This is not a whole-program proof: ordinary Lower must still inspect os init.
			m := &behavior.Model{SchemaVersion: behavior.SchemaVersion, Semantics: behavior.CommunicationSemantics, Termination: behavior.MainReturn, Metadata: modelMetadata(Options{}), Name: "model", Outcome: "precisely-modeled"}
			b := &builder{p: p, m: m, nodes: map[string]*node{}, names: map[string]int{}, globals: map[*ssa.Global]string{}, fields: map[string]string{}, channelFields: map[string]string{}, stack: map[*ssa.Function]bool{}, resources: map[string]bool{}, effects: effects.New(p, nil), plans: map[*ssa.Function]*functionPlan{}, loopReported: map[*ssa.Function]bool{}}
			b.process(main, nil, "main")
			m.InitialState = behavior.InitialState{Main: "main", Active: []string{"main"}}
			b.regions()
			if m.HasErrors() {
				t.Fatalf("isolated exit lowering failed: %+v", m.Diagnostics)
			}
			exits, closes := 0, 0
			for _, transition := range m.Transitions {
				for _, e := range transition.Effects {
					if e.Kind == behavior.Exit {
						exits++
						for _, proc := range m.Processes {
							if proc.ID == transition.Process && transition.Destination != proc.Terminal {
								t.Fatal("exit enters cleanup")
							}
						}
					}
					if e.Kind == behavior.CloseChannel {
						closes++
					}
				}
			}
			if exits != 1 || closes != tc.closes {
				t.Fatalf("exit/cleanup/argument effects: exits=%d closes=%d", exits, closes)
			}
			if err := behavior.Validate(m); err != nil {
				t.Fatal(err)
			}
			full, err := Lower(p, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if !full.HasErrors() {
				t.Fatal("os initialization was silently dropped")
			}
		})
	}
}

func TestNamedExitIsNotPrimitive(t *testing.T) {
	p := testutil.Load(t, `package main;func Exit(n int){};func main(){Exit(0)}`)
	main, _ := p.Main()
	for _, b := range main.Blocks {
		for _, i := range b.Instrs {
			if primitive, ok := discovery.Recognize(i); ok && primitive.Kind == behavior.Exit {
				t.Fatal("user Exit treated as process termination")
			}
		}
	}
}
