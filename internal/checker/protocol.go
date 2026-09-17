package checker

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// TLC -tool message codes from tlc2.output.EC (verified with official TLC 1.8.0).
// Require framed outcomes AND matching exit codes; prose alone never proves success.
type protocol struct {
	started, finished, success, deadlock, invariant, resourceLimit, otherError bool
	version, stats                                                             string
	lastPC, previousPC                                                         map[string]string
}

func parseProtocol(r io.Reader) (protocol, error) {
	var p protocol
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 4096), 1<<20)
	code, class := -1, 0
	var body strings.Builder
	for s.Scan() {
		line := s.Text()
		if strings.Contains(line, "java.lang.OutOfMemoryError") {
			p.resourceLimit = true
		}
		if codeText, classText, ok := messageHeader(line); ok {
			if code != -1 {
				return p, fmt.Errorf("nested TLC message")
			}
			code, _ = strconv.Atoi(codeText)
			class, _ = strconv.Atoi(classText)
			body.Reset()
			continue
		}
		if strings.HasPrefix(line, "@!@!@STARTMSG") {
			return p, fmt.Errorf("malformed TLC message header")
		}
		if strings.HasPrefix(line, "@!@!@ENDMSG") {
			if code == -1 || line != fmt.Sprintf("@!@!@ENDMSG %d @!@!@", code) {
				return p, fmt.Errorf("mismatched TLC message end")
			}
			p.accept(code, class, strings.TrimSpace(body.String()))
			code = -1
			continue
		}
		if code != -1 {
			if body.Len()+len(line) > 1<<20 {
				return p, fmt.Errorf("TLC message exceeds parser limit")
			}
			body.WriteString(line)
			body.WriteByte('\n')
		}
	}
	if err := s.Err(); err != nil {
		return p, err
	}
	if code != -1 {
		return p, fmt.Errorf("truncated TLC message")
	}
	return p, nil
}

func (p *protocol) accept(code, class int, text string) {
	expected := -1
	switch code {
	case 2262, 2185, 2186, 2193, 2199:
		expected = 0
	case 2114, 2107, 2110, 1001, 1002, 1003, 2121:
		expected = 1
	case 2216, 2217:
		expected = 4
	}
	if expected != -1 && class != expected {
		p.otherError = true
		return
	}
	switch code {
	case 2262:
		if p.version != "" {
			p.otherError = true
		}
		p.version = text
	case 2185:
		if p.started || p.finished {
			p.otherError = true
		}
		p.started = true
	case 2186:
		if !p.started || p.finished {
			p.otherError = true
		}
		p.finished = true
	case 2193:
		if !p.started || p.finished || p.success {
			p.otherError = true
		}
		p.success = true
	case 2114:
		p.deadlock = true
	case 2107, 2110:
		if strings.HasPrefix(text, "Invariant NoSynchronizationErrors is violated") {
			p.invariant = true
		} else {
			p.otherError = true
		}
	case 1001, 1002, 1003:
		p.resourceLimit = true
	case 2199:
		p.stats = text
	case 2216, 2217:
		p.previousPC, p.lastPC = p.lastPC, tracePC(text)
	case 2121: // Counterexample preamble, classified by its preceding violation.
	default:
		if class == 1 {
			p.otherError = true
		}
	}
}

// Only recognize the generated model's simple string-valued pc function. These
// locations are source candidates, not a general TLA value/trace decoder.
func tracePC(text string) map[string]string {
	_, pc, ok := strings.Cut(text, `/\ pc = `)
	if !ok {
		return nil
	}
	pc, _, _ = strings.Cut(pc, "\n/\\ ")
	return pcEntries(pc)
}

func (p protocol) classify(exit int) (Status, string) {
	if p.resourceLimit || exit == 152 {
		return Incomplete, "TLC resource limit reached; verification did not complete"
	}
	if !p.started || !p.finished || p.version == "" || p.otherError {
		return ToolError, "TLC did not produce a recognized completed result; see tlc.log"
	}
	switch {
	case exit == 0 && p.success && !p.deadlock && !p.invariant:
		return Passed, "No modeled violation found under the recorded assumptions"
	case exit == 11 && p.deadlock && !p.success && !p.invariant:
		return Deadlock, "Potential whole-program deadlock; inspect the trace and abstraction assumptions"
	case exit == 12 && p.invariant && !p.success && !p.deadlock:
		return SynchronizationError, "Potential invalid synchronization operation; inspect the trace and assumptions"
	default:
		return ToolError, "TLC outcome and exit status do not match a supported result; see tlc.log"
	}
}
