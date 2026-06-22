package trace

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const (
	maxSnapshotNameBytes    = 512
	maxStatusMessageBytes   = 1024
	maxSourceFileBytes      = 512
	maxAttributeCount       = 64
	maxAttributeKeyBytes    = 128
	maxAttributeStringBytes = 2048
	maxAttributeArrayLength = 32
	maxEventCount           = 64
	maxEventLevelBytes      = 32
	maxEventMessageBytes    = 1024
	maxSnapshotEncodedBytes = 64 << 10
)

func cloneSnapshot(snapshot Snapshot) Snapshot {
	snapshot = normalizeSnapshotSource(snapshot)
	snapshot.Name = limitString(snapshot.Name, maxSnapshotNameBytes)
	snapshot.Status.Message = limitString(snapshot.Status.Message, maxStatusMessageBytes)
	snapshot.Source.File = limitString(snapshot.Source.File, maxSourceFileBytes)
	snapshot.Source.OwnerKind = limitString(snapshot.Source.OwnerKind, maxAttributeStringBytes)
	snapshot.Source.OwnerID = limitString(snapshot.Source.OwnerID, maxAttributeStringBytes)
	snapshot.Attributes = cloneAttributes(snapshot.Attributes)
	snapshot.Events = cloneEvents(snapshot.Events)
	return snapshot
}

func cloneAttributes(attrs []Attribute) []Attribute {
	if len(attrs) == 0 {
		return nil
	}
	if len(attrs) > maxAttributeCount {
		attrs = attrs[:maxAttributeCount]
	}
	out := make([]Attribute, 0, len(attrs))
	for _, attr := range attrs {
		normalized, ok := normalizeAttribute(attr)
		if !ok {
			continue
		}
		out = append(out, normalized)
	}
	return out
}

func cloneEvents(events []Event) []Event {
	if len(events) == 0 {
		return nil
	}
	if len(events) > maxEventCount {
		events = events[:maxEventCount]
	}
	out := make([]Event, 0, len(events))
	for _, event := range events {
		event.Level = limitString(event.Level, maxEventLevelBytes)
		event.Message = limitString(event.Message, maxEventMessageBytes)
		event.Attributes = cloneAttributes(event.Attributes)
		out = append(out, event)
	}
	return out
}

func normalizeAttribute(attr Attribute) (Attribute, bool) {
	key := limitString(strings.TrimSpace(attr.Key), maxAttributeKeyBytes)
	if key == "" {
		return Attribute{}, false
	}
	value, ok := normalizeAttributeValue(attr.Value)
	if !ok {
		return Attribute{}, false
	}
	return Attribute{Key: key, Value: value}, true
}

func normalizeAttributeValue(value any) (any, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, false
	case string:
		return limitString(typed, maxAttributeStringBytes), true
	case bool:
		return typed, true
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return typed, true
	case uint:
		if typed > math.MaxInt64 {
			return nil, false
		}
		return int64(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int64(typed), true
	case uint64:
		if typed > math.MaxInt64 {
			return nil, false
		}
		return int64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return nil, false
		}
		return typed, true
	case []string:
		out := make([]string, 0, min(len(typed), maxAttributeArrayLength))
		for _, item := range typed[:min(len(typed), maxAttributeArrayLength)] {
			out = append(out, limitString(item, maxAttributeStringBytes))
		}
		return out, true
	case []bool:
		return append([]bool(nil), typed[:min(len(typed), maxAttributeArrayLength)]...), true
	case []int:
		return append([]int(nil), typed[:min(len(typed), maxAttributeArrayLength)]...), true
	case []int64:
		return append([]int64(nil), typed[:min(len(typed), maxAttributeArrayLength)]...), true
	case []float64:
		out := make([]float64, 0, min(len(typed), maxAttributeArrayLength))
		for _, item := range typed[:min(len(typed), maxAttributeArrayLength)] {
			if math.IsNaN(item) || math.IsInf(item, 0) {
				return nil, false
			}
			out = append(out, item)
		}
		return out, true
	case []any:
		return normalizeAnyArray(typed)
	default:
		return nil, false
	}
}

func normalizeAnyArray(values []any) (any, bool) {
	if len(values) > maxAttributeArrayLength {
		values = values[:maxAttributeArrayLength]
	}
	if len(values) == 0 {
		return []string{}, true
	}
	switch values[0].(type) {
	case string:
		out := make([]string, 0, len(values))
		for _, value := range values {
			item, ok := value.(string)
			if !ok {
				return nil, false
			}
			out = append(out, limitString(item, maxAttributeStringBytes))
		}
		return out, true
	case bool:
		out := make([]bool, 0, len(values))
		for _, value := range values {
			item, ok := value.(bool)
			if !ok {
				return nil, false
			}
			out = append(out, item)
		}
		return out, true
	case float64:
		out := make([]float64, 0, len(values))
		for _, value := range values {
			item, ok := value.(float64)
			if !ok || math.IsNaN(item) || math.IsInf(item, 0) {
				return nil, false
			}
			out = append(out, item)
		}
		return out, true
	default:
		return nil, false
	}
}

func validateSnapshot(snapshot Snapshot) error {
	if !snapshot.TraceID.Valid() || !snapshot.SpanID.Valid() {
		return fmt.Errorf("invalid trace payload")
	}
	if len(snapshot.Name) > maxSnapshotNameBytes {
		return fmt.Errorf("trace payload span name is too large")
	}
	if len(snapshot.Status.Message) > maxStatusMessageBytes {
		return fmt.Errorf("trace payload status message is too large")
	}
	if len(snapshot.Attributes) > maxAttributeCount {
		return fmt.Errorf("trace payload has too many attributes")
	}
	for _, attr := range snapshot.Attributes {
		if len(strings.TrimSpace(attr.Key)) == 0 || len(attr.Key) > maxAttributeKeyBytes {
			return fmt.Errorf("trace payload attribute key is invalid")
		}
		if _, ok := normalizeAttributeValue(attr.Value); !ok {
			return fmt.Errorf("trace payload attribute %q has unsupported value", attr.Key)
		}
	}
	if len(snapshot.Events) > maxEventCount {
		return fmt.Errorf("trace payload has too many events")
	}
	for _, event := range snapshot.Events {
		if len(event.Level) > maxEventLevelBytes || len(event.Message) > maxEventMessageBytes {
			return fmt.Errorf("trace payload event is too large")
		}
		if len(event.Attributes) > maxAttributeCount {
			return fmt.Errorf("trace payload event has too many attributes")
		}
		for _, attr := range event.Attributes {
			if len(strings.TrimSpace(attr.Key)) == 0 || len(attr.Key) > maxAttributeKeyBytes {
				return fmt.Errorf("trace payload event attribute key is invalid")
			}
			if _, ok := normalizeAttributeValue(attr.Value); !ok {
				return fmt.Errorf("trace payload event attribute %q has unsupported value", attr.Key)
			}
		}
	}
	encoded, err := json.Marshal(cloneSnapshot(snapshot))
	if err != nil {
		return fmt.Errorf("trace payload cannot be encoded")
	}
	if len(encoded) > maxSnapshotEncodedBytes {
		return fmt.Errorf("trace payload span exceeds byte limit")
	}
	return nil
}

func snapshotEncodedSize(snapshot Snapshot) int {
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return maxSnapshotEncodedBytes + 1
	}
	return len(encoded)
}

func limitString(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	return value[:maxBytes]
}
