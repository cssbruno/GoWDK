package app

import (
	"sync"
	"testing"
)

func TestMetricsConcurrentSnapshots(t *testing.T) {
	var metrics Metrics
	const workers = 16
	const iterations = 100

	var ready sync.WaitGroup
	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)

	for worker := 0; worker < workers; worker++ {
		ready.Add(1)
		done.Add(1)
		go func() {
			defer done.Done()
			ready.Done()
			start.Wait()
			for iteration := 0; iteration < iterations; iteration++ {
				requestStart := metrics.startRequest()
				metrics.recordHealth()
				metrics.recordCookieAck()
				metrics.recordBackend()
				metrics.recordAPI()
				metrics.recordAction()
				metrics.recordSSRExact()
				metrics.recordSSRDynamic()
				metrics.recordStatic()
				metrics.recordMethodNotAllowed()
				metrics.recordNotFound()
				metrics.recordForbidden()
				metrics.recordCSRFUnavailable()
				metrics.finishRequest(requestStart, 200)
			}
		}()
	}

	ready.Wait()
	start.Done()
	for i := 0; i < iterations; i++ {
		_ = metrics.Snapshot()
	}
	done.Wait()

	snapshot := metrics.Snapshot()
	if snapshot.Requests != workers*iterations {
		t.Fatalf("requests = %d, want %d", snapshot.Requests, workers*iterations)
	}
	if snapshot.ActiveRequests != 0 || snapshot.TotalLatencyNS <= 0 || snapshot.MaxLatencyNS <= 0 {
		t.Fatalf("unexpected request lifecycle metrics: %#v", snapshot)
	}
	if snapshot.Health != workers*iterations || snapshot.API != workers*iterations || snapshot.CSRFUnavailable != workers*iterations {
		t.Fatalf("unexpected metrics snapshot: %#v", snapshot)
	}
}
