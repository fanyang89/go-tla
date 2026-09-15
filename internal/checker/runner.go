package checker

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var errLogLimit = errors.New("TLC log limit reached")

// FindJAR applies explicit flag > environment > current-directory JAR precedence.
func FindJAR(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if path := os.Getenv("TLC_JAR"); path != "" {
		return path
	}
	if info, err := os.Stat("tla2tools.jar"); err == nil && info.Mode().IsRegular() {
		return "tla2tools.jar"
	}
	return ""
}

// Run streams bounded combined output to logPath (which must not already exist).
// TLC gets a private workspace so stale states/configuration/trace modules cannot
// affect a new run. Only this newly created workspace is removed afterward.
func Run(parent context.Context, cfg Config, spec, configText, logPath string) (report Report) {
	start := time.Now()
	report.Status = ToolError
	defer func() { report.DurationMS = time.Since(start).Milliseconds() }()
	fail := func(err error) Report {
		report.Reason = err.Error()
		return report
	}
	if err := cfg.Validate(); err != nil {
		return fail(err)
	}
	deadline, stop := context.WithTimeout(parent, cfg.Timeout)
	defer stop()
	ctx, cancel := context.WithCancelCause(deadline)
	defer cancel(nil)
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fail(fmt.Errorf("create TLC log: %w", err))
	}
	defer log.Close()
	output := &boundedLog{file: log, remaining: int64(cfg.MaxLogMiB) << 20, cancel: cancel}
	if err := ctx.Err(); err != nil {
		report.Status = Incomplete
		return fail(fmt.Errorf("checker cancelled before startup: %w", err))
	}
	if cfg.JAR == "" {
		return fail(fmt.Errorf("TLC JAR unavailable: set -tlc-jar or TLC_JAR, or provision ./tla2tools.jar"))
	}
	report.JAR, err = filepath.Abs(cfg.JAR)
	if err != nil {
		return fail(err)
	}
	report.JARSHA256, err = hashJAR(ctx, report.JAR)
	if err != nil {
		if ctx.Err() != nil {
			report.Status = Incomplete
		}
		return fail(fmt.Errorf("read TLC JAR: %w", err))
	}
	report.Java, err = exec.LookPath(cfg.Java)
	if err != nil {
		return fail(fmt.Errorf("Java unavailable: %w", err))
	}
	report.Java, err = filepath.Abs(report.Java)
	if err != nil {
		return fail(err)
	}
	work, err := os.MkdirTemp(filepath.Dir(logPath), ".gotla-tlc-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(work)
	for name, text := range map[string]string{"model.tla": spec, "model.cfg": configText} {
		if err := os.WriteFile(filepath.Join(work, name), []byte(text), 0600); err != nil {
			return fail(err)
		}
	}
	env := javaEnvironment()
	run := func(args ...string) error {
		cmd := exec.CommandContext(ctx, report.Java, args...)
		cmd.Dir, cmd.Env = work, env
		cmd.Stdout, cmd.Stderr = output, output
		// Bound pipe draining even if a broken Java launcher leaves inherited pipes.
		cmd.WaitDelay = time.Second
		return cmd.Run()
	}
	versionErr := run("-version")
	// The probe is preserved in the raw log but cannot supply TLC outcome frames.
	tlcOffset, err := log.Seek(0, io.SeekCurrent)
	if err != nil {
		return fail(err)
	}
	versionBytes := make([]byte, min(tlcOffset, 4096))
	if probe, err := os.Open(logPath); err == nil {
		n, _ := probe.Read(versionBytes)
		report.JavaVersion = strings.TrimSpace(string(versionBytes[:n]))
		probe.Close()
	}
	var runErr error
	if versionErr == nil && ctx.Err() == nil {
		args := []string{fmt.Sprintf("-Xmx%dm", cfg.MemoryMiB), "-XX:+UseParallelGC", "-cp", report.JAR,
			"tlc2.TLC", "-tool", "-workers", fmt.Sprint(cfg.Workers), "-seed", "1", "-fp", "0", "model.tla"}
		report.Command = append([]string{report.Java}, args...)
		runErr = run(args...)
		if runErr == nil {
			report.ExitCode = new(0)
		} else if exit, ok := errors.AsType[*exec.ExitError](runErr); ok {
			report.ExitCode = new(exit.ExitCode())
		}
	}
	closeErr := log.Close()
	f, err := os.Open(logPath)
	if err != nil {
		return fail(err)
	}
	if _, err := f.Seek(tlcOffset, io.SeekStart); err != nil {
		f.Close()
		return fail(err)
	}
	p, parseErr := parseProtocol(f)
	f.Close()
	report.TLCVersion, report.StateStats = p.version, p.stats
	report.LastPC, report.PreviousPC = p.lastPC, p.previousPC
	if cause := context.Cause(ctx); cause != nil {
		if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) || errors.Is(cause, errLogLimit) {
			report.Status = Incomplete
		}
		report.Reason = cause.Error() + "; verification did not complete"
		return report
	}
	if output.err != nil || closeErr != nil {
		return fail(fmt.Errorf("write TLC log: %w", errors.Join(output.err, closeErr)))
	}
	if versionErr != nil {
		return fail(fmt.Errorf("Java version probe failed: %w", versionErr))
	}
	if p.resourceLimit {
		report.Status, report.Reason = Incomplete, "TLC/JVM resource limit reached; verification did not complete"
		return report
	}
	if parseErr != nil {
		return fail(fmt.Errorf("unrecognized or incomplete TLC protocol: %w", parseErr))
	}
	if report.ExitCode == nil {
		return fail(fmt.Errorf("TLC process failed: %v", runErr))
	}
	report.Status, report.Reason = p.classify(*report.ExitCode)
	return report
}

// Ambient JVM options can override heap/agent configuration. The checker runs
// without these three overrides and records its explicit invocation instead.
func javaEnvironment() []string {
	var out []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key != "JAVA_TOOL_OPTIONS" && key != "JDK_JAVA_OPTIONS" && key != "_JAVA_OPTIONS" {
			out = append(out, entry)
		}
	}
	return out
}

func hashJAR(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("JAR is not a regular file")
	}
	h := sha256.New()
	buf := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buf)
		h.Write(buf[:n])
		if err == io.EOF {
			return fmt.Sprintf("%x", h.Sum(nil)), nil
		}
		if err != nil {
			return "", err
		}
	}
}

type boundedLog struct {
	mu        sync.Mutex
	file      io.Writer
	remaining int64
	cancel    context.CancelCauseFunc
	err       error
}

func (w *boundedLog) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	data := p
	if int64(len(data)) > w.remaining {
		data = data[:w.remaining]
	}
	n, err := w.file.Write(data)
	w.remaining -= int64(n)
	if err == nil && n != len(p) {
		err = errLogLimit
	}
	if err != nil {
		w.err = err
		w.cancel(err)
	}
	return n, err
}
