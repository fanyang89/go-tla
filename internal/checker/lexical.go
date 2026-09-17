package checker

import (
	"strings"

	"github.com/fanmi/go-tla/internal/tlalex"
)

func decimalDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func messageHeader(line string) (code, class string, ok bool) {
	line, ok = strings.CutPrefix(line, "@!@!@STARTMSG ")
	if !ok {
		return "", "", false
	}
	line, ok = strings.CutSuffix(line, " @!@!@")
	if !ok {
		return "", "", false
	}
	code, class, ok = strings.Cut(line, ":")
	return code, class, ok && decimalDigits(code) && decimalDigits(class)
}

func identifierEnd(s string, start int) int {
	if start >= len(s) || !tlalex.IdentifierStart(s[start]) {
		return start
	}
	end := start + 1
	for end < len(s) && tlalex.IdentifierByte(s[end]) {
		end++
	}
	return end
}

func whitespaceEnd(s string, start int) int {
	for start < len(s) {
		// RE2's ASCII \s excludes vertical tab and Unicode spaces.
		switch s[start] {
		case ' ', '\t', '\n', '\r', '\f':
			start++
		default:
			return start
		}
	}
	return start
}

// pcEntryAt preserves the old unanchored grammar and returns the next candidate
// position even on failure. Skipping an entire unquoted identifier is safe: all
// its suffixes have the same following operator/value, avoiding quadratic scans.
func pcEntryAt(s string, start int) (id, location string, next int, ok bool) {
	quoted := s[start] == '"'
	begin := start
	if quoted {
		begin++
	}
	end := identifierEnd(s, begin)
	if end == begin {
		return "", "", start + 1, false
	}
	next = end
	if quoted {
		// A failed quoted candidate may contain a valid unquoted candidate.
		next = start + 1
	}
	pos := end
	op := "|->"
	if quoted {
		if pos >= len(s) || s[pos] != '"' {
			return "", "", next, false
		}
		pos++
		op = ":>"
	}
	pos = whitespaceEnd(s, pos)
	if !strings.HasPrefix(s[pos:], op) {
		return "", "", next, false
	}
	pos = whitespaceEnd(s, pos+len(op))
	if pos >= len(s) || s[pos] != '"' {
		return "", "", next, false
	}
	value := pos + 1
	valueEnd := identifierEnd(s, value)
	if valueEnd == value || valueEnd >= len(s) || s[valueEnd] != '"' {
		return "", "", next, false
	}
	return s[begin:end], s[value:valueEnd], valueEnd + 1, true
}

func pcEntries(s string) map[string]string {
	out := map[string]string{}
	for pos := 0; pos < len(s); {
		id, location, next, ok := pcEntryAt(s, pos)
		if ok {
			out[id] = location
		}
		pos = next
	}
	return out
}
