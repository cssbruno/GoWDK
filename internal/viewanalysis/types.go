package viewanalysis

// Dependencies records source dependencies visible in the first view subset.
type Dependencies struct {
	Assets          []string
	CSSClasses      []string
	StyleAttributes []string
}

// ComponentIslandUsage records one component call that explicitly selects an
// island runtime.
type ComponentIslandUsage struct {
	Component string
	Mode      string
}

// ComponentCallUsage records one component call and its optional island mode.
type ComponentCallUsage struct {
	Component     string
	Island        string
	ReactiveProps bool
}

// ComponentReference records one component call with source offsets.
type ComponentReference struct {
	Name  string
	Start int
	End   int
}

// ContractReference records one template-local backend contract intent.
type ContractReference struct {
	Kind   ContractReferenceKind
	Name   string
	Method string
	Path   string
	Start  int
	End    int
}

type ContractReferenceKind string

const (
	ContractReferenceCommand ContractReferenceKind = "command"
	ContractReferenceQuery   ContractReferenceKind = "query"
)

// CommandReference records one form-local backend command intent.
type CommandReference struct {
	Command string
	Method  string
	Path    string
	Start   int
	End     int
}

// QueryReference records one template-local backend query intent.
type QueryReference struct {
	Query string
	Start int
	End   int
}

// SubscriptionReference records one query-bounded presentation-event
// subscription intent.
type SubscriptionReference struct {
	Query      string
	QueryStart int
	QueryEnd   int
	Event      string
	EventStart int
	EventEnd   int
}
