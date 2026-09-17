package effects

import (
	"go/constant"
	"go/token"
	"go/types"
	"reflect"
	"testing"

	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestConstantTypeMetadata(t *testing.T) {
	for _, tc := range []struct {
		source string
		native reflect.Type
	}{
		{"int", reflect.TypeFor[int]()}, {"byte", reflect.TypeFor[byte]()}, {"*struct{X int}", reflect.TypeFor[*struct{ X int }]()},
		{"chan int", reflect.TypeFor[chan int]()}, {"func()", reflect.TypeFor[func()]()}, {"any", reflect.TypeFor[any]()},
	} {
		t.Run(tc.source, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "reflect";func main(){_=reflect.TypeFor[`+tc.source+`]()} `)
			main, _ := p.Main()
			a := New(p, nil)
			for _, bb := range main.Blocks {
				for _, i := range bb.Instrs {
					if site, ok := i.(*ssa.Call); ok {
						proof := a.ProveConstantData(site)
						if proof == nil || len(proof.ModeledOperations) != 1 || proof.ModeledOperations[0] != ReflectionTypeModel {
							t.Fatal("metadata model refused")
						}
						e := &dataEval{program: p, sizes: p.Packages[0].TypesSizes, steps: 100000, cells: 65536, owned: map[*dataCell]bool{}, stack: map[*ssa.Function]bool{}}
						v := e.function(p.CallTarget(site), nil)[0]
						if v.reflected == nil || !types.Identical(v.reflected, p.CallTarget(site).TypeArgs()[0]) || e.sizes.Sizeof(v.reflected) != int64(tc.native.Size()) {
							t.Fatal("metadata/native mismatch")
						}
						if v.pointer != nil || v.elements != nil || v.view != nil || v.scalar != nil {
							t.Fatal("type description contains runtime storage")
						}
						return
					}
				}
			}
			t.Fatal("call missing")
		})
	}
}

func TestReflectionMetadataGuards(t *testing.T) {
	for _, broken := range []string{"wrapper", "abi", "typeof", "escape", "cast", "graph"} {
		t.Run(broken, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "reflect";func main(){_=reflect.TypeFor[int]()}`)
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
			if site == nil || !a.ProveConstantDataCall(site) {
				t.Fatal("baseline metadata missing")
			}
			root := p.CallTarget(site)
			changed := false
			if broken == "graph" {
				p.Calls.Nodes[root].Out = nil
				changed = true
			}
			for f := range p.Calls.Nodes {
				if f == nil {
					continue
				}
				name := f.String()
				if broken == "escape" && name == "internal/abi.NoEscape" {
					for _, bb := range f.Blocks {
						for _, i := range bb.Instrs {
							if x, ok := i.(*ssa.BinOp); ok {
								x.Y = ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Uintptr])
								changed = true
							}
						}
					}
				}
				if broken == "typeof" && name == "internal/abi.TypeOf" {
					for _, bb := range f.Blocks {
						for _, i := range bb.Instrs {
							if x, ok := i.(*ssa.FieldAddr); ok {
								x.Field = 1
								changed = true
							}
						}
					}
				}
				if broken == "cast" && name == "reflect.toRType" {
					f.Blocks[0].Instrs = f.Blocks[0].Instrs[:2]
					changed = true
				}
			}
			if broken == "wrapper" {
				root.Blocks[0].Instrs = root.Blocks[0].Instrs[:3]
				changed = true
			}
			if broken == "abi" {
				c := root.Blocks[0].Instrs[0].(*ssa.Call)
				f := p.CallTarget(c)
				for _, i := range f.Blocks[0].Instrs {
					if x, ok := i.(*ssa.UnOp); ok {
						x.Op = token.ARROW
						changed = true
					}
				}
			}
			if !changed {
				t.Fatal("mutation missing")
			}
			if a.ProveConstantDataCall(site) {
				t.Fatal("stale metadata extraction accepted")
			}
		})
	}
}

func TestReflectiveActionsRemainRefused(t *testing.T) {
	for _, expr := range []string{`_=reflect.TypeFor[*int]().Name()`, `_=reflect.TypeFor[struct{X int}]().NumField()`, `_=reflect.ValueOf(1).Interface()`} {
		t.Run(expr, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "reflect";func build(){`+expr+`};func main(){build()}`)
			main, _ := p.Main()
			for _, bb := range main.Blocks {
				for _, i := range bb.Instrs {
					if c, ok := i.(*ssa.Call); ok {
						if New(p, nil).ProveConstantDataCall(c) {
							t.Fatal("reflective action accepted")
						}
						return
					}
				}
			}
		})
	}
}
