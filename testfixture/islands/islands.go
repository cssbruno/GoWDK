package islands

type CounterProps struct {
	Label string
}

type CounterState struct {
	Count int
	Open  bool
}

type OtherState struct {
	Name string
}

type TaggedState struct {
	Count int `json:"count,omitempty"`
}

type TextState struct {
	Query string
}

type User struct {
	Name string
	Open bool
}

type Item struct {
	ID   string
	Name string
	Done bool
}

type NestedState struct {
	User  User
	Items []Item
	Flags []bool
	Count int
}

type FilterState struct {
	Query string
	Items []Item
}

// SessionState carries a secret-resembling field. It exists to exercise the
// persisted-store secret-field warning and should not be persisted in practice.
type SessionState struct {
	Token string
	Open  bool
}

func NewCounterState() CounterState {
	return CounterState{Count: 1, Open: false}
}

func NewSessionState() SessionState {
	return SessionState{}
}

func NewOtherState() OtherState {
	return OtherState{Name: "other"}
}

func NewTaggedState() TaggedState {
	return TaggedState{}
}

func NewTextState() TextState {
	return TextState{Query: "initial"}
}

func NewNestedState() NestedState {
	return NestedState{
		User:  User{Name: "Ada", Open: true},
		Items: []Item{{ID: "first", Name: "first", Done: false}, {ID: "second", Name: "second", Done: true}},
		Flags: []bool{true, false},
		Count: 0,
	}
}

func NewFilterState() FilterState {
	return FilterState{
		Query: "fir",
		Items: []Item{{ID: "first", Name: "First result", Done: false}, {ID: "second", Name: "Second result", Done: false}},
	}
}
