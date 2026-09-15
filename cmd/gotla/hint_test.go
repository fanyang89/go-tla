package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTLCHint(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("TLC_JAR", "")
	var out bytes.Buffer
	printTLCHint(&out, "output")
	if !strings.Contains(out.String(), "Set TLC_JAR") || !strings.Contains(out.String(), "${TLC_JAR:?") {
		t.Fatalf("missing JAR setup guidance: %s", &out)
	}
	if err := os.WriteFile("tla2tools.jar", nil, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	printTLCHint(&out, "output")
	if !strings.Contains(out.String(), "java -cp "+shellQuote(filepath.Join(dir, "tla2tools.jar"))) {
		t.Fatalf("local JAR not detected: %s", &out)
	}
	t.Setenv("TLC_JAR", "tools/my 'TLC.jar")
	out.Reset()
	printTLCHint(&out, "model's output")
	want := "Run TLC:\n  (cd " + shellQuote(filepath.Join(dir, "model's output")) + " && java -cp " + shellQuote(filepath.Join(dir, "tools/my 'TLC.jar")) + " tlc2.TLC -workers 1 model.tla)\n"
	if out.String() != want {
		t.Fatalf("hint = %q, want %q", out.String(), want)
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("a'b $HOME; x"); got != "'a'\"'\"'b $HOME; x'" {
		t.Fatalf("unsafe quoting: %s", got)
	}
}
