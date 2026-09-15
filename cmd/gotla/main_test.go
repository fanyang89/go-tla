package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCLIAnalyzeInspectAndStaleArtifacts(t *testing.T) {
	var stdout, stderr bytes.Buffer
	out := t.TempDir()
	if code := run([]string{"analyze", "-out", out, "../../examples/unbuffered"}, &stdout, &stderr); code != 0 {
		t.Fatalf("analyze code %d: %s", code, stderr.String())
	}
	for _, name := range []string{"model.tla", "model.cfg", "model.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatal(err)
		}
	}
	if code := run([]string{"inspect", "../../examples/unbuffered"}, &stdout, &stderr); code != 0 {
		t.Fatalf("inspect: %s", stderr.String())
	}
	// The unknown example lacks a trust contract in this invocation.
	if code := run([]string{"analyze", "-out", out, "../../examples/unknown"}, &stdout, &stderr); code == 0 {
		t.Fatal("unsupported CLI returned success")
	}
	for _, name := range []string{"model.tla", "model.cfg"} {
		if _, err := os.Stat(filepath.Join(out, name)); !os.IsNotExist(err) {
			t.Fatalf("stale %s survived", name)
		}
	}
	if code := run([]string{"nonsense"}, &stdout, &stderr); code != 2 {
		t.Fatal("bad usage exit")
	}
}
