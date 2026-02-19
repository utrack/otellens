package capture

import (
	"strings"

	"github.com/utrack/otellens/internal/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Filter defines matching conditions for live capture sessions.
type Filter struct {
	Signals            map[model.SignalType]struct{}
	MetricNames        map[string]struct{}
	SpanNames          map[string]struct{}
	LogBodyContains    string
	MinSeverityNumber  plog.SeverityNumber
	ResourceAttributes map[string]string
}

// MatchMetrics checks whether a metrics batch matches this filter.
func (f Filter) MatchMetrics(md pmetric.Metrics) bool {
	if !f.acceptsSignal(model.SignalMetrics) {
		return false
	}

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		if !f.matchResourceAttrs(rm.Resource().Attributes()) {
			continue
		}

		if len(f.MetricNames) == 0 {
			return true
		}

		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			metrics := sms.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				if _, ok := f.MetricNames[metrics.At(k).Name()]; ok {
					return true
				}
			}
		}
	}

	return false
}

// MatchTraces checks whether a traces batch matches this filter.
func (f Filter) MatchTraces(td ptrace.Traces) bool {
	if !f.acceptsSignal(model.SignalTraces) {
		return false
	}

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		if !f.matchResourceAttrs(rs.Resource().Attributes()) {
			continue
		}

		if len(f.SpanNames) == 0 {
			return true
		}

		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				if _, ok := f.SpanNames[spans.At(k).Name()]; ok {
					return true
				}
			}
		}
	}

	return false
}

// MatchLogs checks whether a logs batch matches this filter.
func (f Filter) MatchLogs(ld plog.Logs) bool {
	if !f.acceptsSignal(model.SignalLogs) {
		return false
	}

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		if !f.matchResourceAttrs(rl.Resource().Attributes()) {
			continue
		}

		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			logs := sls.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				record := logs.At(k)
				if f.MinSeverityNumber > 0 && record.SeverityNumber() < f.MinSeverityNumber {
					continue
				}
				if f.LogBodyContains != "" && !strings.Contains(record.Body().AsString(), f.LogBodyContains) {
					continue
				}
				return true
			}
		}
	}

	return false
}

func (f Filter) acceptsSignal(signal model.SignalType) bool {
	if len(f.Signals) == 0 {
		return true
	}
	_, ok := f.Signals[signal]
	return ok
}

func (f Filter) matchResourceAttrs(attrs pcommon.Map) bool {
	if len(f.ResourceAttributes) == 0 {
		return true
	}
	for key, expected := range f.ResourceAttributes {
		value, ok := attrs.Get(key)
		if !ok || value.AsString() != expected {
			return false
		}
	}
	return true
}
