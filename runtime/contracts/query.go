package contracts

import (
	"context"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
)

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
	contract := typeName[Q]()
	ctx, span := startContractSpan(ctx, string(ObservationExecuteQuery),
		gowdktrace.LaneContract,
		map[string]any{"gowdk.contract.kind": string(Query), "gowdk.contract.type": contract, "gowdk.contract.role": string(role)},
	)
	var spanErr error
	defer func() { finishContractSpan(span, spanErr) }()
	entry, ok := registry.query(contract)
	if !ok {
		spanErr = missingHandlerError(Query, contract)
		return zero, spanErr
	}
	if !roleMayExecute(entry.roles, role) {
		spanErr = roleNotAllowedError(Query, contract, role)
		return zero, spanErr
	}
	handler, ok := entry.handler.(QueryHandler[Q, R])
	if !ok {
		spanErr = unsupportedHandlerError(Query, contract)
		return zero, spanErr
	}
	result, err := handler(ctx, query)
	spanErr = err
	return result, err
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
