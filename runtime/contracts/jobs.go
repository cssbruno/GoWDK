package contracts

import "context"

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
	entry, ok := registry.job(typeName[J]())
	if !ok {
		return missingHandlerError(Job, typeName[J]())
	}
	if !rolesAllow(entry.roles, role) {
		return roleNotAllowedError(Job, typeName[J](), role)
	}
	handler, ok := entry.handler.(JobHandler[J])
	if !ok {
		return unsupportedHandlerError(Job, typeName[J]())
	}
	return handler(ctx, job)
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
