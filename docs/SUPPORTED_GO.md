# Supported Go: current MVP

This guide describes implemented behavior, not the target in
[ROADMAP.md](../ROADMAP.md). The full semantic contract is in
[ARCHITECTURE.md](../ARCHITECTURE.md).

## Read the outcome first

| Extraction outcome | Meaning |
|---|---|
| `precisely-modeled` | Precise within the declared communication-only domain and environment assumptions, not all Go semantics |
| `conservatively-abstracted` | Possible control behavior is retained with abstraction; infeasible traces may be added |
| `unsupported` | Safe extraction was not established; executable TLA+ must not be generated |

These are extraction outcomes, not TLC results. In particular, a precisely modeled
program may deadlock. Successful extraction does not mean successful verification.

## Program exit boundary

`os.Exit` is an explicit standard-library operation, not a trusted pure-call
exemption. Discovery checks its package/function/signature; consumption rechecks
that identity, the current call graph and retained argument operands. It models
ordinary process execution, excluding Go test's panic-on-exit interception.
Unknown dependency initialization (including `os` initialization) still refuses
whole-program emission. Supporting this instruction alone does not make a program
importing `os` executable by the analyzer. Direct deferred/dynamic exit targets are
not newly supported; an otherwise supported source-defined cleanup helper may
contain a direct exit. No deferred cleanup runs after that exit.

## Supported patterns and limits

| Go pattern | Current treatment |
|---|---|
| One main package, direct nonrecursive helper calls | Supported bodies are lowered in the caller's process |
| `go worker(ch)` / statically bound closure | One statically known process per reachable spawn context |
| Direct `os.Exit(int)` calls | Immediate whole-program exit, including from a worker or source helper; no deferred cleanup; argument effects retained, status abstract |
| `for range N` / `for _ = range N` | Main-module constant integer range; no live index, nested loops or branch statements; at most 16 iterations |
| `make(chan T)` / constant capacity | Supported; capacity must be in 0..1024 |
| Send / receive / close | Supported, including nil blocking, closed-channel receive and synchronization errors |
| `select`, including default | Ready cases are nondeterministic; default is unavailable when a communication is ready |
| Channel parameters / direction conversions | Supported when identity is statically resolved |
| Captured single-assignment channel cell | Initialization must dominate every load and capture; no later mutation |
| Local or direct global Mutex / WaitGroup | Stable identities; passing/copying these objects by value is rejected |
| Proved initially open global channels | Unique constant-capacity direct creation, empty buffer, stable global identity; other initializer effects checked separately |
| Proved pre-closed global channels | Unique direct creation and unconditional single close in package initialization; no rebinding/address escape |
| Inline Mutex / WaitGroup fields, including nested value structs | Static allocation/global object plus field path; zero initialization only for synchronization state |
| Direct pointer-receiver methods on these objects | Supported with statically resolved receiver identities; closures can capture stable object pointers |
| Immutable channel fields in local allocated objects | At most one allocation-frame initialization, dominating every read and object escape; omitted initialization means nil |
| Mutex Lock / Unlock | Boolean lock availability, including unlock from another goroutine and invalid-unlock errors |
| Direct `defer mu.Unlock()` / `defer wg.Done()` | Static receiver capture, conditional registration and LIFO cleanup at normal returns; at most 64 sites per invocation |
| WaitGroup Add / Done / Wait | Single phase only; positive Add in main before any spawn or prior Wait; constant delta in -1024..1024 |
| Local arithmetic / payload processing | Removed when irrelevant and otherwise within supported computation/call rules |
| Close-driven channel ranges / equivalent comma-ok loops | Restricted receive SCCs preserve closure exits and finite modeled state, without guessing a trip count |
| Receive `ok` in direct conditions | Exact for direct receives/select receives, negation and comparison with Boolean constants |
| Other nonconstant concurrency-controlling predicate | Abstract nondeterministic branch with explicit commitment before blocking |
| Channel payload and ordinary memory value correlations | Not tracked; conditions depending on them may be independently abstracted |
| Explicit trusted call | User asserts total, side-effect-free, normally returning, nonsynchronizing behavior; results remain abstract |

The current scope does not verify payload correctness or arbitrary shared-memory
properties. Recognizing a type or method name alone does not establish safe
identity or supported surrounding control flow.

### A supported communication skeleton

```go
func worker(ch chan int) { ch <- 1 }

func main() {
    ch := make(chan int)
    go worker(ch)
    <-ch
}
```

See the runnable [unbuffered example](../examples/unbuffered/main.go). Local
computation between behavioral boundaries need not become separate model steps.

### Static synchronization fields

```go
type component struct { mu sync.Mutex }

func (c *component) work(done chan int) {
    c.mu.Lock()
    c.mu.Unlock()
    done <- 1
}

func main() {
    c := new(component)
    done := make(chan int)
    go c.work(done)
    <-done
}
```

