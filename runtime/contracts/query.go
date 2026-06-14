package contracts

import "context"

// RegisterQuery registers one readonly query handler.
func RegisterQuery[Q, R any](registry *Registry, handler QueryHandler[Q, R], roles ...Role) error {
	if handler == nil {
		return nilHandlerError(Query, typeName[Q]())
	}
	return registry.registerQuery(typeName[Q](), typeName[R](), handler, roles)
}

// ExecuteQuery runs a registered query handler.
func ExecuteQuery[Q, R any](ctx context.Context, registry *Registry, query Q) (R, error) {
	return executeQuery[Q, R](ctx, registry, query, "")
}

// ExecuteQueryForRole runs a query handler only when it is available to role.
func ExecuteQueryForRole[Q, R any](ctx context.Context, registry *Registry, role Role, query Q) (R, error) {
	return executeQuery[Q, R](ctx, registry, query, role)
}

func executeQuery[Q, R any](ctx context.Context, registry *Registry, query Q, role Role) (R, error) {
	var zero R
	entry, ok := registry.query(typeName[Q]())
	if !ok {
		return zero, missingHandlerError(Query, typeName[Q]())
	}
	if !rolesAllow(entry.roles, role) {
		return zero, roleNotAllowedError(Query, typeName[Q](), role)
	}
	handler, ok := entry.handler.(QueryHandler[Q, R])
	if !ok {
		return zero, unsupportedHandlerError(Query, typeName[Q]())
	}
	return handler(ctx, query)
}

func (registry *Registry) registerQuery(query, result string, handler any, roles []Role) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.queries[query]; exists {
		return duplicateHandlerError(Query, query)
	}
	registry.queries[query] = queryEntry{query: query, result: result, handler: handler, roles: copyRoles(roles)}
	return nil
}

func (registry *Registry) query(query string) (queryEntry, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entry, ok := registry.queries[query]
	return entry, ok
}
