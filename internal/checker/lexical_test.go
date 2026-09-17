package checker

import (
	"maps"
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
)

var referenceHeader = regexp.MustCompile(`^@!@!@STARTMSG ([0-9]+):([0-9]+) @!@!@$`)
var referencePC = regexp.MustCompile(`(?:"([A-Za-z_][A-Za-z0-9_]*)"\s*:>|([A-Za-z_][A-Za-z0-9_]*)\s*\|->)\s*"([A-Za-z_][A-Za-z0-9_]*)"`)

func compareLexers(t *testing.T, text string) {
	t.Helper()
	code, class, ok := messageHeader(text)
	want := referenceHeader.FindStringSubmatch(text)
	if ok != (want != nil) || ok && (code != want[1] || class != want[2]) {
		t.Fatalf("header differs for %q: %q %q %v vs %q", text, code, class, ok, want)
	}
	pc := map[string]string{}
	for _, match := range referencePC.FindAllStringSubmatch(text, -1) {
		id := match[1]
		if id == "" {
			id = match[2]
		}
		pc[id] = match[3]
	}
	if got := pcEntries(text); !maps.Equal(got, pc) {
		t.Fatalf("PC entries differ for %q: %v vs %v", text, got, pc)
	}
}

func TestLexicalEquivalence(t *testing.T) {
	seeds := []string{"", `@!@!@STARTMSG 2193:0 @!@!@`, `@!@!@STARTMSG 0002193:000 @!@!@`, `@!@!@STARTMSG 999999999999999999999:0 @!@!@`,
		`[P |-> "L", Q |-> "M"]`, `("P" :> "L" @@ "Q" :> "M")`, `"P |-> "L"`, `"P" :> "Q |-> "R"`,
		`9Prefix |-> "L"`, `"P" |-> "L"`, `"P" :> "A" P |-> "B"`, `"" :> "L"`, `P |-> ""`, `P |-> "1Bad"`,
		"P\v|->\"L\"", "P\f|->\n\"L\"", "éP |-> \"L\"", "P\u00a0|-> \"L\"", "P\x00|->\"L\""}
	for _, s := range seeds {
		compareLexers(t, s)
		for i := 0; i <= len(s); i++ {
			for _, b := range []byte{'"', ':', '|', '-', '>', '\n', '\r', '\f', '\v', 0, 255, 'A', '0', '_'} {
				compareLexers(t, s[:i]+string([]byte{b})+s[i:])
			}
		}
	}
	r := rand.New(rand.NewPCG(1, 2))
	fragments := []string{"A", "Z_9", "0", "\"", ":>", "|->", " ", "\t", "\n", "\f", "\v", "\xff", "@@", "[", "]", "@!@!@STARTMSG ", " @!@!@", ":", "2193", "成功"}
	for range 20000 {
		var text strings.Builder
		for range r.IntN(32) {
			text.WriteString(fragments[r.IntN(len(fragments))])
		}
		compareLexers(t, text.String())
	}
	for _, text := range []string{strings.Repeat("X", 1<<20), `"` + strings.Repeat("X", 1<<20) + ` |-> "L"`, strings.Repeat(`"P" :> "Q |-> `, 20000)} {
		compareLexers(t, text)
	}
}

func FuzzLexicalEquivalence(f *testing.F) {
	for _, s := range []string{`P |-> "L"`, `"P" :> "L"`, `@!@!@STARTMSG 2193:0 @!@!@`, `"P |-> "L"`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) { compareLexers(t, text) })
}
