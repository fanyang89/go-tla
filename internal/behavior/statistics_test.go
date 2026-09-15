package behavior

import "testing"

func TestStatistics(t *testing.T) {
	m := &Model{}
	if got := m.Statistics(); got != (Statistics{}) {
		t.Fatalf("empty statistics: %+v", got)
	}
	m.Processes = []Process{{Locations: []string{"start", "done"}}, {Locations: []string{"entry", "send", "done"}}}
	m.Channels = make([]Channel, 2)
	m.Mutexes = []string{"mu"}
	m.WaitGroups = []string{"wg"}
	m.Transitions = make([]Transition, 4)
	m.AbstractedPredicates = make([]Predicate, 3)
	want := Statistics{Processes: 2, Channels: 2, Mutexes: 1, WaitGroups: 1, Locations: 5, Transitions: 4, AbstractedPredicates: 3}
	if got := m.Statistics(); got != want {
		t.Fatalf("statistics = %+v, want %+v", got, want)
	}
	m.Transitions = append(m.Transitions, Transition{})
	if m.Statistics().Transitions != 5 {
		t.Fatal("statistics not derived from current model")
	}
}
