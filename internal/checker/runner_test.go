package checker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The test executable doubles as a shell-free Java stand-in. Normal unit tests
// exercise subprocess failure/limits without installing or downloading TLC.
func TestMain(m *testing.M) {
	if mode := os.Getenv("GOTLA_TEST_JAVA"); mode != "" && len(os.Args) > 1 && (os.Args[1] == "-version" || strings.HasPrefix(os.Args[1], "-Xmx")) {
		if len(os.Args) == 2 && os.Args[1] == "-version" {
			fmt.Println(`openjdk version "fixture"`)
			if mode == "probe-only" {
				fmt.Print(transcript(frame(2193, 0, "misleading version probe")))
			}
			os.Exit(0)
		}
		if os.Getenv("JAVA_TOOL_OPTIONS") != "" || os.Getenv("JDK_JAVA_OPTIONS") != "" || os.Getenv("_JAVA_OPTIONS") != "" {
			os.Exit(99)
		}
		if _, err := os.Stat("model.tla"); err != nil {
			os.Exit(98)
		}
		switch mode {
		case "input-files":
			for name, want := range map[string]string{"model.tla": "spec\nUnicode: 证明\n", "model.cfg": "config\nSPECIFICATION Spec\n"} {
				data, err := os.ReadFile(name)
				info, statErr := os.Stat(name)
				if err != nil || statErr != nil || string(data) != want || info.Mode().Perm() != 0600 {
					os.Exit(97)
				}
			}
			fmt.Print(transcript(frame(2193, 0, "verified input files")))
		case "pass":
			fmt.Print(transcript(frame(2193, 0, "success")))
		case "deadlock":
			fmt.Print(transcript(frame(2114, 1, "deadlock")))
			os.Exit(11)
		case "probe-only":
			fmt.Println("No actual TLC protocol")
		case "raw-memory":
			fmt.Println("java.lang.OutOfMemoryError: Java heap space")
			os.Exit(1)
		case "failure":
			fmt.Println("No error has been found")
			os.Exit(150)
		case "timeout":
			time.Sleep(time.Minute)
		case "log-limit":
			for range 1024 {
				fmt.Println(strings.Repeat("x", 4096))
			}
		case "memory":
			fmt.Print(frame(1001, 1, "out of memory"))
			os.Exit(153)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestRunnerBoundedOutcomes(t *testing.T) {
	java, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		mode string
		want Status
	}{{"pass", Passed}, {"deadlock", Deadlock}, {"failure", ToolError}, {"timeout", Incomplete}, {"log-limit", Incomplete}, {"memory", Incomplete}, {"raw-memory", Incomplete}, {"probe-only", ToolError}} {
		t.Run(c.mode, func(t *testing.T) {
			t.Setenv("GOTLA_TEST_JAVA", c.mode)
			t.Setenv("JAVA_TOOL_OPTIONS", "must be removed")
			t.Setenv("JDK_JAVA_OPTIONS", "must be removed")
			t.Setenv("_JAVA_OPTIONS", "must be removed")
			dir := t.TempDir()
			jar := filepath.Join(dir, "tool's jar.jar")
			if err := os.WriteFile(jar, []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			cfg := DefaultConfig()
			cfg.Java, cfg.JAR, cfg.MaxLogMiB = java, jar, 1
			if c.mode == "timeout" {
				cfg.Timeout = 300 * time.Millisecond
			}
			log := filepath.Join(dir, "tlc.log")
			r := Run(t.Context(), cfg, "spec", "config", log)
			if r.Status != c.want {
				t.Fatalf("%s: %+v", c.mode, r)
			}
			info, err := os.Stat(log)
			if err != nil || info.Size() > 1<<20 {
				t.Fatalf("unbounded/missing log: %v", err)
			}
			if r.JARSHA256 == "" || r.JavaVersion == "" || r.DurationMS < 0 {
				t.Fatalf("missing provenance: %+v", r)
			}
			work, _ := filepath.Glob(filepath.Join(dir, ".gotla-tlc-*"))
			if len(work) != 0 {
				t.Fatal("private workspace leaked")
			}
		})
	}
}

func TestRunnerPrivateInputFiles(t *testing.T) {
	java, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOTLA_TEST_JAVA", "input-files")
	dir := t.TempDir()
	jar := filepath.Join(dir, "fixture.jar")
	if err := os.WriteFile(jar, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.Java, cfg.JAR = java, jar
	r := Run(t.Context(), cfg, "spec\nUnicode: 证明\n", "config\nSPECIFICATION Spec\n", filepath.Join(dir, "tlc.log"))
	if r.Status != Passed {
		t.Fatalf("private inputs changed: %+v", r)
	}
	wrong := Run(t.Context(), cfg, "config\nSPECIFICATION Spec\n", "spec\nUnicode: 证明\n", filepath.Join(dir, "swapped.log"))
	if wrong.Status != ToolError {
		t.Fatal("subprocess input-content oracle accepted swapped files")
	}
	work, err := filepath.Glob(filepath.Join(dir, ".gotla-tlc-*"))
	if err != nil || len(work) != 0 {
		t.Fatal("private workspace cleanup changed")
	}
}

func TestRunnerCancellationAndSetupFailure(t *testing.T) {
	cfg := DefaultConfig()
	for _, c := range []struct{ name, jar string }{{"missing", ""}, {"bad-path", "/nonexistent/gotla-test.jar"}, {"directory", t.TempDir()}} {
		t.Run(c.name, func(t *testing.T) {
			cfg.JAR = c.jar
			if r := Run(t.Context(), cfg, "", "", filepath.Join(t.TempDir(), "tlc.log")); r.Status != ToolError {
				t.Fatalf("setup failure misclassified: %+v", r)
			}
		})
	}
	jar := filepath.Join(t.TempDir(), "fixture.jar")
	if err := os.WriteFile(jar, nil, 0600); err != nil {
		t.Fatal(err)
	}
	cfg.JAR = jar
	cfg.Java = "/nonexistent/java"
	if r := Run(t.Context(), cfg, "", "", filepath.Join(t.TempDir(), "tlc.log")); r.Status != ToolError {
		t.Fatalf("missing Java accepted: %+v", r)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if r := Run(ctx, cfg, "", "", filepath.Join(t.TempDir(), "tlc.log")); r.Status != Incomplete {
		t.Fatalf("cancelled setup accepted: %+v", r)
	}
}

func TestRunnerLiveCancellation(t *testing.T) {
	t.Setenv("GOTLA_TEST_JAVA", "timeout")
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	java, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	jar := filepath.Join(dir, "fixture.jar")
	if err := os.WriteFile(jar, nil, 0600); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.Java, cfg.JAR, cfg.Timeout = java, jar, 5*time.Second
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	timer := time.AfterFunc(200*time.Millisecond, cancel)
	defer timer.Stop()
	start := time.Now()
	r := Run(ctx, cfg, "spec", "config", filepath.Join(dir, "tlc.log"))
	if r.Status != Incomplete || !strings.Contains(r.Reason, "context canceled") || time.Since(start) >= 3*time.Second {
		t.Fatalf("parent cancellation did not stop subprocess promptly: %+v", r)
	}
}

func TestConfigValidation(t *testing.T) {
	for _, mutate := range []func(*Config){
		func(c *Config) { c.Workers = 0 }, func(c *Config) { c.Workers = 65 },
		func(c *Config) { c.Timeout = 0 }, func(c *Config) { c.MemoryMiB = 1 },
		func(c *Config) { c.MaxLogMiB = 0 }, func(c *Config) { c.Java = "" },
	} {
		cfg := DefaultConfig()
		mutate(&cfg)
		if cfg.Validate() == nil {
			t.Fatal("invalid resource option accepted")
		}
	}
}
