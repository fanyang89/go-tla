package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestTLCDormantSynchronizationConstructors(t *testing.T) {
	for _, kind := range []string{"sync.Mutex", "sync.WaitGroup", "sync.Once", "[2]sync.Mutex"} {
		t.Run(kind, func(t *testing.T) {
			m := fromSource(t, `package main;import "sync";type T struct{name string;state `+kind+`};func ctor(name string)*T{return &T{name:name}};var value=ctor("test");func main(){c:=make(chan int,1);c<-1;<-c}`)
			checkTLC(t, m, "No error has been found")
		})
	}
}

func TestDormantSynchronizationRefusals(t *testing.T) {
	for name, body := range map[string]string{
		"operation":         `func ctor()*T{p:=&T{name:"x"};p.mu.Lock();return p};var value=ctor();func main(){}`,
		"copy":              `func ctor()T{p:=&T{name:"x"};return *p};var value=ctor();func main(){}`,
		"returned-identity": `func ctor()*T{return &T{name:"x"}};func main(){ctor().mu.Lock()}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, `package main;import "sync";type T struct{name string;mu sync.Mutex};`+body)
			if !m.HasErrors() {
				t.Fatal("dormant storage granted an operation/copy/identity exemption")
			}
		})
	}
}

func TestNativeDormantSynchronizationConstruction(t *testing.T) {
	source := `package main;import("sync";"go/constant");type T struct{name string;mu sync.Mutex;once sync.Once;wg sync.WaitGroup;next *T};func ctor(name string)*T{return &T{name:name}};func main(){a,b:=ctor("a"),ctor("b");if a==b||a.name!="a"||b.name!="b"||a.next!=nil{panic("bad construction")};if !a.mu.TryLock()||!b.mu.TryLock(){panic("nonzero mutex")};a.mu.Unlock();b.mu.Unlock();n:=0;a.once.Do(func(){n++});a.once.Do(func(){n++});b.once.Do(func(){n++});if n!=2{panic("shared or completed Once")};a.wg.Add(1);a.wg.Done();a.wg.Wait();b.wg.Wait();if constant.StringVal(constant.MakeString("x"))!="x"||constant.MakeString("")!=constant.MakeString(""){panic("constant string identity/value changed")}}`
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "go", "run", path).CombinedOutput()
	if err != nil {
		t.Fatalf("native %v: %s", err, out)
	}
}
