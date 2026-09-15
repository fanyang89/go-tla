---- MODULE model ----
EXTENDS Naturals, Integers, Sequences, TLC
\* Generated from a backend-independent Concurrent Behavioral IR.
\* Assumption: Communication-only analysis assumes no implicit sequential runtime panics or resource exhaustion; synchronization failures remain modeled.
\* Assumption: Go main return terminates the whole program, including blocked workers.
\* Assumption: Trusted-call contracts assert total, side-effect-free execution and no synchronization; return values are abstract.
ProcSet == {"main", "send_goroutine_17", "send_goroutine_22"}
ChannelSet == {"main_channel_15", "main_channel_16"}
MutexSet == {}
WaitGroupSet == {}
LocalSet == {"main_select_8"}
VARIABLES pc, queues, closed, locks, wg, local, fault
vars == <<pc, queues, closed, locks, wg, local, fault>>
Init ==
    /\ pc = [p \in ProcSet |-> CASE p = "main" -> "main_entry_1" [] p = "send_goroutine_17" -> "Dormant" [] p = "send_goroutine_22" -> "Dormant"]
    /\ queues = [c \in ChannelSet |-> <<>>]
    /\ closed = [c \in ChannelSet |-> FALSE]
    /\ locks = [m \in MutexSet |-> FALSE]
    /\ wg = [w \in WaitGroupSet |-> 0]
    /\ local = [v \in LocalSet |-> CASE v = "main_select_8" -> -1]
    /\ fault = FALSE
Running == ~fault /\ pc["main"] # "main_Done"
NoSynchronizationErrors == ~fault

\* github.com/fanmi/go-tla/examples/select.main main.go:7:2
Pre_main_Spawn_L7_0 == Running /\ pc["main"] = "main_entry_1" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.main main.go:8:2
Pre_main_Spawn_L8_1 == Running /\ pc["main"] = "main_L8_6" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.main main.go:10:7
Pre_main_Receive_L10_2 == Running /\ pc["main"] = "main_L9_7" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.main main.go:11:7
Pre_main_Receive_L11_3 == Running /\ pc["main"] = "main_L9_7" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.main main.go:4:6
Pre_main_Finish_5 == Running /\ pc["main"] = "main_L0_9" /\ local["main_select_8"] = 0

\* github.com/fanmi/go-tla/examples/select.main main.go:4:6
Pre_main_Finish_6 == Running /\ pc["main"] = "main_L0_9" /\ ~(local["main_select_8"] = 0)

\* github.com/fanmi/go-tla/examples/select.send main.go:3:29
Pre_send_goroutine_17_Send_L3_7 == Running /\ pc["send_goroutine_17"] = "send_goroutine_17_entry_18" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.send main.go:7:2
Pre_send_goroutine_17_Finish_8 == Running /\ pc["send_goroutine_17"] = "send_L0_21" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.send main.go:3:29
Pre_send_goroutine_22_Send_L3_9 == Running /\ pc["send_goroutine_22"] = "send_goroutine_22_entry_23" /\ TRUE

\* github.com/fanmi/go-tla/examples/select.send main.go:8:2
Pre_send_goroutine_22_Finish_10 == Running /\ pc["send_goroutine_22"] = "send_L0_26" /\ TRUE
Ready_main_Receive_L10_2 == Pre_main_Receive_L10_2 /\ (closed["main_channel_15"] \/ Pre_send_goroutine_17_Send_L3_7)
Ready_main_Receive_L11_3 == Pre_main_Receive_L11_3 /\ (closed["main_channel_16"] \/ Len(queues["main_channel_16"]) > 0)
Ready_send_goroutine_17_Send_L3_7 == Pre_send_goroutine_17_Send_L3_7 /\ (closed["main_channel_15"] \/ Pre_main_Receive_L10_2)
Ready_send_goroutine_22_Send_L3_9 == Pre_send_goroutine_22_Send_L3_9 /\ (closed["main_channel_16"] \/ Len(queues["main_channel_16"]) < 1)
Pre_main_AssignAbstractState_L9_4 == Running /\ pc["main"] = "main_L9_7" /\ ~(Ready_main_Receive_L10_2 \/ Ready_main_Receive_L11_3)

main_Spawn_L7_0 ==
    /\ (Pre_main_Spawn_L7_0 /\ pc["send_goroutine_17"] = "Dormant")
    /\ pc' = ([pc EXCEPT !["main"] = "main_L8_6", !["send_goroutine_17"] = "send_goroutine_17_entry_18"])
    /\ fault' = (FALSE)
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

