package contracts

import (
	"context"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
)

// RegisterJob registers one background or scheduled job handler.
func RegisterJob[J any](registry *Registry, handler JobHandler[J], roles ...Role) error {
	if handler == nil {
		return nilHandlerError(Job, typeName[J]())
	}
	return registry.registerJob(typeName[J](), handler, roles)
}

// ExecuteJob runs a registered job handler.
func ExecuteJob[J any](ctx context.Context, registry *Registry, job J) error {
	return executeJob(ctx, registry, job, "")
}

// ExecuteJobForRole runs a job handler only when it is available to role.
func ExecuteJobForRole[J any](ctx context.Context, registry *Registry, role Role, job J) error {
	return executeJob(ctx, registry, job, role)
}

func executeJob[J any](ctx context.Context, registry *Registry, job J, role Role) error {
	contract := typeName[J]()
	ctx, span := startContractSpan(ctx, string(ObservationExecuteJob),
		gowdktrace.LaneJob,
		map[string]any{"gowdk.contract.kind": string(Job), "gowdk.contract.type": contract, "gowdk.contract.role": string(role)},
	)
	var spanErr error
	defer func() { finishContractSpan(span, spanErr) }()
	entry, ok := registry.job(contract)
	if !ok {
		spanErr = missingHandlerError(Job, contract)
		return spanErr
	}
	if !rolesAllow(entry.roles, role) {
		spanErr = roleNotAllowedError(Job, contract, role)
		return spanErr
	}
	handler, ok := entry.handler.(JobHandler[J])
	if !ok {
		spanErr = unsupportedHandlerError(Job, contract)
		return spanErr
	}
	spanErr = handler(ctx, job)
	return spanErr
}

func (registry *Registry) registerJob(job string, handler any, roles []Role) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.jobs[job]; exists {
		return duplicateHandlerError(Job, job)
	}
	registry.jobs[job] = jobEntry{job: job, handler: handler, roles: copyRoles(roles)}
	return nil
}

func (registry *Registry) job(job string) (jobEntry, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entry, ok := registry.jobs[job]
	return entry, ok
}
