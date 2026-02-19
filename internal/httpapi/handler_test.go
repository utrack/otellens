package httpapi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/utrack/otellens/internal/capture"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestHandleStreamValidatesRequest(t *testing.T) {
	h := NewHandler(capture.NewRegistry(4), zap.NewNop())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := bytes.NewBufferString(`{"max_batches":0}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/capture/stream", body)
	res := httptest.NewRecorder()

	mux.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}

	var out StreamError
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestHealthz(t *testing.T) {
	h := NewHandler(capture.NewRegistry(4), zap.NewNop())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

func TestHandleStreamStreamsMatchingMetricsAndEndsSession(t *testing.T) {
	registry := capture.NewRegistry(4)
	h := NewHandler(registry, zap.NewNop())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	server := httptest.NewServer(mux)
	defer server.Close()

	resultCh := make(chan []map[string]any, 1)
	errCh := make(chan error, 1)

	go func() {
		body := bytes.NewBufferString(`{"signals":["metrics"],"metric_names":["A"],"max_batches":1,"timeout_seconds":5}`)
		req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/capture/stream", body)
		if err != nil {
			errCh <- err
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			errCh <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		lines := make([]map[string]any, 0, 2)
		for scanner.Scan() {
			var item map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
				errCh <- err
				return
			}
			lines = append(lines, item)
			if len(lines) >= 2 {
				break
			}
		}
		if err := scanner.Err(); err != nil {
			errCh <- err
			return
		}
		resultCh <- lines
	}()

	deadline := time.Now().Add(2 * time.Second)
	for !registry.HasActiveSessions() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !registry.HasActiveSessions() {
		t.Fatal("expected an active capture session")
	}

	registry.PublishMetrics(newMetricsBatch("A"))

	select {
	case err := <-errCh:
		t.Fatalf("stream request failed: %v", err)
	case lines := <-resultCh:
		if len(lines) != 2 {
			t.Fatalf("expected 2 ndjson lines (event + end), got %d", len(lines))
		}
		if got := lines[0]["signal"]; got != "metrics" {
			t.Fatalf("expected first line to be metrics envelope, got %v", got)
		}
		payload, ok := lines[0]["payload"].(map[string]any)
		if !ok {
			t.Fatalf("expected payload object, got %T", lines[0]["payload"])
		}
		metrics, ok := payload["metrics"].([]any)
		if !ok || len(metrics) == 0 {
			t.Fatalf("expected detailed metrics list in payload, got %T", payload["metrics"])
		}
		firstMetric, ok := metrics[0].(map[string]any)
		if !ok {
			t.Fatalf("expected first metric object, got %T", metrics[0])
		}
		if got := firstMetric["name"]; got != "A" {
			t.Fatalf("expected metric name A, got %v", got)
		}
		if _, ok := firstMetric["resource_attributes"].(map[string]any); !ok {
			t.Fatalf("expected resource_attributes map, got %T", firstMetric["resource_attributes"])
		}
		scope, ok := firstMetric["scope"].(map[string]any)
		if !ok {
			t.Fatalf("expected scope object, got %T", firstMetric["scope"])
		}
		if _, ok := scope["attributes"].(map[string]any); !ok {
			t.Fatalf("expected scope.attributes map, got %T", scope["attributes"])
		}
		dps, ok := firstMetric["data_points"].([]any)
		if !ok || len(dps) == 0 {
			t.Fatalf("expected datapoints list, got %T", firstMetric["data_points"])
		}
		dp0, ok := dps[0].(map[string]any)
		if !ok {
			t.Fatalf("expected datapoint object, got %T", dps[0])
		}
		if _, ok := dp0["attributes"].(map[string]any); !ok {
			t.Fatalf("expected datapoint attributes map, got %T", dp0["attributes"])
		}
		if got := lines[1]["type"]; got != "end" {
			t.Fatalf("expected second line to be end event, got %v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for streamed response")
	}
}

func newMetricsBatch(name string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "otellens-test")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test.scope")
	sm.Scope().Attributes().PutStr("scope.key", "scope.value")
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(name)
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.Attributes().PutStr("http.method", "GET")
	dp.SetDoubleValue(1)
	return md
}
