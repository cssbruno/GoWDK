package contracts

import "strings"

const (
	maxAudienceLabels     = 16
	maxAudienceLabelBytes = 128
)

func normalizeAudience(audience []string) []string {
	if len(audience) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, min(len(audience), maxAudienceLabels))
	for _, label := range audience {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		if len(label) > maxAudienceLabelBytes {
			label = label[:maxAudienceLabelBytes]
		}
		if seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, label)
		if len(out) == maxAudienceLabels {
			break
		}
	}
	return out
}

// Audience returns normalized audience labels for event delivery. Empty
// audience means broadcast to every subscriber authorized for the event type.
func (event EventEnvelope) AudienceLabels() []string {
	return normalizeAudience(event.Audience)
}
