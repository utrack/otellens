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

func TestRegistryPublishMetrics_EmitsOnlyMatchingMetrics(t *testing.T) {
	registry := NewRegistry(10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := registry.Register(ctx, RegisterRequest{
		Filter: Filter{
			Signals:        map[model.SignalType]struct{}{model.SignalMetrics: {}},
			MetricNames:    map[string]struct{}{"http.server.request.duration": {}},
			AttributeNames: map[string]struct{}{"client_name": {}},
		},
		MaxBatches: 1,
		BufferSize: 2,
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	nonTarget := sm.Metrics().AppendEmpty()
	nonTarget.SetName("hnb_product_api_graphql_total")
	nonTargetDP := nonTarget.SetEmptySum().DataPoints().AppendEmpty()
	nonTargetDP.Attributes().PutStr("client_name", "web")
	nonTargetDP.SetIntValue(1)

	target := sm.Metrics().AppendEmpty()
	target.SetName("http.server.request.duration")
	targetDP := target.SetEmptyGauge().DataPoints().AppendEmpty()
	targetDP.Attributes().PutStr("other", "x")
	targetDP.SetDoubleValue(1)

	registry.PublishMetrics(md)

	select {
	case <-session.Events():
		t.Fatal("unexpected event: non-matching metric batch must not be emitted")
	default:
	}

	targetDP.Attributes().PutStr("client_name", "mobile")
	registry.PublishMetrics(md)

	select {
	case event := <-session.Events():
		payload, ok := event.Payload.(*model.MetricsPayload)
		if !ok {
			t.Fatalf("unexpected payload type: %T", event.Payload)
		}
		if payload.MetricCount != 1 {
			t.Fatalf("expected 1 matching metric, got %d", payload.MetricCount)
		}
		if len(payload.Metrics) != 1 {
			t.Fatalf("expected one metric in payload, got %d", len(payload.Metrics))
		}
		if payload.Metrics[0].Name != "http.server.request.duration" {
			t.Fatalf("unexpected metric in payload: %s", payload.Metrics[0].Name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected emitted event for matching metric")
	}
}

func TestRegistryPublishMetrics_VerboseMetricsControlsHistogramBuckets(t *testing.T) {
	testCases := []struct {
		name           string
		verboseMetrics bool
		expectBuckets  bool
	}{
		{name: "default concise metrics", verboseMetrics: false, expectBuckets: false},
		{name: "verbose metrics", verboseMetrics: true, expectBuckets: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			registry := NewRegistry(2)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			session, err := registry.Register(ctx, RegisterRequest{
				Filter: Filter{
					Signals:     map[model.SignalType]struct{}{model.SignalMetrics: {}},
					MetricNames: map[string]struct{}{"hist.metric": {}},
				},
				VerboseMetrics: tc.verboseMetrics,
				MaxBatches:     1,
				BufferSize:     1,
			})
			if err != nil {
				t.Fatalf("register failed: %v", err)
			}

			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			metric := sm.Metrics().AppendEmpty()
			metric.SetName("hist.metric")
			dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
			dp.SetCount(2)
			dp.SetSum(3)
			dp.ExplicitBounds().FromRaw([]float64{1, 2})
			dp.BucketCounts().FromRaw([]uint64{1, 1, 0})

			registry.PublishMetrics(md)

			select {
			case event := <-session.Events():
				payload, ok := event.Payload.(*model.MetricsPayload)
				if !ok {
					t.Fatalf("unexpected payload type: %T", event.Payload)
				}
				if len(payload.Metrics) != 1 || len(payload.Metrics[0].DataPoints) != 1 {
					t.Fatalf("expected one metric with one datapoint, got %+v", payload)
				}
				detail := payload.Metrics[0].DataPoints[0]
				hasBuckets := len(detail.BucketCounts) > 0 || len(detail.ExplicitBounds) > 0
				if hasBuckets != tc.expectBuckets {
					t.Fatalf("bucket fields mismatch: expect=%v got_counts=%v got_bounds=%v", tc.expectBuckets, detail.BucketCounts, detail.ExplicitBounds)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("expected emitted metrics event")
			}
		})
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
