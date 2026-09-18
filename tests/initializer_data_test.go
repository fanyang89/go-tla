package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestTLCFiniteInitializerWrites(t *testing.T) {
	for _, tc := range []struct{ name, decl, main, want string }{
		{"map", `var m map[int]int;func init(){m=make(map[int]int);for i:=0;i<40;i++{m[i]=i}}`, `c:=make(chan int,1);c<-1;<-c`, "No error has been found"},
		{"named-offset", `type T int;var m map[T]T;func init(){m=make(map[T]T);for i:=T(7);i<T(40);i++{m[i]=i}}`, `c:=make(chan int);close(c);<-c`, "No error has been found"},
		{"shared-array", `var a [40]int;func fill()int{for i:=2;i<40;i++{a[i]=i};return 1};var n=fill()`, `c:=make(chan int,1);c<-1;<-c`, "No error has been found"},
		{"shared-range", `var shared [128]int;func build()*[128]int{p:=new([128]int);for i:=range p{shared[i]=i};return p};var table=build()`, `c:=make(chan int,1);c<-1;<-c`, "No error has been found"},
		{"published-range", `var shared *[128]int;func build()*[128]int{p:=new([128]int);shared=p;for i:=range p{p[i]=i};return p};var table=build()`, `c:=make(chan int,1);c<-1;<-c`, "No error has been found"},
		{"subsequent-deadlock", `var m map[int]int;func init(){m=make(map[int]int);for i:=0;i<40;i++{m[i]=i}}`, `<-make(chan int)`, "Deadlock reached"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, `package main;`+tc.decl+`;func main(){`+tc.main+`}`)
			found := false
			for _, d := range m.Diagnostics {
				if d.Code == "initializer-data" {
					found = true
				}
			}
			if !found {
				t.Fatalf("startup proof missing: %+v", m.Diagnostics)
			}
			checkTLC(t, m, tc.want)
		})
	}
}

func TestStartupDataProofDoesNotApplyAtRuntime(t *testing.T) {
	for name, decl := range map[string]string{
		"map":             `var m map[int]int;func fill(){m=make(map[int]int);for i:=0;i<40;i++{m[i]=i}}`,
		"shared-range":    `var shared [128]int;func fill()*[128]int{p:=new([128]int);for i:=range p{shared[i]=i};return p}`,
		"published-range": `var shared *[128]int;func fill()*[128]int{p:=new([128]int);shared=p;for i:=range p{p[i]=i};return p}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, `package main;`+decl+`;func main(){fill()}`)
			if !m.HasErrors() {
				t.Fatal("runtime writes consumed a startup proof")
			}
		})
	}
}

func TestNativeStartupKeywordTable(t *testing.T) {
	source := `package main;import("go/token";"regexp";"strings");type T int;var m map[T]T;func init(){m=make(map[T]T);for i:=T(7);i<T(40);i++{m[i]=i}};func main(){if len(m)!=33||m[39]!=39{panic("bad table")};words:=[]string{"break","case","chan","const","continue","default","defer","else","fallthrough","for","func","go","goto","if","import","interface","map","package","range","return","select","struct","switch","type","var"};for _,word:=range words{tok:=token.Lookup(word);if !tok.IsKeyword()||tok.String()!=word{panic("keyword table mismatch")}};if token.Lookup("not_a_keyword")!=token.IDENT{panic("identifier mismatch")};for i:=0;i<128;i++{s:=string(byte(i));want:=s;if strings.ContainsRune("\\.+*?()|[]{}^$",rune(i)){want="\\"+s};if regexp.QuoteMeta(s)!=want{panic("regexp table mismatch")}}}`
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
