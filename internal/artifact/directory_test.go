package artifact

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryOwnershipAndInvalidation(t *testing.T) {
	out := t.TempDir()
	for _, name := range append(append([]string{}, managedFiles...), "notes.txt") {
		if err := os.WriteFile(filepath.Join(out, name), []byte("old"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	d, err := Begin(out)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, name := range managedFiles {
		if _, err := os.Stat(filepath.Join(out, name)); !os.IsNotExist(err) {
			t.Fatalf("stale file survived: %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "notes.txt")); err != nil {
		t.Fatal("unrelated file removed")
	}
	if err := d.Write("result.json", []byte(`{"status":"running"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := Begin(out); err == nil {
		t.Fatal("concurrent writer acquired lock")
	}
	data, _ := os.ReadFile(filepath.Join(out, "result.json"))
	if string(data) != `{"status":"running"}` {
		t.Fatal("second writer changed current result")
	}
	if err := d.Write("../escape", nil); err == nil {
		t.Fatal("unmanaged write allowed")
	}
	if err := d.Write("result.json", []byte(`{"status":"passed"}`)); err != nil {
		t.Fatal(err)
	}
	if files, _ := filepath.Glob(filepath.Join(out, ".gotla-write-*")); len(files) > 0 {
		t.Fatal("temporary write file leaked")
	}
}

func TestInvalidationFailureReleasesLock(t *testing.T) {
	out := t.TempDir()
	if err := os.MkdirAll(filepath.Join(out, "model.tla", "preserved"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Begin(out); err == nil {
		t.Fatal("expected cleanup failure")
	}
	if _, err := os.Stat(filepath.Join(out, ".gotla.lock")); !os.IsNotExist(err) {
		t.Fatal("failed output preparation left its lock")
	}
	if _, err := os.Stat(filepath.Join(out, "model.tla", "preserved")); err != nil {
		t.Fatal("recursively removed unknown output contents")
	}
}
