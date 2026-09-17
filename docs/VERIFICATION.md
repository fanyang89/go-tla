# Verification workflow and result interpretation

## Extract, inspect, check

```sh
go build -o gotla ./cmd/gotla
./gotla inspect ./examples/unbuffered
./gotla analyze -out out ./examples/unbuffered
./gotla check -out out -tlc-jar /absolute/path/tla2tools.jar ./examples/unbuffered
```

- `inspect`: print the extracted model, diagnostics, and assumptions.
- `analyze`: write `model.json`, `model.tla`, and `model.cfg`, then print a copyable
  POSIX-shell TLC command. It does not run the checker.
- `check`: extract and generate, then run TLC and save `tlc.log` plus `result.json`.

All commands describe behavioral IR size, not reachable TLC states or generated
runtime actions. Partial unsupported models are labeled as partial by `inspect`.
The CLI never executes the analyzed Go program.

## Tool provisioning

Provision Java and the official TLC JAR explicitly. For a reproducible Linux/Bash
setup matching CI:

```sh
mkdir -p "$HOME/tmp/gotla-tools"
bash scripts/download-tlc.sh "$HOME/tmp/gotla-tools/tla2tools.jar"
export TLC_JAR="$HOME/tmp/gotla-tools/tla2tools.jar"
./gotla check -out out ./examples/unbuffered
```

The provisioning script uses the version/digest in
[scripts/tlc.env](../scripts/tlc.env), checked against the official GitHub release
API, and stages downloads before accepting their checksum. An existing mismatched
file fails verification and is not overwritten. Neither CLI commands nor tests
download the JAR implicitly.

`check` resolves the JAR in this order: `-tlc-jar`, `TLC_JAR`, current-directory
`tla2tools.jar`. It records the actual SHA-256 and TLC version. It does not enforce
CI's pinned digest for a user-selected JAR; use the provisioning script to obtain
the tested version. Protocol/exit mismatches from other versions fail closed rather
than becoming a successful result. Java defaults to `java` on PATH.

## Checker options and bounds

All flags precede the package pattern:

```sh
./gotla check -out out -tlc-jar "$TLC_JAR" -java java \
  -timeout 2m -memory-mib 512 -workers 1 -max-log-mib 16 ./examples/unbuffered
```

| Flag | Default | Boundary |
|---|---|---|
| `-timeout` | `2m` | Positive, at most 24h; Java startup and TLC deadline, **not** the Go analysis phase |
| `-memory-mib` | `512` | 64..65536; JVM maximum heap, not total process/native memory |
| `-workers` | `1` | 1..64 TLC workers |
| `-max-log-mib` | `16` | 1..1024; combined Java/TLC output cap; exceeding it stops the run as incomplete |
| `-java` | `java` | Executable path or PATH name, not a shell command |
| `-trust-call` | empty | Same explicit contracts as `analyze`; not a way to bypass unknown effects |

The subprocess is invoked directly without a shell. Relative JAR/Java paths are
resolved before entering the private checker workspace. `JAVA_TOOL_OPTIONS`,
`JDK_JAVA_OPTIONS`, and `_JAVA_OPTIONS` are removed from the checker environment so
they cannot override the recorded heap/launcher settings. TLC uses breadth-first
checking with fixed seed 1 and fingerprint index 0. Heap and log limits are not
an OS sandbox, total-memory guarantee, or disk-space quota for TLC's state store.
Use external process/container limits when those guarantees are required.

## Two separate outcomes

`result.json`'s `analysisOutcome` records extraction precision; its top-level
`status` records the check outcome. Unknown predicates and lost payload/shared-memory
correlations can produce false positives. “Precisely modeled” is relative to the
supported domain, not arbitrary Go correctness.

| `check` status | Exit | Meaning |
|---|---|---|
| `passed` | 0 | TLC completed; no modeled violation found under the recorded assumptions |
| `deadlock` | 3 | Potential reachable whole-program deadlock; inspect the trace and abstractions |
| `synchronization-error` | 4 | Potential invalid channel, Mutex, or WaitGroup operation |
| `unsupported` | 5 | Extraction refused; no executable TLA+ emitted, TLC not run |
| `analysis-error` | 1 | Package loading/analysis failed; no successful extraction |
| `tool-error` | 1 | Java/JAR, protocol, generation, or output failure; not a verified program bug |
| `incomplete` | 6 | Cancellation, deadline, log/state-space limit, or recognized memory exhaustion |
| `running` | none | Nonfinal on-disk marker; never a completed verification result |

Invalid arguments return 2 without starting a run. Output preparation/locking
failures return 1 and may prevent writing a new result. `inspect`/`analyze` retain
their existing 0/1/2 exit convention. Use the built binary for automation: `go run`
wraps nonzero program exits and therefore does not preserve this exit-code contract.

The checker parses TLC's framed `-tool` messages and cross-checks outcome, completion,
and process exit. A success-looking text line alone is insufficient. A deadlock
requires the matching violation message and exit 11; a synchronization error requires
the generated `NoSynchronizationErrors` invariant and exit 12. Unknown/conflicting
results, malformed/truncated frames, parsing failures, and unexpected exits never
become a pass. Resource/cancellation reasons take precedence over partial outcomes.

Neither fairness/starvation, goroutine leaks, nor payload correctness are checked.
Main return ends the program even if workers remain blocked. The implicit sequential
panic/resource-exhaustion assumption concerns the analyzed Go program, not failures
of the checker itself. False trusted-call contracts invalidate conclusions.

## Result and source evidence

`result.json` schema version 1 contains:

