package behavior

// Statistics counts behavioral IR objects, not TLC states or generated actions.
// Counts on unsupported models describe only the partial extraction.
type Statistics struct {
	Processes            int `json:"processes"`
	Channels             int `json:"channels"`
	Mutexes              int `json:"mutexes"`
	WaitGroups           int `json:"waitGroups"`
	Locations            int `json:"locations"`
	Transitions          int `json:"transitions"`
	AbstractedPredicates int `json:"abstractedPredicates"`
}

func (m *Model) Statistics() Statistics {
	s := Statistics{
		Processes: len(m.Processes), Channels: len(m.Channels),
		Mutexes: len(m.Mutexes), WaitGroups: len(m.WaitGroups),
		Transitions: len(m.Transitions), AbstractedPredicates: len(m.AbstractedPredicates),
	}
	for _, p := range m.Processes {
		s.Locations += len(p.Locations)
	}
	return s
}
