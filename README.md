# gotla

An MVP static analyzer that extracts a small **communication-only concurrent
behavioral model from real Go source** and emits TLA+ for TLC. It uses
`go/packages`, `go/types`, and Go SSA—not LLVM—and a separate, backend-independent
behavioral IR.

This is a deliberately restricted analyzer, not a Go compiler or a proof of all
Go behavior. Unsupported constructs fail closed. Read [ARCHITECTURE.md](ARCHITECTURE.md)
before interpreting a successful check.

## Development target

The next milestone is a basically usable checker for small Go concurrency
components with explicit finite analysis scope—not a general Go verifier.
[ROADMAP.md](ROADMAP.md) defines the target workflow, semantic boundaries,
acceptance criteria, and staged implementation plan. Planned capabilities there
are not supported features until implemented and tested.

- [Supported Go patterns](docs/SUPPORTED_GO.md)
- [Verification workflow and result interpretation](docs/VERIFICATION.md)
- [Semantic test coverage and gaps](docs/SEMANTIC_TEST_MATRIX.md)
- [Component harnesses and measurements](docs/COMPONENTS.md)
- [Local and hosted acceptance evidence](docs/ACCEPTANCE.md)
- [Versioned behavioral IR contract](docs/BEHAVIOR_IR.md)

## Quick start

Requires Go 1.26 or newer. Run from this repository:

```sh
go build -o gotla ./cmd/gotla
./gotla inspect ./examples/unbuffered
./gotla analyze -out out ./examples/unbuffered
# With Java and TLC provisioned, extract and check in one command:
./gotla check -tlc-jar /absolute/path/tla2tools.jar -out out ./examples/unbuffered
```

The output directory contains:

* `model.json`: version-1 behavioral IR, tool/configuration metadata, source positions,
  precision outcome, assumptions and diagnostics. Unversioned artifacts must be regenerated.
* `model.tla`: readable named actions and synchronization runtime.
* `model.cfg`: specification, synchronization-error invariant, deadlock checking.

All three commands print model-size statistics (IR processes, channels, mutexes,
WaitGroups, locations, transitions, and abstract predicates). These are not TLC
state-space counts. `check` additionally saves `tlc.log` and `result.json` with the
checker outcome, assumptions, options, tool versions, artifact hashes, and source
candidates for counterexamples.

