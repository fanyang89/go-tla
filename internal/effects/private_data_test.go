package effects_test

import (
	"testing"

	"github.com/fanmi/go-tla/internal/effects"
	"github.com/fanmi/go-tla/internal/testutil"
	"golang.org/x/tools/go/ssa"
)

// This exercises the installed standard-library source, not an errors.New allowlist.
func TestStandardErrorConstructorIsPrivateData(t *testing.T) {
	p := testutil.Load(t, `package main;import "errors";func main(){_ = errors.New("limit")}`)
	main, err := p.Main()
	if err != nil {
		t.Fatal(err)
	}
	a := effects.New(p, nil)
	for _, bb := range main.Blocks {
		for _, i := range bb.Instrs {
			if call, ok := i.(*ssa.Call); ok && call.Common().StaticCallee() != nil && call.Common().StaticCallee().String() == "errors.New" {
				if got := a.Call(call); got.Kind != effects.Pure {
					t.Fatalf("errors.New summary=%+v", got)
				}
				return
			}
		}
	}
	t.Fatal("missing errors.New call")
}

func TestPrivateDataConstructorSummaries(t *testing.T) {
	for _, tc := range []struct {
		name, imports, body string
		want                effects.Kind
	}{
		{"record", "", `func ctor()*record{return &record{text:"message",count:1}}`, effects.Pure},
		{"scalar", "", `func ctor()*int{return new(7)}`, effects.Pure},
		{"array", "", `func ctor()*[2]int{return &[2]int{1,2}}`, effects.Pure},
		{"array-field", "", `func ctor()any{p:=new(struct{a [2]record});p.a[1].count=3;return p}`, effects.Pure},
		{"array-nested", "", `func ctor()*[2][2]int{p:=new([2][2]int);p[1][0]=3;return p}`, effects.Pure},
		{"array-element-return", "", `func ctor()*int{p:=&[2]int{1,2};return &p[1]}`, effects.Pure},
		{"array-address-escape", "", `func ctor()*[2]int{p:=new([2]int);p[1]=2;observeBox(&p[1]);return p}`, effects.Inspect},
		{"array-shared", "", `var table [2]int;func ctor()*[2]int{p:=&[2]int{1,2};table[1]=3;return p}`, effects.Inspect},
		{"array-pointer-elements", "", `func ctor()any{p:=new([2]*int);p[1]=new(1);return p}`, effects.Inspect},
		{"array-channel-elements", "", `func ctor()any{p:=new([2]chan int);p[1]=nil;return p}`, effects.Inspect},
		{"array-mutex-elements", `import "sync"`, `func ctor()any{p:=new(struct{a [2]sync.Mutex;n int});p.n=1;return p}`, effects.Inspect},
		{"array-slice-alias", "", `func ctor()*[2]int{p:=new([2]int);s:=p[:];s[1]=2;return p}`, effects.ConstantData},
		{"nested", "", `func ctor()*nested{p:=new(nested);p.item.text="message";return p}`, effects.Pure},
		{"field-return", "", `func ctor()*int{p:=&record{count:1};return &p.count}`, effects.Pure},
		{"boxed-error", "", `func ctor()error{return &problem{text:"message"}}`, effects.Pure},
		{"branch", "", `func ctor()*record{p:=&record{count:1};if flag{p.text="a"}else{p.text="b"};return p}`, effects.Pure},
		{"publish", "", `func ctor()*record{p:=&record{text:"message"};published=p;p.count=1;return p}`, effects.Inspect},
		{"foreign-write", "", `func ctor()*record{p:=&record{text:"message"};foreign.count=1;return p}`, effects.Inspect},
		{"readonly-escape", "", `func ctor()*record{p:=&record{text:"message"};observe(p);return p}`, effects.ConstantData},
		{"box-escape", "", `func ctor()any{p:=&record{text:"message"};observeBox(p);return p}`, effects.Inspect},
		{"capture", "", `func ctor()func()int{p:=&record{count:1};return func()int{return p.count}}`, effects.Inspect},
		{"phi", "", `func ctor()*record{p:=&record{text:"message"};if flag{p=&foreign};p.count=1;return p}`, effects.Inspect},
		{"pointer-field", "", `func ctor()any{return &struct{p *int;n int}{n:1}}`, effects.Inspect},
		{"interface-field", "", `func ctor()any{return &struct{p any;n int}{n:1}}`, effects.Inspect},
		{"channel-field", "", `func ctor()any{return &struct{ch chan int;n int}{n:1}}`, effects.Inspect},
		{"function-field", "", `func ctor()any{return &struct{f func();n int}{n:1}}`, effects.Inspect},
		{"mutex-field", `import "sync"`, `func ctor()any{return &struct{mu sync.Mutex;n int}{n:1}}`, effects.Inspect},
		{"atomic-field", `import "sync/atomic"`, `func ctor()any{return &struct{n atomic.Int64;x int}{x:1}}`, effects.Inspect},
		{"map-write", "", `func ctor()*record{p:=&record{text:"message"};ledger[1]=2;return p}`, effects.Inspect},
		{"map-only-write", "", `func ctor(){ledger[1]=2}`, effects.Inspect},
		{"shared-copy", "", `func ctor()*record{p:=&record{text:"message"};copy(buffer,p.text);return p}`, effects.Inspect},
		{"shared-append", "", `func ctor()*record{p:=&record{text:"message"};_ = append(buffer,byte(1));return p}`, effects.Inspect},
		{"shared-delete", "", `func ctor()*record{p:=&record{text:"message"};delete(ledger,1);return p}`, effects.Inspect},
		{"shared-clear", "", `func ctor()*record{p:=&record{text:"message"};clear(ledger);return p}`, effects.Inspect},
		{"printing", "", `func ctor()*record{p:=&record{text:"message"};println(p.text);return p}`, effects.Inspect},
		{"readonly-builtins", "", `func ctor()*record{p:=&record{text:"message"};p.count=max(len(p.text),1);return p}`, effects.Pure},
		{"unsafe", `import "unsafe"`, `func ctor()*record{p:=&record{text:"message"};*(*int)(unsafe.Pointer(&p.count))=1;return p}`, effects.Inspect},
		{"defer", "", `func ctor()*record{p:=&record{text:"message"};defer observe(p);return p}`, effects.Inspect},
		{"goroutine", "", `func ctor()*record{p:=&record{text:"message"};go observe(p);return p}`, effects.Inspect},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.Load(t, `package main
`+tc.imports+`
type record struct{text string;count int}
type nested struct{item record}
type problem struct{text string}
func(p *problem)Error()string{return p.text}
var flag bool
var foreign record
var published *record
var ledger=map[int]int{}
var buffer=make([]byte,2,4)
func observe(p *record)int{return p.count}
func observeBox(p any){}
`+tc.body+`
func main(){ctor()}`)
			main, err := p.Main()
			if err != nil {
				t.Fatal(err)
			}
			a := effects.New(p, nil)
			found := false
			for _, bb := range main.Blocks {
				for _, i := range bb.Instrs {
					if call, ok := i.(*ssa.Call); ok && call.Common().StaticCallee() != nil && call.Common().StaticCallee().Name() == "ctor" {
						found = true
						got := a.Call(call)
						if got.Kind != tc.want {
							t.Fatalf("constructor summary=%s, want %s", got.Kind, tc.want)
						}
						if a.Call(call) != got {
							t.Fatal("cached constructor proof changed")
						}
					}
				}
			}
			if !found {
				t.Fatal("missing constructor call")
			}
		})
	}
}
