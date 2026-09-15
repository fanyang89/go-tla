---- MODULE model ----
EXTENDS Naturals, Integers, Sequences, TLC
\* Generated from a backend-independent Concurrent Behavioral IR.
\* Assumption: Communication-only analysis assumes no implicit sequential runtime panics or resource exhaustion; synchronization failures remain modeled.
\* Assumption: Go main return terminates the whole program, including blocked workers.
\* Assumption: Trusted-call contracts assert total, side-effect-free execution and no synchronization; return values are abstract.
ProcSet == {"main", "worker_goroutine_8"}
ChannelSet == {"main_channel_7"}
MutexSet == {}
WaitGroupSet == {}
LocalSet == {}
VARIABLES pc, queues, closed, locks, wg, local, fault, waiting
vars == <<pc, queues, closed, locks, wg, local, fault, waiting>>
Init ==
    /\ pc = [p \in ProcSet |-> CASE p = "main" -> "main_entry_1" [] p = "worker_goroutine_8" -> "Dormant"]
    /\ queues = [c \in ChannelSet |-> <<>>]
    /\ closed = [c \in ChannelSet |-> FALSE]
    /\ locks = [m \in MutexSet |-> FALSE]
    /\ wg = [w \in WaitGroupSet |-> 0]
    /\ local = [v \in LocalSet |-> 0]
    /\ fault = FALSE
    /\ waiting = [p \in ProcSet |-> FALSE]
Running == ~fault /\ pc["main"] # "main_Done"
NoSynchronizationErrors == ~fault

\* github.com/fanmi/go-tla/examples/unbuffered.main main.go:6:2
Pre_main_Spawn_L6_0 == Running /\ pc["main"] = "main_entry_1" /\ TRUE

\* github.com/fanmi/go-tla/examples/unbuffered.main main.go:7:2
Pre_main_Receive_L7_1 == Running /\ pc["main"] = "main_L7_5" /\ TRUE

\* github.com/fanmi/go-tla/examples/unbuffered.main main.go:4:6
Pre_main_Finish_2 == Running /\ pc["main"] = "main_L0_6" /\ TRUE

\* github.com/fanmi/go-tla/examples/unbuffered.worker main.go:3:31
Pre_worker_goroutine_8_Send_L3_3 == Running /\ pc["worker_goroutine_8"] = "worker_goroutine_8_entry_9" /\ TRUE

\* github.com/fanmi/go-tla/examples/unbuffered.worker main.go:6:2
Pre_worker_goroutine_8_Finish_4 == Running /\ pc["worker_goroutine_8"] = "worker_L0_12" /\ TRUE
Ready_main_Receive_L7_1 == Pre_main_Receive_L7_1 /\ (closed["main_channel_7"] \/ (waiting["worker_goroutine_8"] /\ Pre_worker_goroutine_8_Send_L3_3))
Ready_worker_goroutine_8_Send_L3_3 == Pre_worker_goroutine_8_Send_L3_3 /\ (closed["main_channel_7"] \/ (waiting["main"] /\ Pre_main_Receive_L7_1))

main_Spawn_L6_0 ==
    /\ (Pre_main_Spawn_L6_0 /\ pc["worker_goroutine_8"] = "Dormant")
    /\ pc' = ([pc EXCEPT !["main"] = "main_L7_5", !["worker_goroutine_8"] = "worker_goroutine_8_entry_9"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Register_main_Receive_L7_1 ==
    /\ Pre_main_Receive_L7_1 /\ ~waiting["main"] /\ ~(Ready_main_Receive_L7_1)
    /\ waiting' = [waiting EXCEPT !["main"] = TRUE]
    /\ UNCHANGED <<pc, queues, closed, locks, wg, local, fault>>

main_Receive_L7_1 ==
    /\ (Pre_main_Receive_L7_1 /\ closed["main_channel_7"])
    /\ pc' = ([pc EXCEPT !["main"] = "main_L0_6"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

main_Finish_2 ==
    /\ (Pre_main_Finish_2)
    /\ pc' = ([pc EXCEPT !["main"] = "main_Done"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Register_worker_goroutine_8_Send_L3_3 ==
    /\ Pre_worker_goroutine_8_Send_L3_3 /\ ~waiting["worker_goroutine_8"] /\ ~(Ready_worker_goroutine_8_Send_L3_3)
    /\ waiting' = [waiting EXCEPT !["worker_goroutine_8"] = TRUE]
    /\ UNCHANGED <<pc, queues, closed, locks, wg, local, fault>>

worker_goroutine_8_Send_L3_3 ==
    /\ (Pre_worker_goroutine_8_Send_L3_3 /\ closed["main_channel_7"])
    /\ pc' = ([pc EXCEPT !["worker_goroutine_8"] = "worker_L0_12"])
    /\ fault' = (TRUE)
    /\ waiting' = ([waiting EXCEPT !["worker_goroutine_8"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

worker_goroutine_8_Finish_4 ==
    /\ (Pre_worker_goroutine_8_Finish_4)
    /\ pc' = ([pc EXCEPT !["worker_goroutine_8"] = "worker_goroutine_8_Done"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["worker_goroutine_8"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Rendezvous_worker_goroutine_8_Send_L3_3_main_Receive_L7_1_3_1 ==
    /\ Pre_worker_goroutine_8_Send_L3_3 /\ Pre_main_Receive_L7_1 /\ ~closed["main_channel_7"]
    /\ (waiting["worker_goroutine_8"] \/ waiting["main"])
    /\ pc' = ([pc EXCEPT !["worker_goroutine_8"] = "worker_L0_12", !["main"] = "main_L0_6"])
    /\ waiting' = ([waiting EXCEPT !["worker_goroutine_8"] = FALSE, !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local, fault>>

Terminated == pc["main"] = "main_Done" /\ UNCHANGED vars
Next ==
    \/ main_Spawn_L6_0
    \/ Register_main_Receive_L7_1
    \/ main_Receive_L7_1
    \/ main_Finish_2
    \/ Register_worker_goroutine_8_Send_L3_3
    \/ worker_goroutine_8_Send_L3_3
    \/ worker_goroutine_8_Finish_4
    \/ Rendezvous_worker_goroutine_8_Send_L3_3_main_Receive_L7_1_3_1
    \/ Terminated
Spec == Init /\ [][Next]_vars
====
