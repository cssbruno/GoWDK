package buildgen

import (
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func realtimeSubscriptionEventTypeNames(subscriptions []gwdkir.RealtimeSubscription) map[string]string {
	if len(subscriptions) == 0 {
		return nil
	}
	types := map[string]string{}
	for _, subscription := range subscriptions {
		if subscription.Status != gwdkir.ContractBindingBound {
			continue
		}
		event := strings.TrimSpace(subscription.Event)
		eventType := realtimeSubscriptionEventType(subscription)
		if event == "" || eventType == "" {
			continue
		}
		types[event] = eventType
	}
	if len(types) == 0 {
		return nil
	}
	return types
}

func realtimeSubscriptionEventType(subscription gwdkir.RealtimeSubscription) string {
	eventType := strings.TrimSpace(subscription.EventType)
	if eventType == "" {
		return ""
	}
	importPath := strings.TrimSpace(subscription.EventImportPath)
	if importPath == "" {
		return eventType
	}
	return importPath + "." + eventType
}
