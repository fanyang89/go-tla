package tests

import (
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/tla"
)

func TestInitializerCallDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name, declarations, target string
	}{
		{"source-body", `var shared int;func initialize()int{shared++;return shared};var value=initialize()`, "fixture.initialize"},
		{"unavailable-body", `func external()int;var value=external()`, "fixture.external"},
		{"dynamic", `var callback func()int;var value=callback()`, "unresolved dynamic target"},
		{"interface", `type reader interface{Read()int};var input reader;var value=input.Read()`, "unresolved interface method (fixture.reader).Read"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, "package main;"+tc.declarations+";func main(){}")
			found := false
			for _, d := range m.Diagnostics {
				if d.Code != "initializer" || !strings.HasPrefix(d.Message, "effectful or unknown package initialization unsupported:") {
					continue
				}
				if !strings.Contains(d.Message, tc.target) || !strings.Contains(d.Message, "(in fixture.init)") {
					t.Fatalf("missing target/owner: %+v", d)
				}
				if d.File == "" || d.Line != 1 {
					t.Fatalf("call-site location lost: %+v", d)
				}
				found = true
			}
			if !found || !m.HasErrors() {
				t.Fatalf("initializer must remain rejected: %+v", m.Diagnostics)
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("diagnostic enrichment authorized executable output")
			}
		})
	}
}