With `import "sync"`, this is supported without replacing the component's locking.
Distinct objects and distinct inline Mutex/WaitGroup fields remain distinct resources; repeated
access through a direct pointer parameter/receiver refers to the same resource.
Nested value-struct fields and zero-initialized direct globals are supported too.
Copying/loading whole synchronization-bearing aggregates, value receivers/arguments,
whole-object resets and initializer copies are rejected, even for zero values.
Pointer fields, array/slice elements, returned object identities, nil receivers
and ambiguous object sources remain unsupported. A closure may capture a pointer
cell for a struct containing inline synchronization but **no channel fields** when
one store dominates all loads/captures and no unsupported pointer-cell escape or
write occurs. This includes the receiver cell of a pointer method launching a
closure. Reassignments, future/conditional initialization, closure writes, nil use
and whole-object copies remain refused; capture does not create a fresh object.
Channel-bearing pointer cells retain the stricter refusal below. Restricted
normal-return cleanup is described below.

### Immutable channel fields

```go
type endpoint struct { ch chan int }

func main() {
    var e endpoint
    e.ch = make(chan int)
    go func() { e.ch <- 1 }()
    <-e.ch
}
```

Channel fields may also be initialized in a local struct literal, including nested
value fields. Direct pointer-receiver calls and bound methods can read them. Fields
initialized with the same channel retain that alias; fields initialized with different
allocations remain distinct. An uninitialized field is nil, with normal nil-channel
blocking/close-error behavior, not an invented channel.

The proof inspects every SSA use, not only sliced operations. A field can have at
most one store in its object's allocation frame. That store must dominate every
field read and every call/capture/spawn exposing the object or its subobjects.
This conservatively requires even unrelated channel-field initializations before
an object escape. Initializers use direct allocations, channel parameters, already
resolved captures, nil, or direction/type conversions of those values. Field stores
seed the dependency slice; only proved initialization stores may be erased.

Reassignment, initialization in a called method, field-address escape, whole-object
copy/reset, field-to-field initializer loads, global channel-field access and returned
objects remain unsupported. Capturing an addressable value object (`var e endpoint`)
is supported after initialization; capturing a pointer variable through a `**endpoint`
cell is currently rejected, even when a human can see it is immutable. Direct pointer
receiver calls such as `go e.send()` do not require that pointer-cell capture.

### Restricted deferred cleanup

```go
func (c *component) work(done chan int) {
    c.mu.Lock()
    defer c.mu.Unlock()
    done <- 1
}
```

Direct `sync.Mutex.Unlock` / `sync.WaitGroup.Done` and source-defined static
helper functions, methods and literal closures are accepted as defers. Helper bodies
are inspected, not treated as pure cleanup. Receivers and resource arguments are
resolved from the values evaluated at registration, not
from later variable assignments. Reaching a defer registers cleanup; it does not
unlock or decrement immediately. Conditional/unselected registrations remain exact.
Cleanup executes in LIFO order at normal returns, after return-expression evaluation.
A function blocked before returning has not executed its deferred cleanup. A deferred
helper may itself block; earlier registered cleanup cannot execute until it returns.
Captured resource cells retain their existing single-store/dominance requirements;
this does not permit future initialization or reassignment of captured identities.

Each inlined invocation has separate registration flags. Nested callees drain only
their own registrations. Flags are cleared as calls execute; multiple compiler
`RunDefers` points cannot run the same registration twice. Main's own cleanup precedes
program termination; workers interrupted by main return are not guaranteed cleanup.

Defer registration/drain sites must remain acyclic, including after integer-range
expansion. Each expanded registration executes at most once per invocation.
Cleanup registered outside a supported receive cycle runs only after return, not
while the function is blocked in that cycle.
The implementation accepts at most 64 sites; exceeding this is unsupported, never
truncated cleanup or an assumed stack bound. Primitive cleanup retains the defer
site's source position; helper effects retain their actual body locations. Graph
and retained-operand proofs are required even for a side-effect-free helper.
Dynamic/interface/field defer targets, synthetic/bound method wrappers, direct deferred
Lock/Wait/Add/close, foreign defer stacks, initializer defers, panic/recover and unproved
loop forms remain unsupported. A trusted-call contract does not extend this whitelist
or allow skipping a deferred helper's body. Implicit
sequential panics remain excluded by the recorded domain assumption; this is not
panic unwinding or exception-safety verification.

### Proved finite integer ranges

```go
const workers = 2

func main() {
    var wg sync.WaitGroup
    wg.Add(workers)
    for range workers {
        go func() { defer wg.Done() }()
    }
    wg.Wait()
}
```

