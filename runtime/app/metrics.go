package app

import "sync/atomic"

// Metrics records dependency-free runtime counters for a generated app handler.
type Metrics struct {
	requests        atomic.Uint64
	health          atomic.Uint64
	cookieAck       atomic.Uint64
	backend         atomic.Uint64
	api             atomic.Uint64
	action          atomic.Uint64
	ssrExact        atomic.Uint64
	ssrDynamic      atomic.Uint64
	static          atomic.Uint64
	methodNotAllow  atomic.Uint64
	notFound        atomic.Uint64
	csrfUnavailable atomic.Uint64
}

// MetricsSnapshot is a stable point-in-time copy of runtime counters.
type MetricsSnapshot struct {
	Requests        uint64 `json:"requests"`
	Health          uint64 `json:"health"`
	CookieAck       uint64 `json:"cookieAck"`
	Backend         uint64 `json:"backend"`
	API             uint64 `json:"api"`
	Action          uint64 `json:"action"`
	SSRExact        uint64 `json:"ssrExact"`
	SSRDynamic      uint64 `json:"ssrDynamic"`
	Static          uint64 `json:"static"`
	MethodNotAllow  uint64 `json:"methodNotAllow"`
	NotFound        uint64 `json:"notFound"`
	CSRFUnavailable uint64 `json:"csrfUnavailable"`
}

// Snapshot returns a point-in-time copy of all counters.
func (metrics *Metrics) Snapshot() MetricsSnapshot {
	if metrics == nil {
		return MetricsSnapshot{}
	}
	return MetricsSnapshot{
		Requests:        metrics.requests.Load(),
		Health:          metrics.health.Load(),
		CookieAck:       metrics.cookieAck.Load(),
		Backend:         metrics.backend.Load(),
		API:             metrics.api.Load(),
		Action:          metrics.action.Load(),
		SSRExact:        metrics.ssrExact.Load(),
		SSRDynamic:      metrics.ssrDynamic.Load(),
		Static:          metrics.static.Load(),
		MethodNotAllow:  metrics.methodNotAllow.Load(),
		NotFound:        metrics.notFound.Load(),
		CSRFUnavailable: metrics.csrfUnavailable.Load(),
	}
}

func (metrics *Metrics) recordRequest() {
	if metrics != nil {
		metrics.requests.Add(1)
	}
}

func (metrics *Metrics) recordHealth() {
	if metrics != nil {
		metrics.health.Add(1)
	}
}

func (metrics *Metrics) recordCookieAck() {
	if metrics != nil {
		metrics.cookieAck.Add(1)
	}
}

func (metrics *Metrics) recordBackend() {
	if metrics != nil {
		metrics.backend.Add(1)
	}
}

func (metrics *Metrics) recordAPI() {
	if metrics != nil {
		metrics.api.Add(1)
	}
}

func (metrics *Metrics) recordAction() {
	if metrics != nil {
		metrics.action.Add(1)
	}
}

func (metrics *Metrics) recordSSRExact() {
	if metrics != nil {
		metrics.ssrExact.Add(1)
	}
}

func (metrics *Metrics) recordSSRDynamic() {
	if metrics != nil {
		metrics.ssrDynamic.Add(1)
	}
}

func (metrics *Metrics) recordStatic() {
	if metrics != nil {
		metrics.static.Add(1)
	}
}

func (metrics *Metrics) recordMethodNotAllowed() {
	if metrics != nil {
		metrics.methodNotAllow.Add(1)
	}
}

func (metrics *Metrics) recordNotFound() {
	if metrics != nil {
		metrics.notFound.Add(1)
	}
}

func (metrics *Metrics) recordCSRFUnavailable() {
	if metrics != nil {
		metrics.csrfUnavailable.Add(1)
	}
}
