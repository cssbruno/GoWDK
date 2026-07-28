package trace

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	// DefaultExportQueueSize is the maximum number of completed spans waiting
	// behind the one active sink export.
	DefaultExportQueueSize = 256

	// DefaultExportTimeout is the deadline applied to one sink export.
	DefaultExportTimeout = 5 * time.Second
)

func (tracer *Tracer) enqueueExport(snapshot Snapshot) {
	if tracer == nil || tracer.sink == nil {
		return
	}

	tracer.exportMu.Lock()
	capacity := tracer.exportQueueCapacity
	if capacity <= 0 {
		capacity = DefaultExportQueueSize
		tracer.exportQueueCapacity = capacity
	}
	if len(tracer.exportQueue) >= capacity {
		tracer.exportDropped.Add(1)
		tracer.exportMu.Unlock()
		return
	}
	tracer.exportQueue = append(tracer.exportQueue, snapshot)
	tracer.exportAccepted++
	startWorker := !tracer.exportWorkerRunning
	if startWorker {
		tracer.exportWorkerRunning = true
	}
	tracer.exportMu.Unlock()

	if startWorker {
		go tracer.drainExports()
	}
}

func (tracer *Tracer) drainExports() {
	for {
		snapshot, ok := tracer.nextExport()
		if !ok {
			return
		}
		tracer.exportOne(snapshot)
		tracer.finishExport()
	}
}

func (tracer *Tracer) nextExport() (Snapshot, bool) {
	tracer.exportMu.Lock()
	defer tracer.exportMu.Unlock()
	if len(tracer.exportQueue) == 0 {
		tracer.exportWorkerRunning = false
		tracer.exportInFlight = false
		tracer.notifyExportChangeLocked()
		return Snapshot{}, false
	}
	snapshot := tracer.exportQueue[0]
	tracer.exportQueue[0] = Snapshot{}
	tracer.exportQueue = tracer.exportQueue[1:]
	if len(tracer.exportQueue) == 0 {
		tracer.exportQueue = nil
	}
	tracer.exportInFlight = true
	return snapshot, true
}

func (tracer *Tracer) finishExport() {
	tracer.exportMu.Lock()
	tracer.exportInFlight = false
	tracer.exportCompleted++
	tracer.notifyExportChangeLocked()
	tracer.exportMu.Unlock()
}

func (tracer *Tracer) exportOne(snapshot Snapshot) {
	start := time.Now()
	var (
		exportContext context.Context
		exportErr     error
	)
	defer func() {
		if recovered := recover(); recovered != nil {
			exportErr = fmt.Errorf("panic: %v", recovered)
		}
		timedOut := exportContext != nil && errors.Is(exportContext.Err(), context.DeadlineExceeded)
		if timedOut && exportErr == nil {
			exportErr = context.DeadlineExceeded
		}
		tracer.recordExport(time.Since(start), exportErr, timedOut)
		logSinkFailure(exportErr)
	}()

	timeout := tracer.exportTimeout
	if timeout <= 0 {
		timeout = DefaultExportTimeout
	}
	var cancel context.CancelFunc
	exportContext, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()
	exportErr = tracer.sink.RecordSpan(exportContext, snapshot)
}

// Flush waits until every span accepted by the export queue before the call
// has completed. It does not shut down or flush the configured sink's own
// buffers.
func (tracer *Tracer) Flush(ctx context.Context) error {
	if tracer == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tracer.exportMu.Lock()
	target := tracer.exportAccepted
	for tracer.exportCompleted < target {
		wait := tracer.exportWaitChannelLocked()
		tracer.exportMu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-wait:
		}
		tracer.exportMu.Lock()
	}
	tracer.exportMu.Unlock()
	return nil
}

func (tracer *Tracer) exportWaitChannelLocked() <-chan struct{} {
	if tracer.exportChanged == nil {
		tracer.exportChanged = make(chan struct{})
	}
	return tracer.exportChanged
}

func (tracer *Tracer) notifyExportChangeLocked() {
	if tracer.exportChanged == nil {
		tracer.exportChanged = make(chan struct{})
		return
	}
	close(tracer.exportChanged)
	tracer.exportChanged = make(chan struct{})
}
