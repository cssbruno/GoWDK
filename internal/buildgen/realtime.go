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

func queryInvalidationTypeNames(invalidations []gwdkir.QueryInvalidation) map[string]string {
	if len(invalidations) == 0 {
		return nil
	}
	types := map[string]string{}
	for _, invalidation := range invalidations {
		if invalidation.Status != gwdkir.ContractBindingBound {
			continue
		}
		query := strings.TrimSpace(invalidation.Query)
		queryType := strings.TrimSpace(invalidation.QueryType)
		if query == "" || queryType == "" {
			continue
		}
		types[query] = queryType
	}
	if len(types) == 0 {
		return nil
	}
	return types
}
