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
		if candidate == role || candidate == RoleAny {
			return true
		}
	}
	return false
}

// roleMayExecute reports whether a concrete caller role is authorized to execute
// a command or query guarded by roles. Unlike rolesAllow — which is permissive
// so roleless event subscribers receive every role's events — this gate fails
// CLOSED: a contract that declares no roles is callable only by trusted
// in-process callers (role == ""), never by the web surface or any other
// concrete role. A contract opts into universal execution by declaring RoleAny.
//
// This is the data-layer authorization boundary the architecture treats as the
// source of truth, so a developer who forgets to declare roles gets a denied
// contract rather than a publicly executable one.
func roleMayExecute(roles []Role, role Role) bool {
	if role == "" {
		return true
	}
	if len(roles) == 0 {
		return false
	}
	for _, candidate := range roles {
		if candidate == role || candidate == RoleAny {
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
