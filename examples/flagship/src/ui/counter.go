package ui

type CounterState struct {
	Label string
	Count int
}

func NewCounterState() CounterState {
	return CounterState{
		Label: "Local island counter",
		Count: 0,
	}
}
