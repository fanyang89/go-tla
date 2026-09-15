package checker

import (
	"fmt"
	"strings"
	"testing"
)

func frame(code, class int, text string) string {
	return fmt.Sprintf("@!@!@STARTMSG %d:%d @!@!@\n%s\n@!@!@ENDMSG %d @!@!@\n", code, class, text, code)
}

func transcript(outcome string) string {
	return frame(2262, 0, "TLC version fixture") + frame(2185, 0, "Starting") + outcome + frame(2186, 0, "Finished")
}

func TestProtocolClassification(t *testing.T) {
	cases := []struct {
		name, log string
		exit      int
		want      Status
	}{
		{"pass", transcript(frame(2193, 0, "success")), 0, Passed},
		{"deadlock", transcript(frame(2114, 1, "Deadlock reached.")), 11, Deadlock},
		{"invariant", transcript(frame(2110, 1, "Invariant NoSynchronizationErrors is violated.")), 12, SynchronizationError},
		{"initial-invariant", transcript(frame(2107, 1, "Invariant NoSynchronizationErrors is violated by the initial state")), 12, SynchronizationError},
		{"different-invariant", transcript(frame(2110, 1, "Invariant SomethingElse is violated.")), 12, ToolError},
		{"prose-is-not-proof", "No error has been found\n", 0, ToolError},
		{"misordered-success", frame(2193, 0, "success") + transcript(""), 0, ToolError},
		{"duplicate-run", transcript(frame(2193, 0, "success")) + transcript(frame(2193, 0, "success")), 0, ToolError},
		{"missing-finish", frame(2262, 0, "version") + frame(2185, 0, "start") + frame(2193, 0, "success"), 0, ToolError},
		{"wrong-message-class", transcript(frame(2193, 1, "not success")), 0, ToolError},
		{"wrong-exit", transcript(frame(2193, 0, "success")), 153, ToolError},
		{"zero-exit-deadlock", transcript(frame(2114, 1, "deadlock")), 0, ToolError},
		{"contradictory", transcript(frame(2193, 0, "success") + frame(2114, 1, "deadlock")), 11, ToolError},
		{"other-error", transcript(frame(2193, 0, "success") + frame(9999, 1, "unknown error")), 0, ToolError},
		{"memory", frame(1001, 1, "out of memory"), 153, Incomplete},
		{"state-space-limit", "", 152, Incomplete},
		{"parse-error", "SANY parse failed", 150, ToolError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := parseProtocol(strings.NewReader(c.log))
			if err != nil {
				t.Fatal(err)
			}
			if got, reason := p.classify(c.exit); got != c.want {
				t.Fatalf("got %s (%s), want %s", got, reason, c.want)
			}
		})
	}
}

func TestMalformedProtocolRejected(t *testing.T) {
	for _, log := range []string{
		"@!@!@STARTMSG 2193:0 @!@!@\nsuccess",
		"@!@!@ENDMSG 2193 @!@!@\n",
		"@!@!@STARTMSG broken\n",
		"@!@!@STARTMSG 2193:0 @!@!@\n@!@!@ENDMSG 2186 @!@!@\n",
		"@!@!@STARTMSG 2193:0 @!@!@\n" + frame(2186, 0, "nested"),
		strings.Repeat("x", (1<<20)+1),
	} {
		if _, err := parseProtocol(strings.NewReader(log)); err == nil {
			t.Fatal("malformed or oversized protocol accepted")
		}
	}
}

func TestTraceSourceCandidates(t *testing.T) {
	log := frame(2217, 4, `/\ pc = [main |-> "entry", worker |-> "send"]`)
	log += frame(2217, 4, "/\\ pc = (\"main\" :> \"done\" @@\n\"worker\" :> \"send\")\n/\\ local = [unrelated |-> \"value\"]")
	p, err := parseProtocol(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	if p.previousPC["main"] != "entry" || p.lastPC["main"] != "done" || p.lastPC["worker"] != "send" || len(p.lastPC) != 2 {
		t.Fatalf("incorrect trace candidates: %+v", p)
	}
}

func TestStatusExitCodes(t *testing.T) {
	for status, code := range map[Status]int{Passed: 0, ToolError: 1, AnalysisError: 1, Deadlock: 3, SynchronizationError: 4, Unsupported: 5, Incomplete: 6, Running: 6} {
		if status.ExitCode() != code {
			t.Fatalf("%s exit changed", status)
		}
	}
}
