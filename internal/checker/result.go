// Package checker runs TLC with bounded execution and fail-closed result parsing.
// It does not load Go packages or change the behavioral model.
package checker

import (
	"fmt"
	"time"
)

type Status string

const (
	Running              Status = "running"
	Passed               Status = "passed"
	Deadlock             Status = "deadlock"
	SynchronizationError Status = "synchronization-error"
	Unsupported          Status = "unsupported"
	AnalysisError        Status = "analysis-error"
	ToolError            Status = "tool-error"
	Incomplete           Status = "incomplete"
)

func (s Status) ExitCode() int {
	switch s {
	case Passed:
		return 0
	case Deadlock:
		return 3
	case SynchronizationError:
		return 4
	case Unsupported:
		return 5
	case Incomplete, Running:
		return 6
	default:
		return 1
	}
}

type Config struct {
	JAR       string        `json:"jar"`
	Java      string        `json:"java"`
	Timeout   time.Duration `json:"timeoutNanoseconds"`
	MemoryMiB int           `json:"heapLimitMiB"`
	Workers   int           `json:"workers"`
	MaxLogMiB int           `json:"logLimitMiB"`
}

func DefaultConfig() Config {
	return Config{Java: "java", Timeout: 2 * time.Minute, MemoryMiB: 512, Workers: 1, MaxLogMiB: 16}
}

func (c Config) Validate() error {
	if c.Timeout <= 0 || c.Timeout > 24*time.Hour {
		return fmt.Errorf("timeout must be positive and at most 24h")
	}
	if c.MemoryMiB < 64 || c.MemoryMiB > 65536 {
		return fmt.Errorf("memory-mib must be in 64..65536 (JVM heap, not total process memory)")
	}
	if c.Workers < 1 || c.Workers > 64 {
		return fmt.Errorf("workers must be in 1..64")
	}
	if c.MaxLogMiB < 1 || c.MaxLogMiB > 1024 {
		return fmt.Errorf("max-log-mib must be in 1..1024")
	}
	if c.Java == "" {
		return fmt.Errorf("java executable must not be empty")
	}
	return nil
}

type Report struct {
	Status      Status            `json:"status"`
	Reason      string            `json:"reason"`
	ExitCode    *int              `json:"tlcExitCode,omitempty"`
	DurationMS  int64             `json:"durationMilliseconds"`
	JAR         string            `json:"jar,omitempty"`
	JARSHA256   string            `json:"jarSHA256,omitempty"`
	Java        string            `json:"java,omitempty"`
	JavaVersion string            `json:"javaVersion,omitempty"`
	TLCVersion  string            `json:"tlcVersion,omitempty"`
	Command     []string          `json:"command,omitempty"`
	StateStats  string            `json:"stateStatistics,omitempty"`
	LastPC      map[string]string `json:"lastTracePC,omitempty"`
	PreviousPC  map[string]string `json:"previousTracePC,omitempty"`
}
