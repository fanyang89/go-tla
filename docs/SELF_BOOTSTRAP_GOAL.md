# Full self-bootstrap acceptance contract

## Objective and current status

Objective: go-tla must produce executable TLA+ from its own real Go program and
provide reproducible verification evidence. **Not complete.** Existing logger and
source-capture component checks are useful prerequisites, not substitutes for this
objective. They do not authorize declaring a whole-program self-check successful.

The reference source entry is `./cmd/gotla`, not a separately rewritten coordinator.
A finite input/environment profile may be necessary, but must be explicit, tied to
actual source behavior and enforced by the analyzer. It must not remove troublesome
initializers, replace production calls with trusted no-ops, or truncate behavior
silently. A passing bounded concurrency model is not a theorem of compiler soundness,
functional correctness or arbitrary Go executions.

## Required end-to-end evidence

- [ ] The actual CLI entry and relevant production/dependency effects are analyzed;
      all accepted operations have consumed proofs or explicit, justified model
      semantics. Unresolved calls/identity/control continue to fail closed.
- [ ] Input, environment, finite-state bounds, excluded runtime behavior and checked
      properties are recorded. The model represents the real operation, not merely
      an empty/help path or the two existing component harnesses.
- [ ] The normal source -> SSA -> behavioral IR -> TLA+ pipeline emits executable
      self-model artifacts with nonempty meaningful synchronization and source/provenance
      evidence. A diagnostic-only model or unsupported exit does not satisfy this.
- [ ] The pinned actual TLC checks the generated self-model to completion. Results,
      checker identity, artifact hashes and any abstractions are inspectable.
- [ ] Mutating actual production synchronization produces the expected counterexample
      or justified refusal; the model must not remain vacuously successful.
- [ ] The self-analysis CLI regression is upgraded only with that evidence. Existing
      independent semantic/IR/backend/refusal tests, strict gate and full race suite
      pass without skipped required TLC checks or unreviewed snapshot regeneration.

The full-CLI regression currently correctly requires unsupported / 5 and no executable
artifacts. No completion is claimed until every item above has direct evidence.

## Current implementation work

Static, source-defined deferred helper/closure bodies are now analyzed at normal-return
cleanup. Registration captures identities; LIFO cleanup enters the actual helper
body and waits for its normal return before earlier cleanup can proceed. Primitive
Unlock/Done behavior and existing snapshots are unchanged. Dynamic/interface/field
defer targets, bound wrappers, repeated registration, foreign stacks and panic/recover
remain outside this increment. Trusting a deferred helper does not skip its body.

Twelve new actual TLC cases cover helper/closure/method cleanup, nested scopes, LIFO,
blocking, synchronization errors, argument identity snapshots, conditional registration
and multiple returns. Graph/slice corruption and unproved/recursive/dynamic bodies
remain rejected. Totals: 131 semantic TLC and 18 TLC CLI cases, plus the separate
full-CLI and re-entry refusals. The strict local gate and full race suite pass with
zero skips/failures; original snapshots are unchanged.

This removes the old early rejection at `cmd/gotla/main.go`'s `defer dir.Close()`.
The real self-input probe now reaches deeper `check.go`/artifact cleanup paths and
exposes more unsupported operations. Counts are not a coverage metric: the observed
probe reports 240 initializer, 124 unknown-effect, 268 identity, 250 receive-loop,
228 sync-method, 83 loop, 46 exception, 42 unsafe-pointer, 9 topology, 9 defer and
1 channel-field-alias diagnostics. Repeated inline sites contribute duplicates.
It still returns unsupported / 5 and emits only diagnostic `model.json` plus the
result envelope. No TLC self-proof exists yet.

Evidence: `$HOME/tmp/pi/gotla-self-bootstrap-defer/` contains `focused.log`, `gate.log`,
`gate-attempt1.log`, `race.log`, `self-check.log` and `cli/` artifacts. The initial gate
caught a stale rejection expectation for an empty literal defer; that case is now a
positive TLC test and unresolved dynamic defers remain refusal tests. These results
are local-only; no hosted pass is claimed.

Remaining work includes dependency initialization/effects, non-receive control loops,
returned/dynamic callback and object identities, runtime synchronization primitives,
I/O/context/process lifecycle semantics and an enforceable finite input/environment
profile. These are implementation tasks, not permission to weaken the acceptance
contract or treat additional component fixtures as completion.
