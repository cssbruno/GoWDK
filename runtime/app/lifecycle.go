package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"sync"
	"syscall"
	"time"
)

// ServiceValueContractRegistry is the ServiceContext.Values key for the
// generated contract registry when a generated app exposes contracts.
const ServiceValueContractRegistry = "gowdk.contractRegistry"

// Service runs alongside the generated web app inside the generated binary.
type Service interface {
	Name() string
	Mount(ServiceContext) error
	Run(context.Context, ServiceContext) error
}

// ServiceContext is shared with lifecycle services during startup and run.
type ServiceContext struct {
	Mux      *http.ServeMux
	Identity Identity
	Values   map[string]any
}

// Application describes a generated app plus its process-level services.
type Application struct {
	Handler  http.Handler
	Mux      *http.ServeMux
	Identity Identity
	Services []Service
	Values   map[string]any
}

// RunOptions controls generated app process lifecycle.
type RunOptions struct {
	ShutdownTimeout time.Duration
}

// ServiceHooks is a small adapter for services that only need a mount hook, a
// run hook, or both.
type ServiceHooks struct {
	ServiceName string
	MountFunc   func(ServiceContext) error
	RunFunc     func(context.Context, ServiceContext) error
}

// DefaultShutdownTimeout is used when RunOptions.ShutdownTimeout is omitted.
const DefaultShutdownTimeout = 10 * time.Second

// Name returns the configured service name or a stable fallback.
func (hooks ServiceHooks) Name() string {
	if hooks.ServiceName != "" {
		return hooks.ServiceName
	}
	return "service"
}

// Mount runs the optional mount hook.
func (hooks ServiceHooks) Mount(context ServiceContext) error {
	if hooks.MountFunc == nil {
		return nil
	}
	return hooks.MountFunc(context)
}

// Run runs the optional run hook.
func (hooks ServiceHooks) Run(ctx context.Context, context ServiceContext) error {
	if hooks.RunFunc == nil {
		return nil
	}
	return hooks.RunFunc(ctx, context)
}

// Run starts services and the configured HTTP server, then coordinates
// cancellation and graceful shutdown.
func Run(ctx context.Context, server *http.Server, application *Application, options RunOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if server == nil {
		return fmt.Errorf("gowdk runtime server is nil")
	}
	if application == nil {
		return fmt.Errorf("gowdk runtime application is nil")
	}
	if application.Mux == nil {
		application.Mux = http.NewServeMux()
	}
	if application.Handler == nil {
		application.Handler = application.Mux
	}
	if server.Handler == nil {
		server.Handler = application.Handler
	}
	if server.Handler == nil {
		return fmt.Errorf("gowdk runtime application handler is nil")
	}
	if options.ShutdownTimeout <= 0 {
		options.ShutdownTimeout = DefaultShutdownTimeout
	}

	signalContext, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	runContext, cancel := context.WithCancel(signalContext)
	defer cancel()

	values := application.Values
	if values == nil {
		values = map[string]any{}
		application.Values = values
	}
	serviceContext := ServiceContext{
		Mux:      application.Mux,
		Identity: application.Identity,
		Values:   values,
	}
	services := nonNilServices(application.Services)
	for _, service := range services {
		if err := service.Mount(serviceContext); err != nil {
			cancel()
			return fmt.Errorf("gowdk service %q mount failed: %w", serviceName(service), err)
		}
	}

	serviceErrors, servicesDone := startServices(runContext, serviceContext, services)
	serverDone := startServer(server)

	var runErr error
	serverDoneConsumed := false
	select {
	case <-signalContext.Done():
	case err := <-serverDone:
		serverDoneConsumed = true
		runErr = err
	case err := <-serviceErrors:
		runErr = err
	}
	cancel()

	shutdownErr := shutdownServer(server, options.ShutdownTimeout)
	if !serverDoneConsumed {
		if err := waitForServer(serverDone, options.ShutdownTimeout); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}
	if err := waitForServices(servicesDone, options.ShutdownTimeout); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}
	if runErr != nil {
		return errors.Join(runErr, shutdownErr)
	}
	return shutdownErr
}

func nonNilServices(services []Service) []Service {
	out := make([]Service, 0, len(services))
	for _, service := range services {
		if service != nil && !isNilServiceValue(service) {
			out = append(out, service)
		}
	}
	return out
}

func isNilServiceValue(service Service) bool {
	value := reflect.ValueOf(service)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func startServices(ctx context.Context, serviceContext ServiceContext, services []Service) (<-chan error, <-chan struct{}) {
	errorsChannel := make(chan error, 1)
	done := make(chan struct{})
	var wait sync.WaitGroup
	for _, service := range services {
		service := service
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := service.Run(ctx, serviceContext); err != nil {
				if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
					return
				}
				select {
				case errorsChannel <- fmt.Errorf("gowdk service %q failed: %w", serviceName(service), err):
				default:
				}
			}
		}()
	}
	go func() {
		wait.Wait()
		close(done)
	}()
	return errorsChannel, done
}

func startServer(server *http.Server) <-chan error {
	done := make(chan error, 1)
	go func() {
		listener, err := inheritedListener()
		if err != nil {
			done <- fmt.Errorf("gowdk server failed: %w", err)
			return
		}
		if listener != nil {
			err = server.Serve(listener)
		} else {
			err = server.ListenAndServe()
		}
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		if err != nil {
			err = fmt.Errorf("gowdk server failed: %w", err)
		}
		done <- err
	}()
	return done
}

func shutdownServer(server *http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("gowdk server shutdown failed: %w", err)
	}
	return nil
}

func waitForServer(done <-chan error, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		return fmt.Errorf("gowdk server shutdown timed out after %s", timeout)
	}
}

func waitForServices(done <-chan struct{}, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return fmt.Errorf("gowdk service shutdown timed out after %s", timeout)
	}
}

func serviceName(service Service) string {
	name := service.Name()
	if name == "" {
		return "service"
	}
	return name
}
