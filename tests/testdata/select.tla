---- MODULE model ----
EXTENDS Naturals, Integers, Sequences, TLC
\* Generated from a backend-independent Concurrent Behavioral IR.
\* Assumption: Communication-only analysis assumes no implicit sequential runtime panics or resource exhaustion; synchronization failures remain modeled.
\* Assumption: Go main return terminates the whole program, including blocked workers.
\* Assumption: Trusted-call contracts assert total, side-effect-free execution and no synchronization; return values are abstract.
ProcSet == {"main", "send_goroutine_1", "send_goroutine_2"}
ChannelSet == {"main_channel_1", "main_channel_2"}
MutexSet == {}
WaitGroupSet == {}
LocalSet == {"main_select_1"}
VARIABLES pc, queues, closed, locks, wg, local, fault, waiting
vars == <<pc, queues, closed, locks, wg, local, fault, waiting>>
Init ==
    /\ pc = [p \in ProcSet |-> CASE p = "main" -> "main_entry" [] p = "send_goroutine_1" -> "Dormant" [] p = "send_goroutine_2" -> "Dormant"]
    /\ queues = [c \in ChannelSet |-> <<>>]
    /\ closed = [c \in ChannelSet |-> FALSE]
    /\ locks = [m \in MutexSet |-> FALSE]
    /\ wg = [w \in WaitGroupSet |-> 0]
    /\ local = [v \in LocalSet |-> CASE v = "main_select_1" -> -1]
    /\ fault = FALSE
    /\ waiting = [p \in ProcSet |-> FALSE]
Running == ~fault /\ pc["main"] # "main_Done"
NoSynchronizationErrors == ~fault

\* github.com/fanmi/go-tla/examples/select.main examples/select/main.go:7:2
Pre_main_Spawn_L7_C2_1 == Running /\ pc["main"] = "main_entry" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.main examples/select/main.go:8:2
Pre_main_Spawn_L8_C2_1 == Running /\ pc["main"] = "main_step_1" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.main examples/select/main.go:10:7
Pre_main_Receive_L10_C7_1 == Running /\ pc["main"] = "main_step_2" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.main examples/select/main.go:11:7
Pre_main_Receive_L11_C7_1 == Running /\ pc["main"] = "main_step_2" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.main examples/select/main.go:4:6
Pre_main_Finish_1 == Running /\ pc["main"] = "main_step_3" /\ local["main_select_1"] = 0

\* github.com/fanmi/go-tla/examples/select.main examples/select/main.go:4:6
Pre_main_Finish_2 == Running /\ pc["main"] = "main_step_3" /\ ~(local["main_select_1"] = 0)

\* github.com/fanmi/go-tla/examples/select.send examples/select/main.go:3:29
Pre_send_goroutine_1_Send_L3_C29_1 == Running /\ pc["send_goroutine_1"] = "send_goroutine_1_entry" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.send examples/select/main.go:3:6
Pre_send_goroutine_1_Finish_1 == Running /\ pc["send_goroutine_1"] = "send_goroutine_1_step_1" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.send examples/select/main.go:3:29
Pre_send_goroutine_2_Send_L3_C29_1 == Running /\ pc["send_goroutine_2"] = "send_goroutine_2_entry" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.send examples/select/main.go:3:6
Pre_send_goroutine_2_Finish_1 == Running /\ pc["send_goroutine_2"] = "send_goroutine_2_step_1" /\ TRUE
Ready_main_Receive_L10_C7_1 == Pre_main_Receive_L10_C7_1 /\ (closed["main_channel_1"] \/ (waiting["send_goroutine_1"] /\ Pre_send_goroutine_1_Send_L3_C29_1))
Ready_main_Receive_L11_C7_1 == Pre_main_Receive_L11_C7_1 /\ (closed["main_channel_2"] \/ Len(queues["main_channel_2"]) > 0)
Ready_send_goroutine_1_Send_L3_C29_1 == Pre_send_goroutine_1_Send_L3_C29_1 /\ (closed["main_channel_1"] \/ (waiting["main"] /\ Pre_main_Receive_L10_C7_1))
Ready_send_goroutine_2_Send_L3_C29_1 == Pre_send_goroutine_2_Send_L3_C29_1 /\ (closed["main_channel_2"] \/ Len(queues["main_channel_2"]) < 1)
Pre_main_AssignAbstractState_L9_C2_1 == Running /\ pc["main"] = "main_step_2" /\ ~(Ready_main_Receive_L10_C7_1 \/ Ready_main_Receive_L11_C7_1)

Step_main_Spawn_L7_C2_1 ==
    /\ (Pre_main_Spawn_L7_C2_1 /\ pc["send_goroutine_1"] = "Dormant")
    /\ pc' = ([pc EXCEPT !["main"] = "main_step_1", !["send_goroutine_1"] = "send_goroutine_1_entry"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Step_main_Spawn_L8_C2_1 ==
    /\ (Pre_main_Spawn_L8_C2_1 /\ pc["send_goroutine_2"] = "Dormant")
    /\ pc' = ([pc EXCEPT !["main"] = "main_step_2", !["send_goroutine_2"] = "send_goroutine_2_entry"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Step_main_Receive_L10_C7_1 ==
    /\ (Pre_main_Receive_L10_C7_1 /\ closed["main_channel_1"])
    /\ pc' = ([pc EXCEPT !["main"] = "main_step_3"])
    /\ local' = ([local EXCEPT !["main_select_1"] = 0])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg>>

