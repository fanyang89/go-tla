// Package tlalex recognizes the ASCII identifiers emitted by the TLA backend.
package tlalex

func IdentifierStart(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b == '_'
}

func IdentifierByte(b byte) bool {
	return IdentifierStart(b) || b >= '0' && b <= '9'
}

func Identifier(s string) bool {
	if len(s) == 0 || !IdentifierStart(s[0]) {
		return false
	}
	for i := range len(s) {
		if !IdentifierByte(s[i]) {
			return false
		}
	}
	return true
}
