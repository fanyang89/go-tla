package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/fanmi/go-tla/internal/tla"
)

func TestTLCPrivateReferenceConstructors(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"pointer", `package main;type B struct{p *int};func construct(p *int)*B{return &B{p}};var value=construct(nil);func main(){c:=make(chan int,1);c<-1;<-c}`},
		{"interface", `package main;type I interface{Read()int};type V int;func(v V)Read()int{return int(v)};type B struct{i I};func construct(i I)*B{return &B{i}};var value=construct(V(1));func main(){c:=make(chan int);close(c);<-c}`},
		{"headers", `package main;type B struct{s []int;m map[string]int;f func()};func construct(s []int,m map[string]int,f func())*B{return &B{s,m,f}};var value=construct(nil,nil,nil);func main(){c:=make(chan int,1);c<-1;<-c}`},
	} {
		t.Run(tc.name, func(t *testing.T) { checkTLC(t, fromSource(t, tc.source), "No error has been found") })
	}
}

func TestTLCOpaqueInterfaceConstructors(t *testing.T) {
	for _, value := range []string{"2", "(*int)(nil)", "func(){}"} {
		t.Run(value, func(t *testing.T) {
			m := fromSource(t, `package main;type T struct{V any};func build()*T{return &T{V:`+value+`}};var value=build();func main(){c:=make(chan int,1);c<-1;<-c}`)
			checkTLC(t, m, "No error has been found")
		})
	}
}

func TestPrivateReferenceInitializerRefusals(t *testing.T) {
	for name, source := range map[string]string{
		"borrowed-write":            `package main;type B struct{p *int};var n int;func ctor(p *int)*B{b:=&B{p};*b.p=9;return b};var value=ctor(&n);func main(){}`,
		"callback":                  `package main;type B struct{f func()};func ctor(f func())*B{b:=&B{f};b.f();return b};var value=ctor(func(){});func main(){}`,
		"returned-channel-identity": `package main;type B struct{c chan int};func ctor(c chan int)*B{return &B{c}};func main(){c:=make(chan int);b:=ctor(c);close(b.c)}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, source)
			if !m.HasErrors() {
				t.Fatal("reference transfer granted unproved effects/identity")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unproved reference executable")
			}
		})
	}
}

func TestNativePrivateReferenceConstruction(t *testing.T) {
	source := `package main
import "go/types"
type B struct{p *int;s []int;m map[string]int;f func();i any}
func construct(p *int,s []int,m map[string]int,f func(),i any)*B{return &B{p,s,m,f,i}}
func main(){n:=7;s:=[]int{3};m:=map[string]int{"x":4};calls:=0;f:=func(){calls++};b:=construct(&n,s,m,f,&n)
if n!=7||s[0]!=3||m["x"]!=4||calls!=0||b.p!=&n||b.i!=&n{panic("constructor changed input")}
n=9;s[0]=8;m["x"]=6;if *b.p!=9||b.s[0]!=8||b.m["x"]!=6{panic("reference aliases lost")};b.f();if calls!=1{panic("callback lost")}
t:=types.Typ[types.Int];if types.NewPointer(t).Elem()!=t{panic("type identity lost")}}
`
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "go", "run", path).CombinedOutput()
	if err != nil {
		t.Fatalf("native oracle %v %s", err, out)
	}
}
