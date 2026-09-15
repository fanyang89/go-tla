# gotla

An MVP static analyzer that extracts a small **communication-only concurrent
behavioral model from real Go source** and emits TLA+ for TLC. It uses
`go/packages`, `go/types`, and Go SSA—not LLVM—and a separate, backend-independent
behavioral IR.

This is a deliberately restricted analyzer, not a Go compiler or a proof of all
Go behavior. Unsupported constructs fail closed. Read [ARCHITECTURE.md](ARCHITECTURE.md)
before interpreting a successful check.

## Quick start

Requires Go 1.26 or newer. Run from this repository:

```sh
go build -o gotla ./cmd/gotla
./gotla inspect ./examples/unbuffered
./gotla analyze -out out ./examples/unbuffered
```

The output directory contains:

* `model.json`: behavioral IR, source positions, precision outcome, assumptions, diagnostics.
* `model.tla`: readable named actions and synchronization runtime.
* `model.cfg`: specification, synchronization-error invariant, deadlock checking.

Flags precede package patterns. Supply exactly one main package. The package can
be ordinary code in the current Go module; the tool does not execute it. On an
unsupported analysis, exit status is 1, only diagnostic `model.json` is emitted,
and previous `.tla`/`.cfg` files in that output directory are removed. Load failures
produce no new model. Use a separate output directory per analysis.

## Model checking

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
and CLI tests still run. Golden IR/TLA files are in `tests/testdata`; intentionally
regenerate with `UPDATE_SNAPSHOTS=1 go test ./tests -run TestExamplesAndSnapshots`.

The implementation was checked with Go 1.26.2, Java 25, and the official v1.8.0
release jar (SHA-256 `2c903dcd6f50f12b0c0a2e14c1406be782126c7cee64f3e0b8550c2020870293`).
The integration suite checks all eight examples and twenty-five additional cases
covering nil/closed channels, select readiness/dispatch, rendezvous, main exit,
mutex errors, WaitGroup errors/blocking, and committing an abstract branch before
a blocking operation. Additional rejection regressions cover future-store channel
aliases, capture/spawn initialization order, and unsafe synchronization-state mutation.
