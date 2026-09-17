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

## Static deferred helpers (first full-self prerequisite)

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

## Length-bounded data computation

An independent typed-SSA proof now summarizes externally read-only helpers whose
cycles advance an ordinary int index by one against a stable slice/string length.
Each cycle must cross the dominating comparison header; changed bounds/counters,
wraparound-prone forms and bypass cycles fail. All transitive call bodies and data
types are checked without trusting external functions. Only proved private data
writes are admitted. Shared writes, I/O, synchronization, unsafe code, unknown calls
and recursion are not summarized. No iteration is unrolled or silently truncated.

Lowering rechecks the body/call graph and consumes retained caller operands; argument
evaluation still occurs. Scalar results remain abstract, including branch guards.
Initializer helpers use the same proof. Search/type budgets fail closed. The original
concurrency-loop and resource-identity restrictions remain in force.

This proof is consumed on the **actual own-source** `toolinfo.fromBuildInfo` and
`(*behavior.Model).HasErrors` methods in the complete CLI probe. There are two
source-attributed `finite-data-loop` diagnostics; unsupported-loop diagnostics drop
from 83 to 73, with other categories unchanged. This is a concrete proof result for
these helper computations, not a whole-program coverage percentage. Full self-input
still returns unsupported / 5 and emits no executable TLA+.

Eight new TLC cases cover computation in critical sections, abstract predicate
counterexamples, initialization, blocked argument evaluation, deferred computations,
wrappers and value-struct copies. Tests also reject changing/narrow/overshooting
counters, cycles bypassing progress, shared writes, unknown callbacks and missing
cached graph/slice evidence. A unit test loads the real behavior package rather
than copying HasErrors. Full strict gate and race tests pass with zero skips/failures:
**139 semantic TLC and 18 TLC CLI cases**, plus the refusal regressions. Original
snapshots are unchanged. Evidence is in
`$HOME/tmp/pi/gotla-self-bootstrap-finite-data/{unit.log,focused.log,gate.log,race.log,self-check.log,cli/}`.
No hosted pass or full-self completion is claimed.

## Initializer proof consumption and attribution

Initializer calls now recheck cached callee summaries against the current call graph,
including acyclic pure constructors and calls nested within them. A stale outer or
transitive edge produces a contract error instead of admitting initialization.
Six proof-consumption cases cover intact/corrupted graphs for acyclic and finite
helpers. Four integration cases preserve refusal and call-site attribution for
source, unavailable, dynamic and interface calls. Diagnostic target names are not
proofs: an unresolved static candidate is explicitly marked unproved.

The complete self-input still reports 240 initializer refusals. The enriched messages
identify 104 calls to instantiated `golang.org/x/tools/go/ast/edge.info` and 11 to
`go/types.asGoVersion`; other targets include `os.NewFile`, `reflect.rtypeOf` and
`encoding/base64.NewEncoding`. Source inspection shows the edge metadata factory
uses TypeFor/Elem/FieldByName plus an explicit failure panic, while Go-version
normalization reaches strings.Cut and version parsing. These require actual proofs
or justified semantics, not blanket reflection/string allowlists or skipped init.

The strict gate and full race suite pass; counts remain 139 semantic and 18 CLI/TLC
cases, zero skipped required checks and unchanged original snapshots. Full self-input
remains unsupported / 5 with diagnostic-only artifacts. Local evidence:
`$HOME/tmp/pi/gotla-self-bootstrap-init-targets/{focused.log,gate.log,race.log,self-check.log,initializer-inventory.json,cli/}`.
No whole-self or hosted pass is claimed.

## Proved pre-closed globals (latest continuation)

The actual `context.closedchan` creation and initialization close are now represented
as initial channel state, not skipped under a package exemption. The proof requires
a unique direct make/store in the compiler-generated package initializer, a subsequent
unconditional source init body that only closes that global, and a current unique
call-graph edge. All normal paths from the store reach the close call. Every SSA
function is checked for address escape/rebinding, including functions absent from
or unreachable in the call graph. Proof search is bounded and fails closed. Creation,
close consumption and later global identity resolution recheck these facts.

IR channels have an optional `initiallyClosed` boolean (default false); the saved IR
alone supplies the backend's empty/closed initial state. Existing receive status,
select/default, send and close semantics apply. Open globals, initialization sends,
conditional/repeated closure, factories and unknown effects remain unsupported.
Runtime sends/closes on a pre-closed channel produce the normal synchronization error.

Eight new actual TLC cases cover repeated and concurrent receives, buffered empty
status, range completion, select/default, send/close errors, JSON round trips and an
IR initial-state mutation that restores the expected deadlock. Refusal and corruption
tests cover order, current graph/body, capacity, escaped/rebound globals, removed graph
nodes and control paths bypassing closure. Strict gate and full race suite pass with
zero skips/failures and unchanged original snapshots: **147 semantic TLC and 18 CLI/TLC
cases**. The actual self-probe has creation/close evidence at context/context.go:423/426
and 238 initializer refusals instead of 240; other refusal categories are unchanged.
It still returns unsupported / 5 with no executable full self-model.

Local evidence:
`$HOME/tmp/pi/gotla-self-bootstrap-closed-globals/{focused.log,gate.log,race.log,self-check.log,cli/}`.
This is one real dependency effect modeled, not full bootstrap or a hosted CI claim.

Remaining work includes other dependency initialization/effects, other non-receive control loops,
returned/dynamic callback and object identities, runtime synchronization primitives,
I/O/context/process lifecycle semantics and an enforceable finite input/environment
profile. These are implementation tasks, not permission to weaken the acceptance
contract or treat additional component fixtures as completion.
