package effects

import (
	"go/constant"
	"go/types"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/frontend"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ast/edge"
	"golang.org/x/tools/go/ssa"
)

func TestActualASTEdgeMetadata(t *testing.T) {
	loaded, err := frontend.Load("../..", "golang.org/x/tools/go/ast/edge")
	if err != nil {
		t.Fatal(err)
	}
	p := frontend.BuildSSA(loaded)
	frontend.BuildCallGraph(p)
	a := New(p, nil)
	max, _ := constant.Int64Val(p.Roots[0].Pkg.Scope().Lookup("maxKind").(*types.Const).Val())
	native := map[string]edge.Kind{}
	for n := 1; n < int(max); n++ {
		k := edge.Kind(n)
		native[k.NodeType().String()+"/"+k.FieldName()] = k
	}
	count := 0
	for f := range p.Calls.Nodes {
		if f == nil || f.String() != "golang.org/x/tools/go/ast/edge.init" {
			continue
		}
		for _, bb := range f.Blocks {
			for _, i := range bb.Instrs {
				site, ok := i.(*ssa.Call)
				if !ok || site.Common().StaticCallee() == nil || !strings.HasPrefix(site.Common().StaticCallee().String(), "golang.org/x/tools/go/ast/edge.info[") {
					continue
				}
				proof := a.ProveConstantData(site)
				if proof == nil || len(proof.ModeledOperations) != 2 {
					t.Fatalf("edge initializer refused: %s", site)
				}
				e := &dataEval{program: p, sizes: p.Packages[0].TypesSizes, steps: 100000, cells: 65536, owned: map[*dataCell]bool{}, stack: map[*ssa.Function]bool{}}
				v := e.function(p.CallTarget(site), []dataValue{e.literal(site.Common().Args[0].(*ssa.Const))})[0]
				if len(v.elements) != 4 {
					t.Fatal("unexpected fieldInfo shape")
				}
				spell := func(t types.Type) string {
					return types.TypeString(t, func(p *types.Package) string { return p.Name() })
				}
				key := spell(v.elements[0].value.reflected) + "/" + constant.StringVal(v.elements[1].value.scalar)
				k, ok := native[key]
				if !ok {
					t.Fatalf("native metadata missing: %s", key)
				}
				field, ok := k.NodeType().Elem().FieldByName(k.FieldName())
				index, exact := constant.Int64Val(v.elements[2].value.scalar)
				if !ok || !exact || index != int64(field.Index[0]) || spell(v.elements[3].value.reflected) != k.FieldType().String() {
					t.Fatalf("metadata differs from native Go: %s", key)
				}
				delete(native, key)
				count++
			}
		}
	}
	if count != int(max)-1 || len(native) != 0 {
		t.Fatalf("metadata inventory: got=%d want=%d unmatched=%d", count, max-1, len(native))
	}
}

