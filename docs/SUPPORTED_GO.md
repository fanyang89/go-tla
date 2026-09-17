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

## Supported patterns and limits

| Go pattern | Current treatment |
|---|---|
| One main package, direct nonrecursive helper calls | Supported bodies are lowered in the caller's process |
| `go worker(ch)` / statically bound closure | One statically known process per reachable spawn context |
| `for range N` / `for _ = range N` | Main-module constant integer range; no live index, nested loops or branch statements; at most 16 iterations |
| `make(chan T)` / constant capacity | Supported; capacity must be in 0..1024 |
| Send / receive / close | Supported, including nil blocking, closed-channel receive and synchronization errors |
| `select`, including default | Ready cases are nondeterministic; default is unavailable when a communication is ready |
| Channel parameters / direction conversions | Supported when identity is statically resolved |
| Captured single-assignment channel cell | Initialization must dominate every load and capture; no later mutation |
| Local or direct global Mutex / WaitGroup | Stable identities; passing/copying these objects by value is rejected |
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
to contain a small value. No iteration variable is currently supported except `_`.
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
callbacks or unsafe pointers. Private plain-data copies/initialization retain the
existing complete-address-use proof, including fixed-array elements; writes to
caller/global data, mutation through unproved container aliases,
printing, unknown/unavailable calls, recursion, defers, panic and synchronization
cannot acquire this summary. User trust flags cannot supply missing proof facts.
The existing implicit-panic/resource-exhaustion domain assumption still applies.

Cached call summaries are rechecked against the current graph and body; lowering also
consumes the retained caller operands. Argument evaluation is not erased. Initialization
helpers use the same proof, not a package allowlist. Proof search is capped at 4096
function/instruction steps, with type-graph budgets of 256 nodes and depth 64;
exhaustion refuses the proof. `finite-data-loop` diagnostics identify accepted source
functions. A proof failure does not turn an otherwise unsupported loop into a model.

This is a termination/effect proof for data computation, not a payload, memory-race,
whole-program termination or analyzer-correctness theorem.

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
normal channel semantics. Returning channels through helpers, open globals, closure
factories, conditional/multiple closes, initialization sends and arbitrary initializer
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
do not qualify. Slicing an array creates an unproved alias and remains refused. Pointer phis, closure
captures, address conversions, publication and even read-only address-taking helper
calls remain outside this deliberately narrow proof.

Every call and store in the helper is still checked. Shared writes, map updates,
and potentially mutating/I/O builtins (`append`, `copy`, `delete`, `clear`, `print`,
`println`) prevent a local-computation summary; existing ordinary communication
lowering of sequential builtins is unchanged. No helper/package name is trusted by
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
| Unproved `defer`, explicit panic/recover | Normal-return Unlock/Done and static source helpers are modeled; no dynamic targets or panic unwinding |
| RWMutex and other unsupported synchronization APIs | No corresponding implemented semantics |
| WaitGroup reuse or concurrent positive enrollment | Counter-only representation does not model waiter generations |
| Reachable unsafe pointer operations, including inspected helpers/init | Can bypass modeled resource identity/state |
| Unproved application/dependency initialization with concurrency, unknown calls, or cycles | Initializers are checked, not silently dropped; the pre-closed global pattern models its proved initial state |

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
