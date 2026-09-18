package effects

import (
	"go/types"
	"testing"

	"github.com/fanmi/go-tla/internal/frontend"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestDormantSynchronizationConstructors(t *testing.T) {
	pre := `package main;import "sync";type T struct{name string;mu sync.Mutex;once sync.Once;wg sync.WaitGroup;next *T};`
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"ordinary-field", `func ctor(name string)*T{return &T{name:name}}`, true},
		{"nested", `type N struct{t T};func ctor(name string)*N{p:=new(N);p.t.name=name;return p}`, true},
		{"boxed-pointer", `func ctor(name string)any{return &T{name:name}}`, true},
		{"state-copy", `func ctor(mu sync.Mutex)*T{return &T{mu:mu}}`, false},
		{"owner-copy", `func ctor(name string)T{p:=&T{name:name};return *p}`, false},
		{"reset", `func ctor(name string)*T{p:=&T{name:name};*p=T{};return p}`, false},
		{"sync-address", `func ctor(name string)*sync.Mutex{p:=&T{name:name};return &p.mu}`, false},
		{"lock", `func ctor(name string)*T{p:=&T{name:name};p.mu.Lock();return p}`, false},
		{"once", `func ctor(name string)*T{p:=&T{name:name};p.once.Do(func(){});return p}`, false},
		{"publish", `var out *T;func ctor(name string)*T{p:=&T{name:name};out=p;return p}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.Load(t, pre+tc.body+`;func main(){}`)
			a := New(p, nil)
			if got := a.ProvePure(p.Roots[0].Func("ctor")); got != tc.want {
				t.Fatalf("pure=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestActualDormantConstructors(t *testing.T) {
	loaded, err := frontend.LoadContext(t.Context(), "../frontend", ".")
	if err != nil {
		t.Fatal(err)
	}
	p := frontend.BuildSSA(loaded)
	frontend.BuildCallGraph(p)
	a := New(p, nil)
	found := map[string]bool{"internal/godebug.New": false, "go/constant.MakeString": false}
	for f := range p.Calls.Nodes {
		if f == nil || f.Pkg == nil {
			continue
		}
		key := f.Pkg.Pkg.Path() + "." + f.Name()
		if _, ok := found[key]; ok {
			found[key] = true
			if !a.ProvePure(f) {
				t.Errorf("actual constructor not proved: %s", key)
			}
		}
	}
	for key, ok := range found {
		if !ok {
			t.Errorf("actual constructor missing: %s", key)
		}
	}
}

func TestDormantTypeBudgets(t *testing.T) {
	p := testutil.Load(t, `package main;import "sync";type T struct{name string;once sync.Once};func main(){}`)
	typ := p.Roots[0].Members["T"].(*ssa.Type).Type()
	budget := 256
	if !dormantContainer(typ, 0, &budget) {
		t.Fatal("valid dormant type refused")
	}
	budget = 0
	if dormantContainer(typ, 0, &budget) {
		t.Fatal("type budget ignored")
	}
	budget = 256
	if dormantContainer(typ, 65, &budget) {
		t.Fatal("type depth ignored")
	}
	for range 65 {
		typ = types.NewArray(typ, 1)
	}
	budget = 256
	if dormantContainer(typ, 0, &budget) {
		t.Fatal("nested depth ignored")
	}
}

func TestDormantStoreInventory(t *testing.T) {
	for _, mutation := range []string{"none", "referrers", "publication", "sync-address", "budget"} {
		t.Run(mutation, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "sync";type T struct{name string;mu sync.Mutex};var out *T;func ctor(name string)*T{p:=&T{name:name};out=nil;return p};func main(){}`)
			f := p.Roots[0].Func("ctor")
			var root *ssa.Alloc
			var field *ssa.FieldAddr
			var global *ssa.Store
			for _, bb := range f.Blocks {
				for _, i := range bb.Instrs {
					switch x := i.(type) {
					case *ssa.Alloc:
						root = x
					case *ssa.FieldAddr:
						field = x
					case *ssa.Store:
						if _, ok := x.Addr.(*ssa.Global); ok {
							global = x
						}
					}
				}
			}
			if root == nil || field == nil || global == nil || !privateDormantAllocation(root) {
				t.Fatal("dormant fixture missing")
			}
			switch mutation {
			case "referrers":
				*root.Referrers() = nil
			case "publication":
				global.Val = root
			case "sync-address":
				field.Field = 1
			case "budget":
				for range 4097 {
					f.Blocks[0].Instrs = append(f.Blocks[0].Instrs, root)
				}
			}
			if got := privateDormantAllocation(root); got != (mutation == "none" || mutation == "referrers") {
				t.Fatal("invalid dormant ownership proof")
			}
		})
	}
}