Flags precede package patterns. Supply exactly one main package. The package can
be ordinary code in the current Go module; the tool does not execute it. Use a
dedicated output directory: `analyze` and `check` reserve and invalidate
`model.{json,tla,cfg}`, `result.json`, and `tlc.log` before loading packages, including
on load failure. They exclude concurrent writers with `.gotla.lock`. Unsupported
`analyze` exits 1 and emits diagnostic JSON only; unsupported `check` exits 5 and
also records its result, without running TLC. Argument/output-lock failures do not
start a new run. See [output safety](docs/VERIFICATION.md#output-safety-and-cancellation).

## Model checking

`check` is the one-command workflow:

```sh
./gotla check -out out -timeout 2m -memory-mib 512 -workers 1 ./examples/unbuffered
```

JAR lookup order is `-tlc-jar`, `TLC_JAR`, then `./tla2tools.jar`. Java defaults to
`java` on PATH (`-java` overrides it). The deadline covers the checker, not Go
analysis; the memory option limits JVM heap, not total process memory. Output is
capped at 16 MiB by default (`-max-log-mib`). No tool is downloaded implicitly.

| Exit | Check result |
|---|---|
| 0 | `passed`: no modeled violation found under recorded assumptions |
| 1 | `analysis-error` or `tool-error`, including output failure |
| 2 | Invalid arguments |
| 3 | `deadlock` |
| 4 | `synchronization-error` |
| 5 | `unsupported` |
| 6 | `incomplete`: cancellation, timeout, or resource limit |

Use the built binary for these exit codes; `go run` wraps nonzero program exits.
`result.json` also distinguishes extraction precision from the checker outcome.
Source summaries are candidates from trace locations, not complete counterexample
decoding. See the [verification guide](docs/VERIFICATION.md) for the result contract.

### Run TLC manually

Obtain `tla2tools.jar` from the [official TLA+ releases](https://github.com/tlaplus/tlaplus/releases).
No jar is bundled or automatically downloaded. After a successful `analyze`, the
CLI prints a copyable POSIX-shell TLC command. It uses `TLC_JAR` when set, otherwise
looks for `tla2tools.jar` in the current directory. If neither is available, it
prompts you to set `TLC_JAR` to an absolute JAR path. With Java installed:

```sh
cd out
java -cp /absolute/path/tla2tools.jar tlc2.TLC -workers 1 model.tla
```

The unbuffered example should complete without errors. For an intentional deadlock:

```sh
./gotla analyze -out deadlock-out ./examples/deadlock
(cd deadlock-out && java -cp /absolute/path/tla2tools.jar tlc2.TLC model.tla)
```

TLC reports `Deadlock reached`. Normal main return terminates the whole Go program;
blocked workers after main returns are **not** reported as whole-program deadlocks.
No stuttering action is enabled merely because a worker is blocked or finished.

## Examples

| Package | Purpose | Expected TLC result |
|---|---|---|
| `unbuffered` | Spawn, send/receive rendezvous | No error |
| `buffered` | Capacity-two queue and close | No error |
| `deadlock` | Unbuffered send without receiver | Deadlock |
| `select` | Two channels and default readiness | No error |
| `mutex` | Two workers sharing one mutex | No error |
| `waitgroup` | Add, Done, Wait | No error |
| `sequential` | Arithmetic/helpers removed before send | No error |
| `unknown` | Trusted external predicate becomes nondeterministic | Possible deadlock |

The last example requires an **explicit user contract**:

```sh
./gotla analyze -trust-call strings.HasPrefix -out unknown-out ./examples/unknown
```

`-trust-call` is a comma-separated list of exact SSA function names (normally
`import/path.Function`). It asserts that each call terminates, has no shared side
effects, does not synchronize or spawn, and returns normally. Results remain
abstract. A false contract invalidates the analysis. Without that contract the
example's dependency implementation is unsupported, not silently assumed pure.
Trusted call names, warnings, and environment assumptions are emitted in metadata.

## Tests

```sh
go test ./...
go vet ./...
TLC_JAR=/absolute/path/tla2tools.jar go test ./... -count=1
```

Without `TLC_JAR`, only the actual TLC tests skip; frontend, IR, snapshot, backend,
and CLI tests still run. This is not sufficient for the semantic verification gate.
The CI workflow runs the following strict gate with Java 25 and the pinned official
TLC release:

```sh
bash scripts/download-tlc.sh "$HOME/tmp/gotla-tools/tla2tools.jar"
TLC_JAR="$HOME/tmp/gotla-tools/tla2tools.jar" bash scripts/test-ci.sh
```

Provisioning is explicit and checksum-verified. The gate fails for a missing or
mismatched JAR and forbids snapshot regeneration. `GOTLA_REQUIRE_TLC=1` also makes
Go integration tests fail rather than skip when `TLC_JAR` is missing. See the
[verification guide](docs/VERIFICATION.md) for details and CI limitations.

Golden IR/TLA and model-size files are in `tests/testdata`; intentionally
regenerate with `UPDATE_SNAPSHOTS=1 go test ./tests -run TestExamplesAndSnapshots`.

The implementation was checked with Go 1.26.2, Java 25, and the official v1.8.0
release jar (SHA-256 `2c903dcd6f50f12b0c0a2e14c1406be782126c7cee64f3e0b8550c2020870293`).
The original TLC integration suite checks all eight examples and twenty-five additional cases
covering nil/closed channels, select readiness/dispatch, rendezvous, main exit,
mutex errors, WaitGroup errors/blocking, and committing an abstract branch before
a blocking operation. Additional rejection regressions cover future-store channel
aliases, capture/spawn initialization order, and unsafe synchronization-state mutation.
Additional `check` CLI integration tests invoke real TLC for pass/deadlock/invariant
outcomes; subprocess tests cover timeout, log limits, malformed output, cancellation,
and tool failure without requiring Java. Output ownership and stale-artifact tests
cover failed analysis and concurrent writers.
