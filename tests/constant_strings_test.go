package tests

import "testing"

func TestStringSearchPreservesInitializationRefusal(t *testing.T) {
	m := fromSource(t, `package main;import "strings";func main(){_=strings.IndexByte("aba",97)}`)
	if !m.HasErrors() {
		t.Fatal("unproved dependency initialization omitted")
	}
	proved := false
	for _, d := range m.Diagnostics {
		if d.Code == "modeled-data-operation" {
			proved = true
		}
	}
	if !proved {
		t.Fatal("expected explicit operation model")
	}
}

func TestTLCConstantStringConcatenation(t *testing.T) {
	decl := `func join(s string)string{v:=s+"suffix";if v!="prefixsuffix"{panic("bad")};return v}`
	for _, tc := range []struct{ name, body, want string }{
		{"communication", `_=join("prefix");c:=make(chan int);go func(){c<-1}();<-c`, "No error has been found"},
		{"initializer", `c:=make(chan int);close(c);<-c`, "No error has been found"},
		{"abstract-result", `c:=make(chan int);if join("prefix")=="prefixsuffix"{close(c)}else{close(c);close(c)}`, "Invariant NoSynchronizationErrors is violated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			extra := ""
			if tc.name == "initializer" {
				extra = `;var value=join("prefix")`
			}
			m := fromSource(t, "package main;"+decl+extra+";func main(){"+tc.body+"}")
			if m.HasErrors() {
				t.Fatalf("string computation refused: %+v", m.Diagnostics)
			}
			proved := false
			for _, d := range m.Diagnostics {
				if d.Code == "constant-data-call" {
					proved = true
				}
			}
			if !proved {
				t.Fatal("missing consumed computation proof")
			}
			checkTLC(t, m, tc.want)
		})
	}
}
