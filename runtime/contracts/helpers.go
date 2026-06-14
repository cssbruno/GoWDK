package contracts

import (
	"reflect"
	"sort"
)

// ContractName returns the stable contract name used by registry metadata and
// event envelopes for T.
func ContractName[T any]() string {
	return typeName[T]()
}

func typeName[T any]() string {
	t := reflect.TypeOf((*T)(nil)).Elem()
	if t.PkgPath() == "" || t.Name() == "" {
		return t.String()
	}
	return t.PkgPath() + "." + t.Name()
}

func copyRoles(roles []Role) []Role {
	if len(roles) == 0 {
		return nil
	}
	copied := make([]Role, len(roles))
	copy(copied, roles)
	return copied
}

func rolesAllow(roles []Role, role Role) bool {
	if role == "" || len(roles) == 0 {
		return true
	}
	for _, candidate := range roles {
		if candidate == role {
			return true
		}
	}
	return false
}

func eventEntriesForRole(entries []eventEntry, role Role) []eventEntry {
	if role == "" {
		copied := make([]eventEntry, len(entries))
		copy(copied, entries)
		return copied
	}
	var allowed []eventEntry
	for _, entry := range entries {
		if rolesAllow(entry.roles, role) {
			allowed = append(allowed, entry)
		}
	}
	return allowed
}

func eventsForCategory(events []EventEnvelope, category EventCategory) []EventEnvelope {
	var filtered []EventEnvelope
	for _, event := range events {
		if event.Category == category {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func eventRoles(entries []eventEntry) []Role {
	seen := map[Role]bool{}
	var roles []Role
	for _, entry := range entries {
		for _, role := range entry.roles {
			if !seen[role] {
				seen[role] = true
				roles = append(roles, role)
			}
		}
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i] < roles[j] })
	return roles
}