Step_main_Receive_L11_C7_1 ==
    /\ (Pre_main_Receive_L11_C7_1 /\ (closed["main_channel_2"] \/ Len(queues["main_channel_2"]) > 0))
    /\ pc' = ([pc EXCEPT !["main"] = "main_step_3"])
    /\ queues' = (IF Len(queues["main_channel_2"]) > 0 THEN [queues EXCEPT !["main_channel_2"] = Tail(@)] ELSE queues)
    /\ local' = ([local EXCEPT !["main_select_1"] = 1])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["main"] = FALSE])
    /\ UNCHANGED <<closed, locks, wg>>

Step_main_AssignAbstractState_L9_C2_1 ==
    /\ (Pre_main_AssignAbstractState_L9_C2_1)
    /\ pc' = ([pc EXCEPT !["main"] = "main_step_3"])
    /\ local' = ([local EXCEPT !["main_select_1"] = -1])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg>>

Step_main_Finish_1 ==
    /\ (Pre_main_Finish_1)
    /\ pc' = ([pc EXCEPT !["main"] = "main_Done"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Step_main_Finish_2 ==
    /\ (Pre_main_Finish_2)
    /\ pc' = ([pc EXCEPT !["main"] = "main_Done"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Register_send_goroutine_1_Send_L3_C29_1 ==
    /\ Pre_send_goroutine_1_Send_L3_C29_1 /\ ~waiting["send_goroutine_1"] /\ ~(Ready_send_goroutine_1_Send_L3_C29_1)
    /\ waiting' = [waiting EXCEPT !["send_goroutine_1"] = TRUE]
    /\ UNCHANGED <<pc, queues, closed, locks, wg, local, fault>>

Step_send_goroutine_1_Send_L3_C29_1 ==
    /\ (Pre_send_goroutine_1_Send_L3_C29_1 /\ closed["main_channel_1"])
    /\ pc' = ([pc EXCEPT !["send_goroutine_1"] = "send_goroutine_1_step_1"])
    /\ fault' = (TRUE)
    /\ waiting' = ([waiting EXCEPT !["send_goroutine_1"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Step_send_goroutine_1_Finish_1 ==
    /\ (Pre_send_goroutine_1_Finish_1)
    /\ pc' = ([pc EXCEPT !["send_goroutine_1"] = "send_goroutine_1_Done"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["send_goroutine_1"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Register_send_goroutine_2_Send_L3_C29_1 ==
    /\ Pre_send_goroutine_2_Send_L3_C29_1 /\ ~waiting["send_goroutine_2"] /\ ~(Ready_send_goroutine_2_Send_L3_C29_1)
    /\ waiting' = [waiting EXCEPT !["send_goroutine_2"] = TRUE]
    /\ UNCHANGED <<pc, queues, closed, locks, wg, local, fault>>

Step_send_goroutine_2_Send_L3_C29_1 ==
    /\ (Pre_send_goroutine_2_Send_L3_C29_1 /\ (closed["main_channel_2"] \/ Len(queues["main_channel_2"]) < 1))
    /\ pc' = ([pc EXCEPT !["send_goroutine_2"] = "send_goroutine_2_step_1"])
    /\ queues' = (IF closed["main_channel_2"] THEN queues ELSE [queues EXCEPT !["main_channel_2"] = Append(@, 0)])
    /\ fault' = (closed["main_channel_2"])
    /\ waiting' = ([waiting EXCEPT !["send_goroutine_2"] = FALSE])
    /\ UNCHANGED <<closed, locks, wg, local>>

Step_send_goroutine_2_Finish_1 ==
    /\ (Pre_send_goroutine_2_Finish_1)
    /\ pc' = ([pc EXCEPT !["send_goroutine_2"] = "send_goroutine_2_Done"])
    /\ fault' = (FALSE)
    /\ waiting' = ([waiting EXCEPT !["send_goroutine_2"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Rendezvous_send_goroutine_1_Send_L3_C29_1_main_Receive_L10_C7_1 ==
    /\ Pre_send_goroutine_1_Send_L3_C29_1 /\ Pre_main_Receive_L10_C7_1 /\ ~closed["main_channel_1"]
    /\ (waiting["send_goroutine_1"] \/ waiting["main"])
    /\ pc' = ([pc EXCEPT !["send_goroutine_1"] = "send_goroutine_1_step_1", !["main"] = "main_step_3"])
    /\ local' = ([local EXCEPT !["main_select_1"] = 0])
    /\ waiting' = ([waiting EXCEPT !["send_goroutine_1"] = FALSE, !["main"] = FALSE])
    /\ UNCHANGED <<queues, closed, locks, wg, fault>>

Terminated == pc["main"] = "main_Done" /\ UNCHANGED vars
Next ==
    \/ Step_main_Spawn_L7_C2_1
    \/ Step_main_Spawn_L8_C2_1
    \/ Step_main_Receive_L10_C7_1
    \/ Step_main_Receive_L11_C7_1
    \/ Step_main_AssignAbstractState_L9_C2_1
    \/ Step_main_Finish_1
    \/ Step_main_Finish_2
    \/ Register_send_goroutine_1_Send_L3_C29_1
    \/ Step_send_goroutine_1_Send_L3_C29_1
    \/ Step_send_goroutine_1_Finish_1
    \/ Register_send_goroutine_2_Send_L3_C29_1
    \/ Step_send_goroutine_2_Send_L3_C29_1
    \/ Step_send_goroutine_2_Finish_1
    \/ Rendezvous_send_goroutine_1_Send_L3_C29_1_main_Receive_L10_C7_1
    \/ Terminated
Spec == Init /\ [][Next]_vars
====
