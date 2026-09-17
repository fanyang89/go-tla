package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fanmi/go-tla/internal/checker"
)

func TestCheckBuiltinOutputBoundary(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{"go.mod": "module fixture\n\ngo 1.26\n", "main.go": `package main;func main(){print("native ");println("output")}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	cmd := exec.CommandContext(t.Context(), "go", "run", ".")
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off")
	var native bytes.Buffer
	cmd.Stderr = &native
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if native.String() != "native output\n" {
		t.Fatalf("native output missing: %q", native.String())
	}
	for _, trust := range []string{"", "print,println"} {
		out := t.TempDir()
		seedOldArtifacts(t, out)
		var stdout, stderr bytes.Buffer
		args := []string{"check", "-out", out, "-tlc-jar", filepath.Join(out, "unused.jar")}
		if trust != "" {
			args = append(args, "-trust-call", trust)
		}
		args = append(args, ".")
		code := runContext(t.Context(), args, &stdout, &stderr)
		r := readResult(t, out)
		if code != 5 || r.Status != checker.Unsupported || len(r.Command) != 0 || len(r.Artifacts) != 1 || r.Artifacts["model.json"].SHA256 == "" {
			t.Fatalf("output silently passed: %d %+v", code, r)
		}
		count := 0
		for _, d := range r.Diagnostics {
			if d.Code == "output-effects" {
				count++
			}
		}
		if count != 2 {
			t.Fatal("output operation diagnostics missing")
		}
		for _, name := range []string{"model.tla", "model.cfg", "tlc.log"} {
			if _, err := os.Stat(filepath.Join(out, name)); !os.IsNotExist(err) {
				t.Fatal("stale executable artifact survived output refusal")
			}
		}
	}
}
