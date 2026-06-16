package contracts

import (
	"context"
	"sort"
)

// RegisterInvalidation records that a domain event type invalidates a query
// type. The compiler scans this metadata to generate browser query refresh
// wiring; runtime callers can also inspect it through Registry.Invalidations.
func RegisterInvalidation[E, Q any](registry *Registry) error {
	if registry == nil {
		return Error{Kind: ErrNilHandler, Message: "contract invalidation registry cannot be nil"}
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	invalidation := QueryInvalidation{
		EventCategory: DomainEvent,
		EventType:     typeName[E](),
		QueryType:     typeName[Q](),
	}
	for _, existing := range registry.invalidations {
		if existing == invalidation {
			return nil
		}
	}
	registry.invalidations = append(registry.invalidations, invalidation)
	return nil
}

// QueryInvalidationCommandEventSink sends one generated presentation event for
// query types invalidated by captured command events. Fanout errors are ignored
// so invalidation delivery never decides command success.
func QueryInvalidationCommandEventSink(fanout PresentationFanout, invalidations []QueryInvalidation) CommandEventSink {
	copied := append([]QueryInvalidation(nil), invalidations...)
	return commandEventSinkFunc(func(ctx context.Context, registry *Registry, role Role, events []EventEnvelope) error {
		if fanout == nil || len(events) == 0 || len(copied) == 0 {
			return nil
		}
		queries, sourceEvents := invalidatedQueries(copied, events)
		if len(queries) == 0 {
			return nil
		}
		_ = fanout.SendPresentationEvents(ctx, []EventEnvelope{{
			Category: PresentationEvent,
			Type:     QueryInvalidationPresentationEventType,
			Value: QueryInvalidationNotice{
				Queries: queries,
				Events:  sourceEvents,
			},
		}})
		return nil
	})
}

// InvalidatedQueryTypes returns the query types invalidated by the given command
// events under the registered invalidation edges. The generated command adapter
// uses it to tell the submitting client exactly which g:query regions to refresh
// (the single-flight write path), independent of realtime fanout to other
// clients. It returns nil when nothing is invalidated.
func InvalidatedQueryTypes(invalidations []QueryInvalidation, events []EventEnvelope) []string {
	queries, _ := invalidatedQueries(invalidations, events)
	return queries
}

func invalidatedQueries(invalidations []QueryInvalidation, events []EventEnvelope) ([]string, []string) {
	queries := map[string]bool{}
	sourceEvents := map[string]bool{}
	for _, event := range events {
		for _, invalidation := range invalidations {
			if invalidation.EventCategory != event.Category || invalidation.EventType != event.Type || invalidation.QueryType == "" {
				continue
			}
			queries[invalidation.QueryType] = true
			sourceEvents[string(event.Category)+":"+event.Type] = true
		}
	}
	return sortedKeys(queries), sortedKeys(sourceEvents)
}

func sortedKeys(values map[string]bool) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
