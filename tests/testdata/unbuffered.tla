---- MODULE model ----
EXTENDS Naturals, Integers, Sequences, TLC
\* Generated from a backend-independent Concurrent Behavioral IR.
\* Assumption: Communication-only analysis assumes no implicit sequential runtime panics or resource exhaustion; synchronization failures remain modeled.
\* Assumption: Go main return terminates the whole program, including blocked workers.
\* Assumption: Trusted-call contracts assert total, side-effect-free execution and no synchronization; return values are abstract.
ProcSet == {"main", "worker_goroutine_1"}
ChannelSet == {"main_channel_1"}
MutexSet == {}
WaitGroupSet == {}
LocalSet == {}
VARIABLES pc, queues, closed, locks, wg, local, fault, waiting
vars == <<pc, queues, closed, locks, wg, local, fault, waiting>>
Init ==
    /\ pc = [p \in ProcSet |-> CASE p = "main" -> "main_entry" [] p = "worker_goroutine_1" -> "Dormant"]
    /\ queues = [c \in ChannelSet |-> <<>>]
    /\ closed = [c \in ChannelSet |-> FALSE]
    /\ locks = [m \in MutexSet |-> FALSE]
    /\ wg = [w \in WaitGroupSet |-> 0]
    /\ local = [v \in LocalSet |-> 0]
    /\ fault = FALSE
    /\ waiting = [p \in ProcSet |-> FALSE]
Running == ~fault /\ pc["main"] # "main_Done"
NoSynchronizationErrors == ~fault

\* github.com/fanmi/go-tla/examples/unbuffered.main examples/unbuffered/main.go:6:2
Pre_main_Spawn_L6_C2_1 == Running /\ pc["main"] = "main_entry" /\ TRUE

\* github.com/fanmi/go-tla/examples/unbuffered.main examples/unbuffered/main.go:7:2
Pre_main_Receive_L7_C2_1 == Running /\ pc["main"] = "main_step_1" /\ TRUE

\* github.com/fanmi/go-tla/examples/unbuffered.main examples/unbuffered/main.go:4:6
Pre_main_Finish_1 == Running /\ pc["main"] = "main_step_2" /\ TRUE

\* github.com/fanmi/go-tla/examples/unbuffered.worker examples/unbuffered/main.go:3:31
Pre_worker_goroutine_1_Send_L3_C31_1 == Running /\ pc["worker_goroutine_1"] = "worker_goroutine_1_entry" /\ TRUE

\* github.com/fanmi/go-tla/examples/unbuffered.worker examples/unbuffered/main.go:3:6
Pre_worker_goroutine_1_Finish_1 == Running /\ pc["worker_goroutine_1"] = "worker_goroutine_1_step_1" /\ TRUE
Ready_main_Receive_L7_C2_1 == Pre_main_Receive_L7_C2_1 /\ (closed["main_channel_1"] \/ (waiting["worker_goroutine_1"] /\ Pre_worker_goroutine_1_Send_L3_C31_1))
Ready_worker_goroutine_1_Send_L3_C31_1 == Pre_worker_goroutine_1_Send_L3_C31_1 /\ (closed["main_channel_1"] \/ (waiting["main"] /\ Pre_main_Receive_L7_C2_1))

Step_main_Spawn_L6_C2_1 ==
    /\ (Pre_main_Spawn_L6_C2_1 /\ pc["worker_goroutine_1"] = "Dormant")
    /\ pc' = ([pc EXCEPT !["main"] = "main_step_1", !["worker_goroutine_1"] = "worker_goroutine_1_entry"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Register_main_Receive_L7_C2_1 ==
    /\ Pre_main_Receive_L7_C2_1 /\ ~waiting["main"] /\ ~(Ready_main_Receive_L7_C2_1)
    /\ waiting' = [waiting EXCEPT !["main"] = TRUE]
    /\ UNCHANGED <<pc, queues, closed, locks, wg, local, fault>>

Step_main_Receive_L7_C2_1 ==
    /\ (Pre_main_Receive_L7_C2_1 /\ closed["main_channel_1"])
    /\ pc' = ([pc EXCEPT !["main"] = "main_step_2"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Step_main_Finish_1 ==
    /\ (Pre_main_Finish_1)
    /\ pc' = ([pc EXCEPT !["main"] = "main_Done"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Register_worker_goroutine_1_Send_L3_C31_1 ==
    /\ Pre_worker_goroutine_1_Send_L3_C31_1 /\ ~waiting["worker_goroutine_1"] /\ ~(Ready_worker_goroutine_1_Send_L3_C31_1)
    /\ waiting' = [waiting EXCEPT !["worker_goroutine_1"] = TRUE]
    /\ UNCHANGED <<pc, queues, closed, locks, wg, local, fault>>

Step_worker_goroutine_1_Send_L3_C31_1 ==
    /\ (Pre_worker_goroutine_1_Send_L3_C31_1 /\ closed["main_channel_1"])
    /\ pc' = ([pc EXCEPT !["worker_goroutine_1"] = "worker_goroutine_1_step_1"])
    /\ fault' = (TRUE)
    /\ waiting' = ([waiting EXCEPT !["worker_goroutine_1"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Step_worker_goroutine_1_Finish_1 ==
    /\ (Pre_worker_goroutine_1_Finish_1)
    /\ pc' = ([pc EXCEPT !["worker_goroutine_1"] = "worker_goroutine_1_Done"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["worker_goroutine_1"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Rendezvous_worker_goroutine_1_Send_L3_C31_1_main_Receive_L7_C2_1 ==
    /\ Pre_worker_goroutine_1_Send_L3_C31_1 /\ Pre_main_Receive_L7_C2_1 /\ ~closed["main_channel_1"]
    /\ (waiting["worker_goroutine_1"] \/ waiting["main"])
    /\ pc' = ([pc EXCEPT !["worker_goroutine_1"] = "worker_goroutine_1_step_1", !["main"] = "main_step_2"])
    /\ waiting' = ([waiting EXCEPT !["worker_goroutine_1"] = FALSE, !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local, fault>>

Terminated == pc["main"] = "main_Done" /\ UNCHANGED vars
Next ==
    \/ Step_main_Spawn_L6_C2_1
    \/ Register_main_Receive_L7_C2_1
    \/ Step_main_Receive_L7_C2_1
    \/ Step_main_Finish_1
    \/ Register_worker_goroutine_1_Send_L3_C31_1
    \/ Step_worker_goroutine_1_Send_L3_C31_1
    \/ Step_worker_goroutine_1_Finish_1
    \/ Rendezvous_worker_goroutine_1_Send_L3_C31_1_main_Receive_L7_C2_1
    \/ Terminated
Spec == Init /\ [][Next]_vars
====
