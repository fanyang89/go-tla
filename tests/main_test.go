package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	jar := os.Getenv("TLC_JAR")
	if err := validateTLCJar(os.Getenv("GOTLA_REQUIRE_TLC") == "1", jar); err != nil {
		fmt.Fprintln(os.Stderr, "TLC prerequisite:", err)
		os.Exit(1)
	}
	if jar != "" {
		if _, err := exec.LookPath("java"); err != nil {
			fmt.Fprintln(os.Stderr, "TLC prerequisite: Java unavailable:", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

func validateTLCJar(required bool, jar string) error {
	if jar == "" {
		if required {
			return fmt.Errorf("GOTLA_REQUIRE_TLC=1 requires TLC_JAR; refusing to skip model checking")
		}
		return nil
	}
	info, err := os.Stat(jar)
	if err != nil {
		return fmt.Errorf("TLC_JAR: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("TLC_JAR must point to a regular file: %s", jar)
	}
	return nil
}

func TestTLCPrerequisites(t *testing.T) {
	if err := validateTLCJar(false, ""); err != nil {
		t.Fatal(err)
	}
	if err := validateTLCJar(true, ""); err == nil {
		t.Fatal("required checker silently skipped")
	}
	dir := t.TempDir()
	for _, path := range []string{dir, filepath.Join(dir, "missing.jar")} {
		if err := validateTLCJar(false, path); err == nil {
			t.Fatalf("bad explicit JAR accepted: %s", path)
		}
	}
	jar := filepath.Join(dir, "fixture.jar")
	if err := os.WriteFile(jar, nil, 0600); err != nil {
		t.Fatal(err)
	}
	// Content integrity is enforced by test-ci.sh; TLC itself rejects invalid JARs.
	if err := validateTLCJar(true, jar); err != nil {
		t.Fatal(err)
	}
}
