package capture

import (
	"testing"

	"github.com/utrack/otellens/internal/model"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestFilterMatchMetricsByAttributeNameAcrossLevels(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "svc")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().Attributes().PutStr("scope.key", "scope.value")
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("http.server.request.duration")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("http.route", "/v1")
	dp.SetDoubleValue(1)

	f := Filter{
		Signals:        map[model.SignalType]struct{}{model.SignalMetrics: {}},
		MetricNames:    map[string]struct{}{"http.server.request.duration": {}},
		AttributeNames: map[string]struct{}{"http.route": {}},
	}
	if !f.MatchMetrics(md) {
		t.Fatal("expected metrics match by datapoint attribute key")
	}

	f = Filter{
		Signals:        map[model.SignalType]struct{}{model.SignalMetrics: {}},
		AttributeNames: map[string]struct{}{"scope.key": {}},
	}
	if !f.MatchMetrics(md) {
		t.Fatal("expected metrics match by scope attribute key")
	}

	f = Filter{
		Signals:        map[model.SignalType]struct{}{model.SignalMetrics: {}},
		AttributeNames: map[string]struct{}{"missing.key": {}},
	}
	if f.MatchMetrics(md) {
		t.Fatal("expected metrics miss for absent attribute key")
	}
}

func TestFilterMatchTracesByAttributeNameAcrossLevels(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "svc")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().Attributes().PutStr("scope.key", "scope.value")
	span := ss.Spans().AppendEmpty()
	span.SetName("GET /")
	span.Attributes().PutStr("span.key", "span.value")
	event := span.Events().AppendEmpty()
	event.Attributes().PutStr("event.key", "event.value")

	f := Filter{
		Signals:        map[model.SignalType]struct{}{model.SignalTraces: {}},
		SpanNames:      map[string]struct{}{"GET /": {}},
		AttributeNames: map[string]struct{}{"event.key": {}},
	}
	if !f.MatchTraces(td) {
		t.Fatal("expected traces match by event attribute key")
	}

	f = Filter{
		Signals:        map[model.SignalType]struct{}{model.SignalTraces: {}},
		AttributeNames: map[string]struct{}{"scope.key": {}},
	}
	if !f.MatchTraces(td) {
		t.Fatal("expected traces match by scope attribute key")
	}

	f = Filter{
		Signals:        map[model.SignalType]struct{}{model.SignalTraces: {}},
		AttributeNames: map[string]struct{}{"missing.key": {}},
	}
	if f.MatchTraces(td) {
		t.Fatal("expected traces miss for absent attribute key")
	}
}

func TestFilterMatchLogsByAttributeNameAcrossLevels(t *testing.T) {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "svc")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().Attributes().PutStr("scope.key", "scope.value")
	record := sl.LogRecords().AppendEmpty()
	record.SetSeverityNumber(plog.SeverityNumberInfo)
	record.Attributes().PutStr("log.key", "log.value")
	record.Body().SetStr("hello")

	f := Filter{
		Signals:        map[model.SignalType]struct{}{model.SignalLogs: {}},
		AttributeNames: map[string]struct{}{"log.key": {}},
	}
	if !f.MatchLogs(ld) {
		t.Fatal("expected logs match by log attribute key")
	}

	f = Filter{
		Signals:           map[model.SignalType]struct{}{model.SignalLogs: {}},
		AttributeNames:    map[string]struct{}{"missing.key": {}},
		MinSeverityNumber: plog.SeverityNumberInfo,
	}
	if f.MatchLogs(ld) {
		t.Fatal("expected logs miss for absent attribute key")
	}
}
