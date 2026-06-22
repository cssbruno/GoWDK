package otel

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestBuildResourceIncludesServiceAndEnvironment(t *testing.T) {
	res := buildResource(config{
		serviceName: "checkout",
		serviceVer:  "1.2.3",
		environment: "production",
		resourceKVs: map[string]string{"team": "payments"},
	})
	set := res.Set()
	want := map[string]string{
		"service.name":           "checkout",
		"service.version":        "1.2.3",
		"deployment.environment": "production",
		"team":                   "payments",
	}
	for key, value := range want {
		got, ok := set.Value(attribute.Key(key))
		if !ok || got.AsString() != value {
			t.Fatalf("resource[%s] = %q (ok=%v), want %q", key, got.AsString(), ok, value)
		}
	}
}

func TestBuildResourceDefaultsServiceName(t *testing.T) {
	got, ok := buildResource(config{}).Set().Value("service.name")
	if !ok || got.AsString() != defaultServiceName {
		t.Fatalf("default service.name = %q (ok=%v), want %q", got.AsString(), ok, defaultServiceName)
	}
}

type stubExporter struct{ err error }

func (exporter stubExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	return exporter.err
}

func (stubExporter) Shutdown(context.Context) error { return nil }

func TestCountingExporterCountsFailures(t *testing.T) {
	before := ExporterFailureCount()

	failing := countingExporter{inner: stubExporter{err: errors.New("boom")}}
	if err := failing.ExportSpans(context.Background(), nil); err == nil {
		t.Fatal("expected the wrapped exporter error to propagate")
	}
	if got := ExporterFailureCount(); got != before+1 {
		t.Fatalf("exporter failure count = %d, want %d", got, before+1)
	}

	healthy := countingExporter{inner: stubExporter{}}
	if err := healthy.ExportSpans(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := ExporterFailureCount(); got != before+1 {
		t.Fatalf("a successful export must not increment failures, got %d", got)
	}
}

func TestExporterAndBatchOptionsReflectConfig(t *testing.T) {
	cfg := config{
		endpoint:           "localhost:4318",
		insecure:           true,
		gzip:               true,
		timeout:            time.Second,
		headers:            map[string]string{"x": "y"},
		retry:              &RetryConfig{Enabled: true},
		maxQueueSize:       100,
		maxExportBatchSize: 10,
		batchTimeout:       time.Second,
		exportTimeout:      time.Second,
	}
	if got := len(exporterOptionsFor(cfg)); got != 6 {
		t.Fatalf("exporter options = %d, want 6", got)
	}
	if got := len(batchOptionsFor(cfg)); got != 4 {
		t.Fatalf("batch options = %d, want 4", got)
	}
	if got := len(exporterOptionsFor(config{})); got != 0 {
		t.Fatalf("empty-config exporter options = %d, want 0", got)
	}
	if got := len(batchOptionsFor(config{})); got != 0 {
		t.Fatalf("empty-config batch options = %d, want 0", got)
	}
}

func TestForceFlushIsSafeOnOwnedAndNilSink(t *testing.T) {
	sink := NewSinkWithProvider(nil) // GOWDK-owned, identity-preserving
	if err := sink.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush returned error: %v", err)
	}
	if err := sink.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
	var nilSink *Sink
	if err := nilSink.ForceFlush(context.Background()); err != nil {
		t.Fatalf("nil ForceFlush returned error: %v", err)
	}
}

func TestNewSinkAcceptsProductionOptions(t *testing.T) {
	sink, err := NewSink(context.Background(),
		WithEndpoint("localhost:4318"),
		WithInsecure(),
		WithGzip(),
		WithServiceName("checkout"),
		WithServiceVersion("1.0.0"),
		WithEnvironment("production"),
		WithResourceAttributes(map[string]string{"team": "payments"}),
		WithMaxQueueSize(2048),
		WithMaxExportBatchSize(512),
		WithBatchTimeout(5*time.Second),
		WithRetry(RetryConfig{Enabled: true, InitialInterval: time.Second, MaxInterval: 5 * time.Second, MaxElapsedTime: 30 * time.Second}),
	)
	if err != nil {
		t.Fatalf("NewSink with production options returned error: %v", err)
	}
	if sink == nil {
		t.Fatal("expected a non-nil sink")
	}
	if err := sink.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush: %v", err)
	}
	if err := sink.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}
