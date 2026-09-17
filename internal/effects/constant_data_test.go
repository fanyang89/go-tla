package effects

import (
	"encoding/base64"
	"go/constant"
	"go/types"
	"reflect"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestConstantDataInvocation(t *testing.T) {
	for _, tc := range []struct {
		name, decl, arg string
		want            bool
	}{
		{"validation", `func build(n int)int{if n!=2{panic("bad")};return n+1}`, `2`, true},
		{"panic", `func build(n int)int{if n!=2{panic("bad")};return n+1}`, `1`, false},
		{"private-copy", `func build(s string)*[4]byte{p:=new([4]byte);copy(p[:],s);if p[0]!='a'{panic("bad")};return p}`, `"abcd"`, true},
		{"copy-overlap", `func build(s string)*[4]byte{p:=new([4]byte);copy(p[:],s);copy(p[1:],p[:3]);if p[2]!='b'{panic("bad")};return p}`, `"abcd"`, true},
		{"array-copy-value", `func build(n int)*[2]int{p:=&[2]int{n,2};q:=*p;q[0]=4;if p[0]!=1{panic("alias")};return p}`, `1`, true},
		{"reset-array-alias", `func build(n int)int{p:=&[2]int{n,2};a:=&p[0];*p=[2]int{3,4};if *a!=3{panic("bad")};return *a}`, `1`, true},
		{"reset-array-panic", `func build(n int)int{p:=&[2]int{n,2};a:=&p[0];*p=[2]int{3,4};if *a==3{panic("bad")};return *a}`, `1`, false},
		{"reset-struct-alias", `type T struct{N int};func build(n int)int{p:=&T{n};a:=&p.N;*p=T{3};if *a!=3{panic("bad")};return *a}`, `1`, true},
		{"phi-swap", `func build(n int)int{a,b:=1,2;for i:=0;i<n;i++{a,b=b,a};if a!=1||b!=2{panic("bad")};return a}`, `2`, true},
		{"division", `func build(n int)int{v:=n/2;if v!=-1{panic("bad")};return v}`, `-3`, true},
		{"private-mutator", `type T struct{N int};func set(p *T,n int){p.N=n};func build(n int)*T{p:=new(T);set(p,n);if p.N!=n{panic("bad")};return p}`, `2`, true},
		{"allocation-budget", `func build(n int)*[4097]int{p:=new([4097]int);if n!=2{panic("bad")};return p}`, `2`, false},
		{"tuple", `func pair(n int)(int,int){return n,n+1};func build(n int)int{a,b:=pair(n);if b!=a+1{panic("bad")};return a}`, `1`, true},
		{"shared-write", `var shared int;func build(n int)int{shared=n;return n}`, `1`, false},
		{"global-read", `var shared int;func build(n int)int{return shared+n}`, `1`, false},
		{"unknown", `func other()int;func build(n int)int{return other()}`, `1`, false},
		{"recursion", `func build(n int)int{return build(n)}`, `1`, false},
		{"divergence", `func build(n int)int{for{};return n}`, `1`, false},
		{"overflow", `func build(n int8)int8{return n+1}`, `127`, false},
		{"conversion-wrap", `func build(n int)byte{return byte(n)}`, `256`, false},
		{"nil-array-slice", `func build(p *[4]int)[]int{return p[:0]}`, `nil`, false},
		{"nil-array-length", `func build(p *[4]int)int{return len(p)}`, `nil`, true},
		{"bounds", `func build(n int)int{p:=new([2]int);return p[n]}`, `2`, false},
		{"synchronization", `func build(n int)int{c:=make(chan int);close(c);return n}`, `1`, false},
		{"dormant-effects", `func build(n int)int{if n==1{return 2};c:=make(chan int);close(c);return n}`, `1`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.Load(t, "package main;"+tc.decl+";func main(){_=build("+tc.arg+")}")
			main, _ := p.Main()
			a := New(p, []string{"fixture.build", "fixture.other"}) // Trust never supplies interpreter facts.
			for _, bb := range main.Blocks {
				for _, i := range bb.Instrs {
					if call, ok := i.(*ssa.Call); ok && call.Common().StaticCallee() != nil && call.Common().StaticCallee().Name() == "build" {
						if got := a.ProveConstantDataCall(call); got != tc.want {
							t.Fatalf("proof=%v want=%v", got, tc.want)
						}
						return
					}
				}
			}
			t.Fatal("missing invocation")
		})
	}
}

func TestActualBase64EncodingConstantData(t *testing.T) {
	p := testutil.Load(t, `package main;import "encoding/base64";func main(){_=base64.NewEncoding("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/")}`)
	main, _ := p.Main()
	a := New(p, nil)
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if call, ok := i.(*ssa.Call); ok && call.Common().StaticCallee() != nil && call.Common().StaticCallee().String() == "encoding/base64.NewEncoding" {
				if !a.ProveConstantDataCall(call) {
					t.Fatal("actual source constructor not proved")
				}
				// Check concrete private-memory semantics against the native standard
				// constructor, independently of the proof's success boolean.
				e := &dataEval{program: p, sizes: p.Packages[0].TypesSizes, steps: 100000, cells: 65536, owned: map[*dataCell]bool{}, stack: map[*ssa.Function]bool{}}
				arg := call.Common().Args[0].(*ssa.Const)
				result := e.function(p.CallTarget(call), []dataValue{e.literal(arg)})[0].pointer.value
				native := reflect.ValueOf(base64.NewEncoding(constant.StringVal(arg.Value))).Elem()
				structure := result.typ.Underlying().(*types.Struct)
				for i := range structure.NumFields() {
					name := structure.Field(i).Name()
					if name != "encode" && name != "decodeMap" {
						continue
					}
					for j, cell := range result.elements[i].value.elements {
						value, ok := constant.Uint64Val(cell.value.scalar)
						if !ok || value != native.FieldByName(name).Index(j).Uint() {
							t.Fatalf("native mismatch at %s[%d]", name, j)
						}
					}
				}
				for _, bad := range []string{"short", strings.Repeat("A", 64), "\n" + constant.StringVal(arg.Value)[1:]} {
					call.Call.Args[0] = ssa.NewConst(constant.MakeString(bad), arg.Type())
					if a.ProveConstantDataCall(call) {
						t.Fatal("mutated invalid alphabet admitted")
					}
				}
				return
			}
		}
	}
	t.Fatal("missing actual constructor")
}
