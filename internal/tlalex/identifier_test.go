package tlalex

import (
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
)

var referenceIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func TestIdentifierEquivalence(t *testing.T) {
	check := func(s string) {
		t.Helper()
		if Identifier(s) != referenceIdentifier.MatchString(s) {
			t.Fatalf("identifier differs: %q", s)
		}
	}
	for _, s := range []string{"", "_", "_0", "main", "L0", "0L", "A\n", "é", "A_成功", strings.Repeat("a", 1<<20)} {
		check(s)
	}
	for n := range 256 {
		b := string([]byte{byte(n)})
		check(b)
		check("A" + b)
		check(b + "X")
	}
	r := rand.New(rand.NewPCG(3, 4))
	for range 10000 {
		b := make([]byte, r.IntN(128))
		for i := range b {
			b[i] = byte(r.IntN(256))
		}
		check(string(b))
	}
}

func FuzzIdentifierEquivalence(f *testing.F) {
	for _, s := range []string{"", "P0", "_", "0P", "é", "P\n"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if Identifier(s) != referenceIdentifier.MatchString(s) {
			t.Fatalf("identifier differs: %q", s)
		}
	})
}
