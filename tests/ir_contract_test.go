package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fanmi/go-tla/internal/analysis"
	"github.com/fanmi/go-tla/internal/behavior"
	"github.com/fanmi/go-tla/internal/lowering"
	"github.com/fanmi/go-tla/internal/tla"
)

func TestVersionedExamplesRoundTrip(t *testing.T) {
	for _, name := range []string{"unbuffered", "buffered", "deadlock", "select", "mutex", "waitgroup", "sequential", "unknown"} {
		t.Run(name, func(t *testing.T) {
			m := example(t, name)
			if m.SchemaVersion != behavior.SchemaVersion || m.Metadata.Producer != "gotla" || m.Metadata.Toolchain == "" {
				t.Fatal("missing version/provenance")
			}
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := behavior.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(m, decoded) {
				t.Fatal("IR JSON round trip changed model")
			}
			a, ac, err := tla.Generate(m)
			if err != nil {
				t.Fatal(err)
			}
			b, bc, err := tla.Generate(decoded)
			if err != nil {
				t.Fatal(err)
			}
			if a != b || ac != bc {
				t.Fatal("backend depends on information outside saved IR")
			}
		})
	}
}

func TestSequentialEditsDoNotRenameBehavior(t *testing.T) {
	source := `package main
func worker(ch chan int){ch<-1}
func compute(x int)int{a:=x+1;b:=a*2;return b}
func main(){
_ = 1
ch:=make(chan int)
go worker(ch)
<-ch
}`
	a := fromSource(t, source)
	b := fromSource(t, strings.Replace(source, "_ = 1", "_ = compute(1)", 1))
	if a.HasErrors() || b.HasErrors() {
		t.Fatal("fixture unsupported")
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("irrelevant sequential SSA instructions renamed/changed behavioral IR")
	}
}

func TestOtherProcessEditsDoNotRenumberActions(t *testing.T) {
	source := `package main
func a(ch chan int){ch<-1}
func b(ch chan int){ch<-2}
func main(){ch:=make(chan int,2);go a(ch);go b(ch);<-ch;<-ch}`
	first := fromSource(t, source)
	second := fromSource(t, strings.Replace(source, "ch<-1}", "ch<-1;ch<-3}", 1))
	var processA, processB behavior.Process
	for _, p := range first.Processes {
		if p.Kind == "b" {
			processA = p
		}
	}
	for _, p := range second.Processes {
		if p.Kind == "b" {
			processB = p
		}
	}
	if processA.ID == "" || !reflect.DeepEqual(processA, processB) {
		t.Fatal("unrelated process was renamed")
	}
	var a, b []behavior.Transition
	for _, tr := range first.Transitions {
		if tr.Process == processA.ID {
			a = append(a, tr)
		}
	}
	for _, tr := range second.Transitions {
		if tr.Process == processB.ID {
			b = append(b, tr)
		}
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("unrelated transitions were globally renumbered")
	}
}

func TestQualifiedSourcePathsAndProcessDeclarations(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.test/component\n\ngo 1.26.2\n",
		"main.go": `package main
import("example.test/component/left";"example.test/component/right")
func main(){ch:=make(chan int,2);go left.Work(ch);go right.Work(ch);<-ch;<-ch}`,
		"left/worker.go":  "package left\nfunc Work(ch chan int){ch<-1}\n",
		"right/worker.go": "package right\nfunc Work(ch chan int){ch<-2}\n",
	}
	for path, text := range files {
		path = filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m, err := analysis.Analyze(dir, []string{"."}, lowering.Options{})
	if err != nil || m.HasErrors() {
		t.Fatalf("analysis: %v %+v", err, m)
	}
	seen := map[string]bool{}
	for _, p := range m.Processes {
		if p.Kind == "Work" {
			seen[p.Source.File] = true
			if p.Source.Line != 2 || p.Source.Function != "Work" {
				t.Fatalf("process source is not its declaration: %+v", p.Source)
			}
		}
	}
	if !seen["left/worker.go"] || !seen["right/worker.go"] {
		t.Fatalf("basename collision or absolute path leak: %v", seen)
	}
	for _, tr := range m.Transitions {
		for _, e := range tr.Effects {
			if e.Kind == behavior.Spawn && (tr.SourcePosition.File != "main.go" || tr.SourcePosition.Function != "main") {
				t.Fatal("spawn source not in caller")
			}
		}
	}
}

func TestGlobalAndLocalSynchronizationIDsCannotAlias(t *testing.T) {
	m := fromSource(t, `package main;import "sync";var mu_1 sync.Mutex;func fixture(){var mu sync.Mutex;mu.Lock();mu_1.Lock();mu_1.Unlock();mu.Unlock()};func main(){fixture()}`)
	if m.HasErrors() || len(m.Mutexes) != 2 || m.Mutexes[0] == m.Mutexes[1] {
		t.Fatalf("resource naming aliased distinct allocations: %+v", m)
	}
}