With `import "sync"`, the two goroutines above are distinct static instances. The
range bound must be a Go compile-time integer constant, not a variable that happens
to contain a small value. No integer-range iteration variable is currently supported except `_`.
The body may return or register supported defers: return exits the original function,
and defers execute at its normal return, **not** at the end of each iteration.
Nonpositive bounds execute no body operations.

Bounds above 16, nested loop syntax, labels, break/continue/goto/fallthrough in a body,
and more than 256 KiB of expanded text per file are rejected rather than truncated.
The current normalizer handles main-module function declarations and nested function
literals, not dependency modules or files with existing line directives. Original
source is type-checked first, then equivalent Go is expanded in memory and reloaded
before SSA construction; files on disk and import initialization are preserved.
Proofs and refusal reasons are reported. Original source locations are retained.

Classic `for i := ...` loops, live indices and nonconstant integer ranges are not
accepted by the expansion rule. Entirely read-only length-bounded computations have
the separate proof below; channel ranges have their own non-unrolling rule.
Returned/dynamic resource topology remains unsupported.
Synchronization/alias rules still apply inside accepted loops, including WaitGroup's
single enrollment phase and the per-invocation defer-site limit after expansion.

### Proved scalar-array value ranges

`for i, value := range array` (including a blank index) is expanded when `array` has
a by-value fixed-array type of at most 16 scalar elements and the effective source
language version is Go 1.22 or newer. The value identifier must be nonblank and
bindings must use `:=`. Pointer arrays, key-only ranges, assignment bindings and
reference/resource-bearing elements are not supported by this rule.

The array expression is evaluated once into a fresh, collision-free local snapshot.
Each iteration declares its own index/value in a distinct lexical block; no helper
function is introduced. Returns and defers retain their original function scope.
Even a zero-length two-value range evaluates the expression, while its body stays
nonexecuting but type-checked. Native differential tests cover snapshot mutation,
evaluation count, zero length, closure/defer capture, return and name collisions.
Source locations and on-disk input remain unchanged; the generated Go is re-type-checked.
The existing nested-control restrictions and 256 KiB per-file budget still apply.

Index/value data is still abstract in the synchronization model. A conditional return
based on an index may therefore yield a conservative counterexample; normalization
does not introduce a payload/constant-folding correctness claim.
The production artifact managed-file table is now a fixed array, so its two actual
five-file loops use this proof. File operations and error paths are not omitted.

### Length/constant-bounded data computations

A cyclic helper can be summarized without unrolling when its **entire transitive
body** is externally read-only and provably finite. Examples include scanning a
slice for a scalar condition or summing its elements. This does not infer the result;
returned data stays abstract, including when it controls synchronization.

