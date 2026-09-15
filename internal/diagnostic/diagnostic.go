// Package diagnostic makes precision and rejection visible to every consumer.
package diagnostic

type Outcome string

const (
	Precise     Outcome = "precisely-modeled"
	Abstracted  Outcome = "conservatively-abstracted"
	Unsupported Outcome = "unsupported"
)

type Diagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitzero"`
}