- A unique run ID, timestamps, status/reason, final gotla `exitCode`, and separate
  extraction outcome. `tlcExitCode` is the checker process exit, not gotla's exit.
- Source directory, package patterns, trusted calls, assumptions, and diagnostics.
- IR-size statistics and effective requested checker configuration.
- Optional `runtimeProcs`: the explicit conditional target startup GOMAXPROCS value,
  also stored as metadata option `startup.GOMAXPROCS` in the hashed model. This is
  not the analyzer's observed host setting or a TLC worker limit. Run the target
  with that startup environment when relying on the condition; setters/resets/escapes
  and trusted-call combinations are refused. Other environment dimensions are not
  established by this flag.
- Go/tool build version (revision when available), resolved Java/JAR paths, Java/TLC
  versions when executed, JAR SHA-256, actual command arguments, and TLC exit code.
- Checker duration in milliseconds, raw state-statistics text when available, and
  current artifact paths with SHA-256 values (`model.json`, `.tla`, `.cfg`, `tlc.log`).
- Best-effort source candidates for recognized trace process locations. Invariant
  failures include candidates from the previous state because the failing operation
  can advance the process location.

Durations exclude Go analysis; `config.timeoutNanoseconds` serializes the duration
flag explicitly in nanoseconds. Tool revision describes gotla, not the analyzed
source repository. Preserve the analyzed source revision and package-loading
configuration/environment separately; automatic complete build/input capture and
versioning of the behavioral IR itself remain M3/future work.

Source candidates are outgoing IR transitions at recognized trace locations, not
proof that each listed statement caused the bug. Guards, choice, registration, and
completion can make this list ambiguous. Full trace decoding/replay is not implemented.
The complete checker output remains in `tlc.log` (truncated only at the configured
limit, which produces `incomplete`). Normal model checking uses a private workspace
with copies of the exact generated TLA/config; temporary states and extra TLC trace
modules are removed on normal command return. Reproduce from the saved model/config,
JAR digest, options, and raw log, not the removed temporary workspace.

## Output safety and cancellation

Use a dedicated output directory. Both `analyze` and `check` reserve
`model.json`, `model.tla`, `model.cfg`, `result.json`, and `tlc.log`; these files are
invalidated **before** package loading after acquiring `.gotla.lock`. Even an early
load failure cannot leave a previous success masquerading as the current run.
Unrelated files and directories are preserved. Concurrent writers to the same
output directory fail rather than interleave artifacts.

`check` writes a `running` result before analysis and replaces it atomically with
the final result. Individual artifacts are written by same-directory rename;
file existence alone is not completion. Only inspect final artifacts after the
command returns, checking its exit status, result status/run ID, and hashes.
`analyze` invalidates previous check results but does not write a new check result.

Ctrl-C/SIGTERM cancels package loading and the Java subprocess. Cancellation is
checked between analysis passes; SSA construction/lowering cannot yet be interrupted
mid-pass. The checker deadline does not bound those passes. A final `incomplete`
result is saved when output remains writable. Fatal termination (e.g. SIGKILL) or
output I/O failure may leave `running`, no result, a private workspace, or a lock.
These are never passes. After verifying that no writer is active, manually remove
only that output directory's stale `.gotla.lock` and any identified abandoned
`.gotla-tlc-*` workspace, or choose a fresh output directory. There is no automatic
lock expiry that could race an active long-running analysis.

Argument errors and lock/preparation failures do not promise a new result. Never
interpret an old result as a new run after a command that could not acquire output.

## Tests and required CI gate

Fast local checks:

```sh
go test ./...
go vet ./...
```

Without `TLC_JAR`, real model-checker and CLI/TLC integration tests explicitly skip.
That run alone is not the semantic verification gate. Enforce a required checker:

```sh
GOTLA_REQUIRE_TLC=1 TLC_JAR=/absolute/path/tla2tools.jar go test ./... -count=1
# The pinned strict gate used by CI:
TLC_JAR=/absolute/path/tla2tools.jar bash scripts/test-ci.sh
```

The script verifies the official digest and Java availability, prints versions,
clears ambient `GOFLAGS` that could filter tests, runs vet/full uncached verbose
tests, and checks tracked snapshots were not modified. Snapshot regeneration is
forbidden. Original semantic TLC tests use a 30-second deadline, 512 MiB heap and
one worker. Additional real CLI tests cover pass, deadlock, invariant violation,
and deadline classification. Shell-free fake-Java subprocess tests cover tool
failures, misleading output, cancellation, and log limits without downloading tools.

[GitHub Actions](../.github/workflows/ci.yml) provisions Go from `go.mod`, Java 25,
and the pinned TLC; it has a 15-minute job limit and preserves the verification log
when available. Configure branch protection to require **Go and required TLC**
separately; adding a workflow does not change hosting policy. A local gate pass is
not evidence that GitHub-hosted CI ran.

## Regression baselines

- IR/TLA snapshots: `tests/testdata/{unbuffered,select}.{json,tla}`.
- Eight-example IR size baseline: [model-sizes.json](../tests/testdata/model-sizes.json).
- Repeated analysis checks IR/diagnostic/TLA/config determinism for all eight examples.
  This does not promise stable IDs across source/toolchain changes.
- Sequential and unbuffered examples each retain five IR transitions.

Intentionally regenerate only after semantic review:

```sh
UPDATE_SNAPSHOTS=1 go test ./tests -run '^TestExamplesAndSnapshots$'
git diff -- tests/testdata
```

Then run the strict gate with regeneration disabled. See the
[coverage matrix](SEMANTIC_TEST_MATRIX.md) for evidence and remaining gaps.
