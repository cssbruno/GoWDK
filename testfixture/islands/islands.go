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

// StringCountState declares a Count field typed as string. It exists to exercise
// the store/local-state field-type conflict diagnostic against CounterState's
// int Count.
type StringCountState struct {
	Count string
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

// Credentials nests a secret-resembling field. It exercises the nested
// persisted-store secret-field warning and should not be persisted in practice.
type Credentials struct {
	Token string
	Label string
}

// ProfileState carries a nested Credentials value, so persisting it would write
// the nested Token to browser storage even though no top-level field looks like
// a secret.
type ProfileState struct {
	Name    string
	Account Credentials
}

func NewCounterState() CounterState {
	return CounterState{Count: 1, Open: false}
}

func NewStringCountState() StringCountState {
	return StringCountState{Count: ""}
}

func NewSessionState() SessionState {
	return SessionState{}
}

func NewProfileState() ProfileState {
	return ProfileState{}
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
