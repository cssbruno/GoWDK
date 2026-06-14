package contracts

// ObservationName is a stable operation name for contract logs, metrics, and
// traces. These names are intentionally independent from CLI display text.
type ObservationName string

const (
	ObservationRegisterQuery           ObservationName = "gowdk.contract.register.query"
	ObservationRegisterCommand         ObservationName = "gowdk.contract.register.command"
	ObservationRegisterEvent           ObservationName = "gowdk.contract.register.event"
	ObservationRegisterJob             ObservationName = "gowdk.contract.register.job"
	ObservationExecuteQuery            ObservationName = "gowdk.contract.execute.query"
	ObservationExecuteCommand          ObservationName = "gowdk.contract.execute.command"
	ObservationCaptureCommand          ObservationName = "gowdk.contract.capture.command"
	ObservationExecuteJob              ObservationName = "gowdk.contract.execute.job"
	ObservationPublishEvent            ObservationName = "gowdk.contract.publish.event"
	ObservationStoreCommandEvents      ObservationName = "gowdk.contract.outbox.store"
	ObservationPublishBrokerEvents     ObservationName = "gowdk.contract.broker.publish"
	ObservationSendPresentationEvents  ObservationName = "gowdk.contract.presentation.send"
	ObservationWorkerReceiveEventBatch ObservationName = "gowdk.contract.worker.receive"
	ObservationWorkerAckEventBatch     ObservationName = "gowdk.contract.worker.ack"
	ObservationWorkerNackEventBatch    ObservationName = "gowdk.contract.worker.nack"
	ObservationWorkerDedupSkip         ObservationName = "gowdk.contract.worker.dedup_skip"
)

// ObservationLabels are stable contract attributes for logs, metrics, and
// traces. Empty fields are intentionally omitted by callers that do not need
// them.
type ObservationLabels struct {
	Kind          Kind
	EventCategory EventCategory
	EventID       string
	Contract      string
	Result        string
	Role          Role
	Roles         []Role
	Handlers      int
}

// Observation combines a stable operation name with stable contract labels.
type Observation struct {
	Name   ObservationName
	Labels ObservationLabels
}

// NewObservation creates an observation and copies slice labels so callers can
// safely reuse or mutate their input values.
func NewObservation(name ObservationName, labels ObservationLabels) Observation {
	labels.Roles = copyRoles(labels.Roles)
	return Observation{Name: name, Labels: labels}
}

// ObservationLabels returns stable labels for this registered contract.
func (metadata Metadata) ObservationLabels() ObservationLabels {
	return ObservationLabels{
		Kind:          metadata.Kind,
		EventCategory: metadata.EventCategory,
		Contract:      metadata.Type,
		Result:        metadata.Result,
		Roles:         copyRoles(metadata.Roles),
		Handlers:      metadata.Handlers,
	}
}

// Observation returns a named observation for this registered contract.
func (metadata Metadata) Observation(name ObservationName) Observation {
	return NewObservation(name, metadata.ObservationLabels())
}

// ObservationForRole returns a named observation for this registered contract
// and records the runtime role performing the operation.
func (metadata Metadata) ObservationForRole(name ObservationName, role Role) Observation {
	labels := metadata.ObservationLabels()
	labels.Role = role
	return NewObservation(name, labels)
}

// ObservationLabels returns stable labels for this captured event envelope.
func (event EventEnvelope) ObservationLabels() ObservationLabels {
	return ObservationLabels{
		Kind:          Event,
		EventCategory: event.Category,
		EventID:       event.ID,
		Contract:      event.Type,
	}
}

// Observation returns a named observation for this captured event envelope.
func (event EventEnvelope) Observation(name ObservationName) Observation {
	return NewObservation(name, event.ObservationLabels())
}

// ObservationForRole returns a named observation for this captured event
// envelope and records the runtime role performing the operation.
func (event EventEnvelope) ObservationForRole(name ObservationName, role Role) Observation {
	labels := event.ObservationLabels()
	labels.Role = role
	return NewObservation(name, labels)
}
