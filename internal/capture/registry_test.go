package capture

import (
	"context"
	"testing"
	"time"

	"github.com/utrack/otellens/internal/model"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestRegistryHasActiveSessions(t *testing.T) {
	registry := NewRegistry(10)
	if registry.HasActiveSessions() {
		t.Fatal("expected no active sessions")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := registry.Register(ctx, RegisterRequest{
		Filter:     Filter{Signals: map[model.SignalType]struct{}{model.SignalMetrics: {}}},
		MaxBatches: 2,
		BufferSize: 2,
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if !registry.HasActiveSessions() {
		t.Fatal("expected active session")
	}

	registry.Deregister(session.ID())
	if registry.HasActiveSessions() {
		t.Fatal("expected no active sessions after deregister")
	}
}

func TestRegistryPublishesAndAutoDeregistersAtBatchLimit(t *testing.T) {
	registry := NewRegistry(10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := registry.Register(ctx, RegisterRequest{
		Filter: Filter{
			Signals:     map[model.SignalType]struct{}{model.SignalMetrics: {}},
			MetricNames: map[string]struct{}{"A": {}},
		},
		MaxBatches: 2,
		BufferSize: 2,
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	batch := newMetricsBatch("A")
	registry.PublishMetrics(batch)
	registry.PublishMetrics(batch)

	received := 0
	for range session.Events() {
		received++
	}
	if received != 2 {
		t.Fatalf("expected 2 events, got %d", received)
	}

	if registry.HasActiveSessions() {
		t.Fatal("expected auto-deregister after max batches")
	}
}

func TestRegistryFastDropPath(t *testing.T) {
	registry := NewRegistry(1)
	if registry.HasActiveSessions() {
		t.Fatal("unexpected active sessions")
	}

	batch := newMetricsBatch("A")
	for i := 0; i < 1000; i++ {
		registry.PublishMetrics(batch)
	}
}

func newMetricsBatch(name string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(name)
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(1)
	return md
}
