package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunMountsServicesBeforeRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mux := http.NewServeMux()
	var mounted atomic.Bool
	service := ServiceHooks{
		ServiceName: "routes",
		MountFunc: func(context ServiceContext) error {
			if context.Mux != mux {
				t.Fatalf("service context mux = %p, want %p", context.Mux, mux)
			}
			context.Mux.HandleFunc("/service", func(http.ResponseWriter, *http.Request) {})
			mounted.Store(true)
			return nil
		},
		RunFunc: func(context.Context, ServiceContext) error {
			if !mounted.Load() {
				t.Fatal("service ran before mount")
			}
			cancel()
			return nil
		},
	}

	err := Run(ctx, testLifecycleServer(mux), &Application{
		Handler:  mux,
		Mux:      mux,
		Identity: Identity{AppID: "clinic", ModuleName: "frontend", InstanceID: "frontend-1"},
		Services: []Service{service},
	}, RunOptions{ShutdownTimeout: time.Second})
	if err != nil {
		t.Fatalf("expected graceful lifecycle run, got %v", err)
	}
}

func TestRunIgnoresNilAndNoOpServices(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mux := http.NewServeMux()
	var typedNil *recordingLifecycleService

	err := Run(ctx, testLifecycleServer(mux), &Application{
		Handler:  mux,
		Mux:      mux,
		Services: []Service{nil, typedNil, ServiceHooks{ServiceName: "mount-only"}},
	}, RunOptions{ShutdownTimeout: time.Second})
	if err != nil {
		t.Fatalf("expected no-op services to shut down cleanly, got %v", err)
	}
}

func TestRunReturnsServiceErrorAndCancels(t *testing.T) {
	mux := http.NewServeMux()
	err := Run(context.Background(), testLifecycleServer(mux), &Application{
		Handler: mux,
		Mux:     mux,
		Services: []Service{ServiceHooks{
			ServiceName: "worker",
			RunFunc: func(context.Context, ServiceContext) error {
				return errors.New("boom")
			},
		}},
	}, RunOptions{ShutdownTimeout: time.Second})

	if err == nil || !strings.Contains(err.Error(), `gowdk service "worker" failed: boom`) {
		t.Fatalf("expected service failure, got %v", err)
	}
}

func TestRunReturnsNilForContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mux := http.NewServeMux()

	err := Run(ctx, testLifecycleServer(mux), &Application{Handler: mux, Mux: mux}, RunOptions{ShutdownTimeout: time.Second})
	if err != nil {
		t.Fatalf("expected context cancellation to be graceful, got %v", err)
	}
}

func TestRunReportsServiceShutdownTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mux := http.NewServeMux()
	neverDone := make(chan struct{})

	err := Run(ctx, testLifecycleServer(mux), &Application{
		Handler: mux,
		Mux:     mux,
		Services: []Service{ServiceHooks{
			ServiceName: "stuck",
			RunFunc: func(context.Context, ServiceContext) error {
				<-neverDone
				return nil
			},
		}},
	}, RunOptions{ShutdownTimeout: 10 * time.Millisecond})

	if err == nil || !strings.Contains(err.Error(), "gowdk service shutdown timed out") {
		t.Fatalf("expected service shutdown timeout, got %v", err)
	}
}

func TestRunReturnsServerListenError(t *testing.T) {
	mux := http.NewServeMux()
	err := Run(context.Background(), &http.Server{Addr: "127.0.0.1:-1", Handler: mux}, &Application{Handler: mux, Mux: mux}, RunOptions{ShutdownTimeout: time.Second})
	if err == nil || !strings.Contains(err.Error(), "gowdk server failed") {
		t.Fatalf("expected server listen error, got %v", err)
	}
}

func testLifecycleServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              "127.0.0.1:0",
		Handler:           handler,
		ReadHeaderTimeout: time.Second,
	}
}

type recordingLifecycleService struct{}

func (service *recordingLifecycleService) Name() string {
	return "recording"
}

func (service *recordingLifecycleService) Mount(ServiceContext) error {
	return nil
}

func (service *recordingLifecycleService) Run(context.Context, ServiceContext) error {
	return nil
}
