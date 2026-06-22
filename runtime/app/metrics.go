package app

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics records dependency-free runtime counters for a generated app handler.
type Metrics struct {
	requests        atomic.Uint64
	activeRequests  atomic.Int64
	errors          atomic.Uint64
	totalLatencyNS  atomic.Int64
	maxLatencyNS    atomic.Int64
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
	forbidden       atomic.Uint64
	csrfUnavailable atomic.Uint64
	routes          sync.Map
}

// MetricsSnapshot is a stable point-in-time copy of runtime counters.
type MetricsSnapshot struct {
	Requests        uint64                 `json:"requests"`
	ActiveRequests  int64                  `json:"activeRequests"`
	Errors          uint64                 `json:"errors"`
	TotalLatencyNS  int64                  `json:"totalLatencyNs"`
	MaxLatencyNS    int64                  `json:"maxLatencyNs"`
	Health          uint64                 `json:"health"`
	CookieAck       uint64                 `json:"cookieAck"`
	Backend         uint64                 `json:"backend"`
	API             uint64                 `json:"api"`
	Action          uint64                 `json:"action"`
	SSRExact        uint64                 `json:"ssrExact"`
	SSRDynamic      uint64                 `json:"ssrDynamic"`
	Static          uint64                 `json:"static"`
	MethodNotAllow  uint64                 `json:"methodNotAllow"`
	NotFound        uint64                 `json:"notFound"`
	Forbidden       uint64                 `json:"forbidden"`
	CSRFUnavailable uint64                 `json:"csrfUnavailable"`
	Routes          []RouteMetricsSnapshot `json:"routes,omitempty"`
}

// RouteMetricsSnapshot records low-cardinality metrics for one generated
// backend route template or endpoint.
type RouteMetricsSnapshot struct {
	Kind           string `json:"kind"`
	Route          string `json:"route"`
	EndpointID     string `json:"endpointId,omitempty"`
	Requests       uint64 `json:"requests"`
	ActiveRequests int64  `json:"activeRequests"`
	Errors         uint64 `json:"errors"`
	TotalLatencyNS int64  `json:"totalLatencyNs"`
	MaxLatencyNS   int64  `json:"maxLatencyNs"`
}

// Snapshot returns a point-in-time copy of all counters.
func (metrics *Metrics) Snapshot() MetricsSnapshot {
	if metrics == nil {
		return MetricsSnapshot{}
	}
	return MetricsSnapshot{
		Requests:        metrics.requests.Load(),
		ActiveRequests:  metrics.activeRequests.Load(),
		Errors:          metrics.errors.Load(),
		TotalLatencyNS:  metrics.totalLatencyNS.Load(),
		MaxLatencyNS:    metrics.maxLatencyNS.Load(),
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
		Forbidden:       metrics.forbidden.Load(),
		CSRFUnavailable: metrics.csrfUnavailable.Load(),
		Routes:          metrics.routeSnapshots(),
	}
}

func (metrics *Metrics) startRequest() time.Time {
	if metrics == nil {
		return time.Time{}
	}
	metrics.requests.Add(1)
	metrics.activeRequests.Add(1)
	return time.Now()
}

func (metrics *Metrics) finishRequest(start time.Time, status int) {
	if metrics == nil || start.IsZero() {
		return
	}
	metrics.activeRequests.Add(-1)
	metrics.recordLatency(time.Since(start))
	if status >= http.StatusInternalServerError {
		metrics.errors.Add(1)
	}
}

func (metrics *Metrics) recordLatency(duration time.Duration) {
	if metrics == nil || duration < 0 {
		return
	}
	ns := duration.Nanoseconds()
	metrics.totalLatencyNS.Add(ns)
	updateMaxInt64(&metrics.maxLatencyNS, ns)
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

func (metrics *Metrics) recordForbidden() {
	if metrics != nil {
		metrics.forbidden.Add(1)
	}
}

func (metrics *Metrics) recordCSRFUnavailable() {
	if metrics != nil {
		metrics.csrfUnavailable.Add(1)
	}
}

type metricsContextKey struct{}

func contextWithMetrics(ctx context.Context, metrics *Metrics) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if metrics == nil {
		return ctx
	}
	return context.WithValue(ctx, metricsContextKey{}, metrics)
}

func metricsFromContext(ctx context.Context) *Metrics {
	if ctx == nil {
		return nil
	}
	metrics, _ := ctx.Value(metricsContextKey{}).(*Metrics)
	return metrics
}

type routeMetricsKey struct {
	kind       string
	route      string
	endpointID string
}

type routeMetrics struct {
	key            routeMetricsKey
	requests       atomic.Uint64
	activeRequests atomic.Int64
	errors         atomic.Uint64
	totalLatencyNS atomic.Int64
	maxLatencyNS   atomic.Int64
}

func (metrics *Metrics) startRoute(kind string, route string, endpointID string) (*routeMetrics, time.Time) {
	if metrics == nil {
		return nil, time.Time{}
	}
	key := routeMetricsKey{
		kind:       stableMetricLabel(kind, "backend"),
		route:      stableMetricLabel(route, "/"),
		endpointID: strings.TrimSpace(endpointID),
	}
	value, _ := metrics.routes.LoadOrStore(key, &routeMetrics{key: key})
	routeMetric := value.(*routeMetrics)
	routeMetric.requests.Add(1)
	routeMetric.activeRequests.Add(1)
	return routeMetric, time.Now()
}

func (metrics *Metrics) finishRoute(routeMetric *routeMetrics, start time.Time, status int) {
	if routeMetric == nil || start.IsZero() {
		return
	}
	routeMetric.activeRequests.Add(-1)
	duration := time.Since(start)
	if duration < 0 {
		duration = 0
	}
	ns := duration.Nanoseconds()
	routeMetric.totalLatencyNS.Add(ns)
	updateMaxInt64(&routeMetric.maxLatencyNS, ns)
	if status >= http.StatusInternalServerError {
		routeMetric.errors.Add(1)
	}
}

func (metrics *Metrics) routeSnapshots() []RouteMetricsSnapshot {
	var snapshots []RouteMetricsSnapshot
	metrics.routes.Range(func(_, value any) bool {
		routeMetric := value.(*routeMetrics)
		snapshots = append(snapshots, RouteMetricsSnapshot{
			Kind:           routeMetric.key.kind,
			Route:          routeMetric.key.route,
			EndpointID:     routeMetric.key.endpointID,
			Requests:       routeMetric.requests.Load(),
			ActiveRequests: routeMetric.activeRequests.Load(),
			Errors:         routeMetric.errors.Load(),
			TotalLatencyNS: routeMetric.totalLatencyNS.Load(),
			MaxLatencyNS:   routeMetric.maxLatencyNS.Load(),
		})
		return true
	})
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Kind != snapshots[j].Kind {
			return snapshots[i].Kind < snapshots[j].Kind
		}
		if snapshots[i].Route != snapshots[j].Route {
			return snapshots[i].Route < snapshots[j].Route
		}
		return snapshots[i].EndpointID < snapshots[j].EndpointID
	})
	return snapshots
}

func stableMetricLabel(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func updateMaxInt64(target *atomic.Int64, candidate int64) {
	for {
		current := target.Load()
		if candidate <= current {
			return
		}
		if target.CompareAndSwap(current, candidate) {
			return
		}
	}
}
