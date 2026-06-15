package contracts

import (
	"context"
	"sort"
	"sync"
)

// Registry stores typed contract handlers for one runtime.
type Registry struct {
	mu            sync.RWMutex
	queries       map[string]queryEntry
	commands      map[string]commandEntry
	events        map[eventKey][]eventEntry
	jobs          map[string]jobEntry
	invalidations []QueryInvalidation
}

// NewRegistry creates an empty contract registry.
func NewRegistry() *Registry {
	return &Registry{
		queries:  map[string]queryEntry{},
		commands: map[string]commandEntry{},
		events:   map[eventKey][]eventEntry{},
		jobs:     map[string]jobEntry{},
	}
}

// Contracts returns deterministic metadata for registered contracts.
func (registry *Registry) Contracts() []Metadata {
	return registry.contractsForRole("")
}

// ContractsForRole returns deterministic metadata for contracts available to role.
func (registry *Registry) ContractsForRole(role Role) []Metadata {
	return registry.contractsForRole(role)
}

// Invalidations returns deterministic event-to-query invalidation metadata.
func (registry *Registry) Invalidations() []QueryInvalidation {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	out := append([]QueryInvalidation(nil), registry.invalidations...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].EventCategory != out[j].EventCategory {
			return out[i].EventCategory < out[j].EventCategory
		}
		if out[i].EventType != out[j].EventType {
			return out[i].EventType < out[j].EventType
		}
		return out[i].QueryType < out[j].QueryType
	})
	return out
}

func (registry *Registry) contractsForRole(role Role) []Metadata {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	metadata := make([]Metadata, 0, len(registry.queries)+len(registry.commands)+len(registry.events)+len(registry.jobs))
	for _, entry := range registry.queries {
		// Match Execute*ForRole: command/query metadata is filtered by the
		// fail-closed gate so a roleless contract is never advertised as callable
		// by a concrete role that execution would then deny.
		if roleMayExecute(entry.roles, role) {
			metadata = append(metadata, Metadata{Kind: Query, Type: entry.query, Result: entry.result, Handlers: 1, Roles: copyRoles(entry.roles)})
		}
	}
	for _, entry := range registry.commands {
		if roleMayExecute(entry.roles, role) {
			metadata = append(metadata, Metadata{Kind: Command, Type: entry.command, Result: entry.result, Handlers: 1, Roles: copyRoles(entry.roles)})
		}
	}
	for key, entries := range registry.events {
		allowedEntries := eventEntriesForRole(entries, role)
		if len(allowedEntries) > 0 {
			metadata = append(metadata, Metadata{Kind: Event, EventCategory: key.category, Type: key.event, Handlers: len(allowedEntries), Roles: eventRoles(allowedEntries)})
		}
	}
	for _, entry := range registry.jobs {
		if rolesAllow(entry.roles, role) {
			metadata = append(metadata, Metadata{Kind: Job, Type: entry.job, Handlers: 1, Roles: copyRoles(entry.roles)})
		}
	}
	sort.Slice(metadata, func(i, j int) bool {
		if metadata[i].Kind != metadata[j].Kind {
			return metadata[i].Kind < metadata[j].Kind
		}
		if metadata[i].EventCategory != metadata[j].EventCategory {
			return metadata[i].EventCategory < metadata[j].EventCategory
		}
		return metadata[i].Type < metadata[j].Type
	})
	return metadata
}

type queryEntry struct {
	query   string
	result  string
	handler any
	roles   []Role
}

type commandEntry struct {
	command string
	result  string
	handler any
	roles   []Role
}

type eventKey struct {
	category EventCategory
	event    string
}

type eventEntry struct {
	event    string
	dispatch func(context.Context, any) error
	roles    []Role
}

type jobEntry struct {
	job     string
	handler any
	roles   []Role
}