main_Spawn_L8_1 ==
    /\ (Pre_main_Spawn_L8_1 /\ pc["send_goroutine_22"] = "Dormant")
    /\ pc' = ([pc EXCEPT !["main"] = "main_L9_7", !["send_goroutine_22"] = "send_goroutine_22_entry_23"])
    /\ fault' = (FALSE)
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

main_Receive_L10_2 ==
    /\ (Pre_main_Receive_L10_2 /\ closed["main_channel_15"])
    /\ pc' = ([pc EXCEPT !["main"] = "main_L0_9"])
    /\ local' = ([local EXCEPT !["main_select_8"] = 0])
    /\ fault' = (FALSE)
    /\ UNCHANGED <<queues, closed, locks, wg>>

main_Receive_L11_3 ==
    /\ (Pre_main_Receive_L11_3 /\ (closed["main_channel_16"] \/ Len(queues["main_channel_16"]) > 0))
    /\ pc' = ([pc EXCEPT !["main"] = "main_L0_9"])
    /\ queues' = (IF Len(queues["main_channel_16"]) > 0 THEN [queues EXCEPT !["main_channel_16"] = Tail(@)] ELSE queues)
    /\ local' = ([local EXCEPT !["main_select_8"] = 1])
    /\ fault' = (FALSE)
    /\ UNCHANGED <<closed, locks, wg>>

main_AssignAbstractState_L9_4 ==
    /\ (Pre_main_AssignAbstractState_L9_4)
    /\ pc' = ([pc EXCEPT !["main"] = "main_L0_9"])
    /\ local' = ([local EXCEPT !["main_select_8"] = -1])
    /\ fault' = (FALSE)
    /\ UNCHANGED <<queues, closed, locks, wg>>

main_Finish_5 ==
    /\ (Pre_main_Finish_5)
    /\ pc' = ([pc EXCEPT !["main"] = "main_Done"])
    /\ fault' = (FALSE)
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

main_Finish_6 ==
    /\ (Pre_main_Finish_6)
    /\ pc' = ([pc EXCEPT !["main"] = "main_Done"])
    /\ fault' = (FALSE)
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

send_goroutine_17_Send_L3_7 ==
    /\ (Pre_send_goroutine_17_Send_L3_7 /\ closed["main_channel_15"])
    /\ pc' = ([pc EXCEPT !["send_goroutine_17"] = "send_L0_21"])
    /\ fault' = (TRUE)
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

send_goroutine_17_Finish_8 ==
    /\ (Pre_send_goroutine_17_Finish_8)
    /\ pc' = ([pc EXCEPT !["send_goroutine_17"] = "send_goroutine_17_Done"])
    /\ fault' = (FALSE)
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

send_goroutine_22_Send_L3_9 ==
    /\ (Pre_send_goroutine_22_Send_L3_9 /\ (closed["main_channel_16"] \/ Len(queues["main_channel_16"]) < 1))
    /\ pc' = ([pc EXCEPT !["send_goroutine_22"] = "send_L0_26"])
    /\ queues' = (IF closed["main_channel_16"] THEN queues ELSE [queues EXCEPT !["main_channel_16"] = Append(@, 0)])
    /\ fault' = (closed["main_channel_16"])
    /\ UNCHANGED <<closed, locks, wg, local>>

send_goroutine_22_Finish_10 ==
    /\ (Pre_send_goroutine_22_Finish_10)
    /\ pc' = ([pc EXCEPT !["send_goroutine_22"] = "send_goroutine_22_Done"])
    /\ fault' = (FALSE)
    /\ UNCHANGED <<queues, closed, locks, wg, local>>

Rendezvous_send_goroutine_17_Send_L3_7_main_Receive_L10_2_7_2 ==
    /\ Pre_send_goroutine_17_Send_L3_7 /\ Pre_main_Receive_L10_2 /\ ~closed["main_channel_15"]
    /\ pc' = ([pc EXCEPT !["send_goroutine_17"] = "send_L0_21", !["main"] = "main_L0_9"])
    /\ local' = ([local EXCEPT !["main_select_8"] = 0])
    /\ UNCHANGED <<queues, closed, locks, wg, fault>>

Terminated == pc["main"] = "main_Done" /\ UNCHANGED vars
Next ==
    \/ main_Spawn_L7_0
    \/ main_Spawn_L8_1
    \/ main_Receive_L10_2
    \/ main_Receive_L11_3
    \/ main_AssignAbstractState_L9_4
    \/ main_Finish_5
    \/ main_Finish_6
    \/ send_goroutine_17_Send_L3_7
    \/ send_goroutine_17_Finish_8
    \/ send_goroutine_22_Send_L3_9
    \/ send_goroutine_22_Finish_10
    \/ Rendezvous_send_goroutine_17_Send_L3_7_main_Receive_L10_2_7_2
    \/ Terminated
Spec == Init /\ [][Next]_vars
====