Each cyclic SSA component needs a dominating header testing an ordinary Go int index
against the length of a stable slice/string header or an int constant in
0..2147483647. The latter fits both Go int widths; larger constants refuse this
proof rather than assuming that host and target architectures match. Fixed-array
ranges and classic constant-bound loops use this rule. Every back edge advances exactly
one from zero (or SSA range's pre-increment form starting at -1); the false branch
exits. Removing the header must break every cycle. This proves progress without
integer-wraparound on a taken back edge and without guessing a runtime trip count.
Changed indices/bounds, narrow counters, inclusive tests, bypass subcycles and nested
unproved cycles are refused. The concurrency expansion limit remains unchanged.

All instructions and transitively called bodies are checked. Data types may be
recursive read-only graphs, but not synchronization objects, channels, interfaces,
callbacks or unsafe pointers. Private value copies/initialization retain the
complete-address-use proof, including fixed-array elements. Local copies may contain
data references: copying a slice header or pointer does not write its backing storage
or pointee. Ownership tracing stops at a load, so a copied reference cannot grant
ownership of borrowed memory. Resetting a private header is permitted; modifying a
caller element/header, a loaded pointer's pointee or a slice's backing array is not.
This widened type rule is only used inside the complete finite-data proof, not as a
general constructor-purity exemption. It proves the actual Model.Statistics helper.
Writes to caller/global data, mutation through unproved container aliases,
printing, unknown/unavailable calls, recursion, defers, panic and synchronization
cannot acquire this summary. User trust flags cannot supply missing proof facts.
The existing implicit-panic/resource-exhaustion domain assumption still applies.

The rotated SSA form of range-over-len is also accepted: each external entry must
test zero against the same stable len and initialize the header phi to zero;
every back edge must test its exact +1 increment below that bound. The header must
dominate the component, with no cycle avoiding it. This preserves a non-wrapping
[0,len) measure. This additional shape does not admit constant bounds or relax
constant-range expansion limits. Actual ASCII Identifier and decimalDigits helpers
use this proof. Their production callers retain ordinary behavior and data abstraction.

Cached call summaries are rechecked against the current graph and body; lowering also
consumes the retained caller operands. Argument evaluation is not erased. Initialization
helpers use the same proof, not a package allowlist. Proof search is capped at 4096
function/instruction steps, with type-graph budgets of 256 nodes and depth 64;
exhaustion refuses the proof. `finite-data-loop` diagnostics identify accepted source
functions. A proof failure does not turn an otherwise unsupported loop into a model.

This is a termination/effect proof for data computation, not a payload, memory-race,
whole-program termination or analyzer-correctness theorem.

### Literal-input data invocation proofs

A separate bounded SSA evaluator can prove one call whose arguments are SSA constants
(including typed nil). This is **not** a callee-wide purity annotation. It reads the
actual current call graph and bodies, takes only exactly determined branches, and
requires a normal return. It never executes loaded application/native functions on
the host. Library names alone do not establish purity; explicitly modeled operations
are separate, recorded semantics assumptions. User trust flags do not provide evaluator facts.

Supported computation includes bounded-size bool/integer/string values, private
data-only structs/arrays, owned array-slice views, field/index access, copies,
selected scalar operations (including bounded string concatenation) and transitively inspected source calls. Go target type
sizes bound integer values. Overflow, conversion wrap, unsupported operators, unknown
values, external memory/global access, recursion, defer/recover, executed panic, unsafe operations,
I/O and synchronization refuse this proof. Entirely unchosen effects do not execute:
for example a literal nil slice proves a range has no iterations. Unknown/nonliteral
arguments cannot borrow that proof.

Aggregate assignment preserves existing element/field addresses; Go value copies
are independent and overlapping `copy` snapshots its source. Only evaluator-owned
storage can be written. Private views and fully inspected helper calls can therefore
pass this rule even when the general private-address-use rule refuses them.
Zero pointer/slice/map fields are permitted only after the complete type graph
excludes synchronization, callbacks, nonempty interfaces and unsafe pointers.
Empty-interface slots are admitted only as nil. The separate immutable reflect.Type
metadata model below is the only nonempty-interface exception. General boxing,
assertion, interface conversion and application method dispatch remain refused;
copies enforce these value-representation rules. Nil fields do not create backing storage. Later pointer assignments must still derive from owned
cells; nil dereferences and external/global addresses refuse. Recursive private data
graphs are allowed, but map construction/mutation is not implemented by this rule.
This proves the actual go/constant.newFloat constructor and its SetPrec call on a
fresh zero big.Float, without modeling general floating-point operations.
It also proves the actual ast.NewIdent constructor with its nil object/interface
storage. SSA zero aggregate constants materialize correctly shaped zero values;
resetting an aggregate preserves existing subobject addresses. General finite-data
proofs still exclude all interfaces; nonliteral nil-producing calls do not become
literal inputs.
The whole result remains abstract in behavioral IR; these facts do not turn a
computed integer into a proved channel capacity or establish payload correctness.

Limits are 100000 evaluation steps, 65536 allocated/cloned cells, 4096 elements per
allocated array, 256 KiB strings, allocation depth 64 and call depth 32. Exhaustion
fails, never truncates execution. Fresh lowering rechecks the call's literal arguments,
body and graph; regular calls also consume retained caller operands. Initializer
calls follow the same rule. `constant-data-call` records name the proved invocation.
The installed base64.NewEncoding source is proved for its two valid standard-library
alphabet literals; invalid length, duplicate and newline mutations are refused.

### Explicit byte-search operation model

A source-less `internal/bytealg.IndexByteString(string, byte) int` declaration may
use the exact first-equal-byte-index/-1 semantics of the standard Go toolchain.
The model checks the typed signature, declaration ownership, absent Go body, source
mapping and location under the running toolchain's GOROOT. It is not an arbitrary
unavailable-body or name-only purity exemption. Current call-graph edges are still
required; a supplied Go body is interpreted normally rather than bypassed.

The evaluator searches concrete bytes, not Unicode code points, and charges each
examined byte to its shared step budget. It does not invoke native search functions.
The computed result participates in subsequent SSA branches and return checks.
`ConstantDataProof.ModeledOperations`, `modeled-data-operation` diagnostics and a
model assumption disclose this dependency. **Assembly correctness is assumed, not
proved**; a modified/nonstandard toolchain is outside this operation contract.

This enables ten literal `go/types.asGoVersion` initializers through the actual
version/strings/gover source, with native-result comparisons. The nonliteral current
version initializer remains refused. Standard-library initialization is not omitted:
a whole program importing strings may still fail on other dependency initializers.

### Immutable reflection type metadata

`reflect.TypeFor[T]` may produce a type-description token, not an application value
or resource. The evaluator checks a concrete type argument, standard-source origin,
current call graph and exact typed SSA/dataflow shapes of the TypeFor/ABI extraction
chain (`abi.TypeFor`, `abi.TypeOf`, `abi.NoEscape`, `reflect.toRType`). Added operations,
changed field selection, pointer arithmetic, casts or missing call edges invalidate
that admission. **Runtime ABI/type metadata correctness remains an assumption**,
not a proof of unsafe memory representation or compiler correctness.

Only this known metadata receiver may use the explicit standard `reflect.Type`
`Elem` and direct `FieldByName` models. Method objects, signatures, source ownership
and absence of conflicting target facts are checked. Elem accepts pointer, array,
slice, map and channel type descriptions; invalid/nil receivers refuse. Direct
field queries calculate Name, PkgPath, Type, Tag, Offset, Index and Anonymous from
current Go types and target sizes. The ordinary Go executable's unexported main-package
field path is normalized to `main` and compared with native execution; plugin build
modes are outside this contract. Missing fields return the zero StructField/false
only without embedded fields; promoted search is refused. A direct field may still
shadow an embedded one. A field query is limited to 4096 fields and charged against
the shared step/cell budgets. Concrete type traversal has 256-node/64-depth limits.

The standard-reflection assumption is recorded separately from the ABI assumption.
No reflect.Value, application method, field value, pointer or synchronization object
is constructed by these models; even a described channel/function type is just
metadata. Results remain abstract after the invocation proof is consumed. Other
reflection initialization is still checked and may refuse whole-program emission.
Native tests compare all 104 actual x/tools AST edge initializers plus direct-field
layout/presence/tag results. Five direct encoding/json TypeFor initializers also pass.

### Builtin output is not data-only computation

The Go builtins `print` and `println` perform implementation-dependent output.
They are not silently dropped as sequential computations: ordinary calls report
`output-effects`, and initializer output is refused. Helper, goroutine and deferred
forms also cannot authorize executable emission. Fresh call checks prevent a cached
sequential-builtin summary from hiding current output. `-trust-call print,println`
cannot override builtin semantics; source functions shadowing those names are distinct.
Argument evaluation (including channel receives) remains represented in diagnostic IR.
A literal-input proof that establishes zero executions of the output still qualifies;
executed output requires an explicit I/O model, which is not yet provided here.

### Basic-scalar string formatting

A source-bound operation model admits `fmt.Sprintf` only with unnamed basic scalar
or nil interface arguments. It checks the current standard declaration/signature,
exact newPrinter/doPrintf/buffer-copy/free wrapper and graph edges. Correctness of
standard formatting, its private pool/runtime and normal resource availability is
an explicit assumption—not a proof of fmt's implementation. Format strings/results
remain abstract; this establishes no finite payload bound and performs no host formatting.

Variadic arguments must be a nil slice or a full view of a nonescaping local array
of at most 64 interface slots. Current SSA uses are rebuilt with a 4096 instruction/
operand budget. Each populated slot has one dominating store; its box contains an
unnamed basic value. Zero slots are nil. Named values (even with no methods), custom
String/Format callbacks, aggregates, pointers, shared/escaped/reused slices, changed
stores, partial slices and exhausted budgets refuse this model. Argument evaluation,
blocking and other effects are retained. Formatting I/O functions are not modeled.

Consumption rechecks the source/graph/private-array proof and every retained
operand/store root, recording the assumption once. Native checks cover scalar
formatting while holding a mutex and a named-value callback that must be refused.
Actual own-source formatting and go/types' version initializer consume the model.
Other package initialization still refuses executable emission; this adds no positive
whole-fmt-import TLC proof or whole-self proof.

### Boxed-literal runtime type metadata

Three standard `reflect.rtypeOf` initializers are admitted through an explicit
immutable ABI metadata model, not general interface evaluation. The exact current
standard wrapper and ABI TypeOf/NoEscape chain, call graph and dominating literal
box must match. Existing literal/type budgets apply; nested interface boxes,
nonliteral values and stale source/graph/SSA facts refuse. Lowering also retains
both the box and its literal operand. Runtime ABI correctness is assumed and
recorded separately; returned metadata remains abstract, with no executable runtime
pointer or application interface value. Native public type metadata comparisons
cover the real `string`, `[]byte` and `uint8` inputs. Other package initialization
and reflection execution remain independently checked and may refuse emission.

### Conditional startup GOMAXPROCS profile

`analyze`, `inspect` and `check` accept `-runtime-procs N` (1..1024; zero/default
means unspecified). This is a conditional target environment: an ordinary
**linux/amd64** Go executable starts with `GOMAXPROCS=N`, which disables automatic
updates under standard-runtime semantics. The flag does not change or infer the
analyzer host's setting, constrain goroutine count, or set TLC's worker count.
Executions checked against the condition must actually use that startup environment.
Runtime implementation correctness is explicitly assumed, not source-proved.

Admission requires the current typed runtime declaration/signature, GOROOT source
ownership and target constants. A complete SSA inventory, including graph-omitted
functions, permits only direct `runtime.GOMAXPROCS(0)` references outside the
standard-runtime boundary. Setters, SetDefaultGOMAXPROCS, API escapes, deferred/
spawned calls and nonliteral/nonzero arguments refuse. The inventory has a one-million
instruction budget; exhaustion refuses. Trusted-call contracts cannot be combined
with this profile. Other initialization, unknown effects, native/unsafe operations
and input/environment queries still require their independent checks.

Consumed calls freshly recheck the environment inventory, current/cached call graph
and retained operands. Direct query results may supply local/global channel capacities;
arithmetic/wrapped capacities remain refused and other data results remain abstract.
Global creation/identity/capacity proofs are still rechecked, including closed globals.
Profile/argument/graph/slice/signature/source mutations cannot reuse old evidence.
The selected value is recorded in `model.json` metadata option `startup.GOMAXPROCS`
and `check` result field `runtimeProcs`, plus a conditional environment assumption.
No Go-specific executable expression is added to the backend-independent IR.

Native startup comparisons cover values 1, 2 and 3. Six semantic TLC and two CLI/TLC
cases cover capacity-sensitive pass/deadlock, local/global/closed state and retained
blocking/error behavior. The real self-input at N=2 consumes both x/tools CPU-limit
initializers, but remains unsupported; this is only one dimension of the full finite
environment needed for self-bootstrap.

### Initially open global channels

A direct package variable `var limit = make(chan T, N)` is supported when N is a
constant in 0..1024 and the allocation has one dominating store into its own global.
The same complete SSA address inventory used for pre-closed globals forbids rebinding
and address escape, even from functions omitted from the call graph. Later identity
resolution rechecks creation, store and exact capacity. A capacity change within the
supported range invalidates an already emitted resource, just as an out-of-range
capacity does; pre-closed resources use the same capacity consistency requirement.

`open-global-init` records the source creation. The model starts with an open channel
and empty buffer. Main/goroutine sends, receives and closes then use ordinary channel
semantics. This does not discharge sends, receives, conditional closes or arbitrary
helper effects during initialization. Dynamic GOMAXPROCS-derived capacities remain
unsupported. The actual x/tools packages, buildutil and loader I/O semaphores use
this rule (capacities 20, 20 and 10); no package allowlist is involved.

### Pre-closed global channels

A narrow package-initialization pattern is supported without omitting its effects:

```go
var ready = make(chan struct{})
func init() { close(ready) }
```

The direct make must have constant capacity 0..1024 and exactly one store into its
own package's global. The compiler-generated package initializer must subsequently
call a source `init` whose single block only loads/closes that global and returns.
The store dominates the call, and every normal path after the store reaches it.
The close helper must have exactly one incoming graph edge. No package names are
special-cased; the actual `context.closedchan` uses this pattern.

All SSA functions, not merely graph-reachable functions, are inspected for global
address uses: only the unique store and ordinary loads are admitted. Rebinding or
address escape even in an unused function refuses this proof. Inventory search stops
with refusal beyond one million instructions. Creation, close consumption and later
identity resolution recheck current source/graph facts. `closed-global-init`
diagnostics identify both source operations. Other initialization effects are still
checked and may independently refuse the whole program.

The saved channel has `initiallyClosed: true` and an empty buffer; TLC starts with
that state. Reads, receive status, select/default, and erroneous sends/closes use
normal channel semantics. Open globals use the separate creation rule above.
Returning channels through helpers, closure factories, conditional/multiple closes, initialization sends and arbitrary initializer
helper bodies are not added by this rule. Runtime close/send on the accepted global
is modeled as a synchronization error, not silently discarded.

### Receive completion status

`_, ok := <-ch` and `case _, ok := <-ch` retain an exact process-local status.
Delivering a value sets `ok=true`, including buffered values drained after close;
a closed-and-empty receive sets `ok=false`. A nil or open-empty receive without a
sender blocks and does not produce a status. Unbuffered rendezvous sets true;
waking on closure sets false. The status is captured atomically at reception, not
recomputed from the channel when a later condition executes.

Direct uses, `!ok`, and `ok == false`/`true != ok` stay exact. Merged values, status
returned through calls, shared-memory propagation and received Boolean payloads
still use the documented conservative predicate abstraction. This is not general
Boolean/dataflow tracking. Close-driven control uses the separate proof below.

### Close-driven receive cycles

```go
func double(input <-chan int, output chan<- int) {
    for value := range input {
        output <- value * 2
    }
    close(output)
}
```

The loop is kept cyclic. Consumers are not rewritten to receive a fixed number of
values; neither closure nor termination is assumed. A supported cyclic SSA component
must contain exactly one comma-ok receive in a dominating header, an exact status
branch whose false result exits that component, and no subcycle bypassing reception.
The channel must be evaluated outside the cycle and retain a proved static identity.
Equivalent explicit `for { _, ok := <-ch; if !ok { break }; ... }` forms can qualify.
Sequential receive loops and value payloads are allowed; nested receive cycles are not.

Inside a repeating component, scalar SSA computations, sends, stable field reads
and direct close/Lock/Unlock are allowed. Allocation, goroutine creation, defer
registration/draining, stores, select, helper/trusted calls, changing resource
identities and WaitGroup counter changes are rejected. These operations may occur
outside the cycle under their existing restrictions. Thus a deferred Done registered
before reception runs after normal completion, but not on a missing-close deadlock.

Finite-state refers to the **modeled** queues, Boolean locks/status, finite locals,
fixed processes and nonrepeating counter updates—not concrete Go heap/payload bounds.
A passing safety/deadlock check does not prove termination, delivery counts, fairness
or starvation freedom. A communication cycle may run forever without deadlocking.
Pure/sequential cycles and status-ignoring receive loops are still refused. Saved IR
receives an independent backend finite-state check, not a trusted frontend annotation.
See [the real pipeline harnesses](COMPONENTS.md#close-driven-pipeline).

### Direct, proved interface calls

```go
type sender interface { Send(chan int) }
type worker struct{}
func (worker) Send(ch chan int) { ch <- 1 }
func main() {
    var s sender = worker{}
    ch := make(chan int)
    go s.Send(ch)
    <-ch
}
```

The SSA interface value must be a direct `MakeInterface` in the calling function,
optionally reached through interface widening (`ChangeInterface`). Its concrete
non-generic named type and exact declared receiver identify the method. The refined
call graph must contain that same target; lowering also consumes the receiver's
retained data dependencies. This is not a guess based on the number of implementers.

Ordinary calls and goroutine entries inspect the real method body. Channel arguments
retain their positions after the concrete receiver is bound; inline synchronization
fields retain their existing object identities. Source-located
`resolved-interface-call` diagnostics record the proof. Data returns remain abstract.

Interface parameters, unproved interface fields/loads, phis, returned interfaces,
type assertions and method-value callbacks are not covered by the local-box proof.
The separate immutable-field proof below covers a restricted field case. Nor are promoted methods,
implicit pointer/value receiver adaptation, generic receivers or explicit nil boxes.
Synchronization-bearing value copies and channel-object escapes still fail their
existing checks. Interface dispatch directly to `sync` / `sync/atomic` primitives
(including `sync.Locker`) is refused, even with a trust flag; direct primitive calls
inside an ordinary supported method keep their existing semantics. General deferred
interface calls and helper calls inside receive cycles remain unsupported.

### Immutable interface and callback fields

An object with an existing static synchronization-bearing identity can initialize a
direct interface/function field once in its allocation frame, then call it through
ordinary methods or named helper functions. Interface initializers must directly
box a concrete value; function initializers must be named source functions or source
closures, optionally with a named function-type conversion. The real target body is
inspected, not summarized as harmless. Resource-bearing receivers and closure captures
are bound in the allocation's invocation, not reinterpreted in the reader's frame.

The proof follows direct pointer parameters only when **all** incoming direct-call
edges resolve to the same SSA allocation. Two invocations of that allocation site
keep distinct runtime bindings. Two different allocation sites passed to the same
reader method are conservatively refused, even if both could be safe.

All syntactic object uses and selected-field addresses are checked. One store must
dominate every allocation-frame read and every object call/spawn. Callees may read,
not initialize or replace the field. Returned/global objects, pointer cells, object
closure captures, copies/resets, address escapes/conversions and phi aliases are
refused. Interface-parameter or returned-value initializers, nil targets, bound
method wrappers and general callback parameter dispatch remain unsupported. Captured
resource cells retain their existing unique-store/dominance requirements. Other
ordinary scalar fields may still be updated; this is not shared-data verification.

The graph records candidate edges only after direct-call refinement; every use
rechecks the complete origin/store proof and edge agreement. Lowering additionally
requires matching allocation-frame bindings and retained dependencies.
`resolved-field-call` diagnostics record accepted dispatches. Origin traversal and
alias-use inspection each have a 1024-step proof budget; exhaustion means unsupported,
never an assumed alias or truncated execution. No trusted call can supply a missing
field-origin proof.

This admits bounded environments for stored writers/callbacks, not the actual
`os/exec`/`context` environment of `boundedLog`. Its opaque returned callback and
external object escapes still require separate component-entry/environment work.
The extracted production `boundedlog.Writer` now has finite bootstrap environments
using these proofs; [BOOTSTRAP.md](BOOTSTRAP.md) records their results and assumptions.
This does not admit the actual process/context lifecycle as a whole.

### Private data construction in initialization helpers

```go
type settings struct { label string; count int }
func defaults() *settings {
    return &settings{label: "ready", count: 2}
}
var config = defaults()
```

Acyclic helpers can initialize fresh heap-allocated scalar/value-struct/fixed-array
data when SSA proves the allocation remains private until return. Field and array
index paths may nest; data loads, returned element/field addresses and pointer boxing
solely for return are admitted. Finite helpers satisfying the data-loop proof above
may also fill private arrays without unrolling the computation.
Scalars include immutable strings. Array elements are recursively checked, so arrays
of pointers, interfaces, channels or synchronization state cannot gain this proof.
Pointer/interface/slice/map/function/channel fields and synchronization/atomic state
do not qualify. Slicing an array creates an unproved alias for this general rule; the separate
literal-input evaluator can prove owned slices and inspected helper calls. Pointer phis, closure
captures, address conversions, publication and even read-only address-taking helper
calls remain outside this deliberately narrow proof.

Every call and store in the helper is still checked. Shared writes, map updates,
and potentially mutating/I/O builtins (`append`, `copy`, `delete`, `clear`, `print`,
`println`) prevent a local-computation summary. Ordinary lowering also refuses
`print`/`println` without an explicit I/O model; non-output sequential builtins retain
their existing treatment. No helper/package name is trusted by
this rule. It proves the body of the installed `errors.New`, but **does not** waive
other initialization or dynamic-call restrictions when importing `errors`.

Returning ordinary data is not returning a proved synchronization identity. Scalar
values/payloads remain abstract and the existing implicit-panic/resource-exhaustion
assumption still applies. This does not admit general heap alias analysis, recursive
constructors or synchronization construction during initialization.

### Unknown predicates require effect analysis

```go
if external.ShouldRetry(req) {
    ch <- req
}
```

The predicate result may be abstract, but that is **not** permission to ignore
unknown effects of `ShouldRetry`. Its body must be supported, or the user must
provide a truthful `-trust-call` contract. An unresolved call without such a
contract is rejected even when its result is discarded. A call that may block,
spawn, mutate shared state, or fail to return must not be declared pure just to
make analysis succeed.

### Initialization order matters

```go
var ch chan int
func() { ch <- 1 }()
ch = make(chan int, 1)
```

The later assignment cannot supply an identity to the earlier nil send. The MVP
rejects this captured-cell pattern with `sync-initialization-order`, rather than
retroactively substituting the new channel. Some safe capture-before-initialization
patterns are also rejected; this is intentional conservative scope restriction.

## Unsupported patterns

| Pattern | Why it is currently rejected |
|---|---|
| Unproved/indexed/nested loop forms; recursion | Only documented integer expansion, finite read-only computations or close-driven receive SCCs discharge their respective proof obligations |
| Dynamic or over-budget spawning/channel topology | Only static sites, including accepted integer-range expansions, have finite identities |
| Mutable/global channel fields, pointer fields and synchronization objects in containers | Only local allocation-frame immutable channel fields and inline Mutex/WaitGroup fields have identity proofs |
| Different-identity phis, changing captures, returned channel topology | Identity cannot be selected safely by current rules |
| Unproved callbacks/interface dispatch | Only exact local boxing or the documented immutable-field proof is admitted; ambiguous fields, parameters and other dynamic sources remain unsupported |
| Unproved `defer`, executed/unproved panic or recover | Normal-return cleanup is modeled; literal-input evaluation may prove a panic path unchosen, but no panic unwinding is modeled |
| RWMutex and other unsupported synchronization APIs | No corresponding implemented semantics |
| WaitGroup reuse or concurrent positive enrollment | Counter-only representation does not model waiter generations |
| Reachable unsafe pointer operations, including inspected helpers/init | Can bypass modeled resource identity/state |
| Unproved application/dependency initialization with concurrency, unknown calls, or cycles | Initializers are checked, not silently dropped; proved direct global channel patterns model their initial states |

Narrow standard-library initializer summaries are recorded in the model. They do
not authorize ordinary atomic APIs, arbitrary standard-library calls, or
third-party initializers. Unreachable functions are not rejected merely for
containing unsupported instructions.

## Assumptions and intended usage

- Implicit sequential runtime panics and resource exhaustion are excluded by an
  explicit supported-domain assumption. Synchronization errors are **not** excluded.
- Normal main return terminates all goroutines. Blocked workers after main exits
  are not automatically reported as a whole-program deadlock.
- Use one main package as the analysis entry. For a library, write an explicit main
  harness that chooses the inputs/resources under test and calls the real component;
  do not rewrite its concurrency away. The result covers that harness, not all users
  of the library.
- Inspect diagnostics and assumptions before interpreting TLC results. Unsupported
  source should be reduced or reported, not bypassed with false trust contracts.

See [verification guidance](VERIFICATION.md) and the
[semantic regression matrix](SEMANTIC_TEST_MATRIX.md) for executable evidence and
known coverage gaps.