func TestDirectReflectionFieldMetadata(t *testing.T) {
	native := reflect.TypeFor[struct {
		A byte
		B int64 `doc:"value"`
	}]()
	for _, name := range []string{"A", "B", "Missing"} {
		t.Run(name, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "reflect";func build(s string)(reflect.StructField,bool){return reflect.TypeFor[*struct{A byte;B int64 `+"`doc:\"value\"`"+`}]().Elem().FieldByName(s)};func main(){_,_=build(`+strconv.Quote(name)+`)}`)
			main, _ := p.Main()
			a := New(p, nil)
			for _, bb := range main.Blocks {
				for _, i := range bb.Instrs {
					if site, ok := i.(*ssa.Call); ok {
						if !a.ProveConstantDataCall(site) {
							t.Fatal("direct field query refused")
						}
						e := &dataEval{program: p, sizes: p.Packages[0].TypesSizes, steps: 100000, cells: 65536, owned: map[*dataCell]bool{}, stack: map[*ssa.Function]bool{}}
						values := e.function(p.CallTarget(site), []dataValue{e.literal(site.Common().Args[0].(*ssa.Const))})
						field, found := native.FieldByName(name)
						if constant.BoolVal(values[1].scalar) != found {
							t.Fatal("presence mismatch")
						}
						v := values[0]
						shape := v.typ.Underlying().(*types.Struct)
						for n := range shape.NumFields() {
							data := v.elements[n].value
							switch shape.Field(n).Name() {
							case "Name":
								if constant.StringVal(data.scalar) != field.Name {
									t.Fatal("name mismatch")
								}
							case "PkgPath":
								if constant.StringVal(data.scalar) != field.PkgPath {
									t.Fatal("package mismatch")
								}
							case "Tag":
								if constant.StringVal(data.scalar) != string(field.Tag) {
									t.Fatal("tag mismatch")
								}
							case "Offset":
								off, ok := constant.Uint64Val(data.scalar)
								if !ok || off != uint64(field.Offset) {
									t.Fatal("offset mismatch")
								}
							case "Anonymous":
								if constant.BoolVal(data.scalar) != field.Anonymous {
									t.Fatal("anonymous mismatch")
								}
							case "Type":
								if found {
									if e.sizes.Sizeof(data.reflected) != int64(field.Type.Size()) {
										t.Fatal("field type mismatch")
									}
								} else if data.reflected != nil {
									t.Fatal("absent field has type")
								}
							case "Index":
								if found {
									if data.view == nil || data.view.length != 1 || e.integer(data.view.elements[0].value) != field.Index[0] {
										t.Fatal("index mismatch")
									}
								} else if data.view != nil {
									t.Fatal("absent field has index")
								}
							}
						}
						return
					}
				}
			}
			t.Fatal("call missing")
		})
	}
}

func TestReflectionMainPackagePath(t *testing.T) {
	p := testutil.Load(t, `package main;import("reflect";"fmt");func metadata()reflect.StructField{f,_:=reflect.TypeFor[struct{x int}]().FieldByName("x");return f};func main(){fmt.Println(metadata().PkgPath)}`)
	main, _ := p.Main()
	file := p.Fset.Position(main.Pos()).Filename
	native, err := exec.CommandContext(t.Context(), "go", "run", file).CombinedOutput()
	if err != nil {
		t.Fatalf("native: %v: %s", err, native)
	}
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			site, ok := i.(*ssa.Call)
			if !ok || site.Common().StaticCallee() == nil || site.Common().StaticCallee().Name() != "metadata" {
				continue
			}
			if !New(p, nil).ProveConstantDataCall(site) {
				t.Fatal("metadata refused")
			}
			e := &dataEval{program: p, sizes: p.Packages[0].TypesSizes, steps: 100000, cells: 65536, owned: map[*dataCell]bool{}, stack: map[*ssa.Function]bool{}}
			v := e.function(p.CallTarget(site), nil)[0]
			if constant.StringVal(v.elements[1].value.scalar) != strings.TrimSpace(string(native)) {
				t.Fatal("main package path differs from native executable")
			}
			return
		}
	}
	t.Fatal("metadata call missing")
}

func TestReflectionElementKinds(t *testing.T) {
	for _, tc := range []struct {
		source string
		native reflect.Type
	}{
		{"*int", reflect.TypeFor[*int]()}, {"[2]byte", reflect.TypeFor[[2]byte]()}, {"[]int", reflect.TypeFor[[]int]()}, {"map[string]int64", reflect.TypeFor[map[string]int64]()}, {"<-chan int", reflect.TypeFor[<-chan int]()},
	} {
		t.Run(tc.source, func(t *testing.T) {
			p := testutil.Load(t, `package main;import "reflect";func element()reflect.Type{return reflect.TypeFor[`+tc.source+`]().Elem()};func main(){_=element()}`)
			f := p.Roots[0].Func("element")
			e := &dataEval{program: p, sizes: p.Packages[0].TypesSizes, steps: 100000, cells: 65536, owned: map[*dataCell]bool{}, stack: map[*ssa.Function]bool{}}
			value := e.function(f, nil)[0]
			if value.reflected == nil || e.sizes.Sizeof(value.reflected) != int64(tc.native.Elem().Size()) {
				t.Fatal("element differs from native Go")
			}
		})
	}
}

func TestReflectionQueryRefusals(t *testing.T) {
	for _, body := range []string{
		`func build(){_=reflect.TypeFor[int]().Elem()}`,
		`func build(){var r reflect.Type;_=r.Elem()}`,
		`type Inner struct{X int};type Outer struct{Inner};func build(){_,_=reflect.TypeFor[Outer]().FieldByName("X")}`,
		`func build(){_,_=reflect.TypeFor[int]().FieldByName("X")}`,
		`func build(){_,ok:=reflect.TypeFor[struct{X int}]().FieldByName("Missing");if !ok{panic("missing")}}`,
	} {
		p := testutil.Load(t, "package main;import \"reflect\";"+body+";func main(){build()}")
		main, _ := p.Main()
		for _, bb := range main.Blocks {
			for _, i := range bb.Instrs {
				if site, ok := i.(*ssa.Call); ok {
					if New(p, nil).ProveConstantDataCall(site) {
						t.Fatal("unproved reflection accepted")
					}
				}
			}
		}
	}
}
