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
	AttributeNames     map[string]struct{}
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

		attrsMatch := f.matchMetricAttributeNames(rm)

		if len(f.MetricNames) == 0 {
			if attrsMatch {
				return true
			}
			continue
		}

		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			metrics := sms.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				if _, ok := f.MetricNames[metrics.At(k).Name()]; ok {
					if attrsMatch {
						return true
					}
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

		attrsMatch := f.matchTraceAttributeNames(rs)

		if len(f.SpanNames) == 0 {
			if attrsMatch {
				return true
			}
			continue
		}

		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				if _, ok := f.SpanNames[spans.At(k).Name()]; ok {
					if attrsMatch {
						return true
					}
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

		attrsMatch := f.matchLogAttributeNames(rl)
		if !attrsMatch {
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

func (f Filter) matchMetricAttributeNames(rm pmetric.ResourceMetrics) bool {
	if len(f.AttributeNames) == 0 {
		return true
	}
	if f.containsAnyKey(rm.Resource().Attributes()) {
		return true
	}

	sms := rm.ScopeMetrics()
	for i := 0; i < sms.Len(); i++ {
		sm := sms.At(i)
		if f.containsAnyKey(sm.Scope().Attributes()) {
			return true
		}
		metrics := sm.Metrics()
		for j := 0; j < metrics.Len(); j++ {
			if f.metricDataPointsContainAnyKey(metrics.At(j)) {
				return true
			}
		}
	}

	return false
}

func (f Filter) metricDataPointsContainAnyKey(metric pmetric.Metric) bool {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			if f.containsAnyKey(dps.At(i).Attributes()) {
				return true
			}
		}
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			if f.containsAnyKey(dps.At(i).Attributes()) {
				return true
			}
		}
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			if f.containsAnyKey(dps.At(i).Attributes()) {
				return true
			}
		}
	case pmetric.MetricTypeSummary:
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			if f.containsAnyKey(dps.At(i).Attributes()) {
				return true
			}
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := metric.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			if f.containsAnyKey(dps.At(i).Attributes()) {
				return true
			}
		}
	}

	return false
}

func (f Filter) matchTraceAttributeNames(rs ptrace.ResourceSpans) bool {
	if len(f.AttributeNames) == 0 {
		return true
	}
	if f.containsAnyKey(rs.Resource().Attributes()) {
		return true
	}

	ilss := rs.ScopeSpans()
	for i := 0; i < ilss.Len(); i++ {
		ss := ilss.At(i)
		if f.containsAnyKey(ss.Scope().Attributes()) {
			return true
		}
		spans := ss.Spans()
		for j := 0; j < spans.Len(); j++ {
			span := spans.At(j)
			if f.containsAnyKey(span.Attributes()) {
				return true
			}
			events := span.Events()
			for k := 0; k < events.Len(); k++ {
				if f.containsAnyKey(events.At(k).Attributes()) {
					return true
				}
			}
		}
	}

	return false
}

func (f Filter) matchLogAttributeNames(rl plog.ResourceLogs) bool {
	if len(f.AttributeNames) == 0 {
		return true
	}
	if f.containsAnyKey(rl.Resource().Attributes()) {
		return true
	}

	sls := rl.ScopeLogs()
	for i := 0; i < sls.Len(); i++ {
		sl := sls.At(i)
		if f.containsAnyKey(sl.Scope().Attributes()) {
			return true
		}
		logs := sl.LogRecords()
		for j := 0; j < logs.Len(); j++ {
			if f.containsAnyKey(logs.At(j).Attributes()) {
				return true
			}
		}
	}

	return false
}

func (f Filter) containsAnyKey(attrs pcommon.Map) bool {
	if len(f.AttributeNames) == 0 {
		return true
	}
	for key := range f.AttributeNames {
		if _, ok := attrs.Get(key); ok {
			return true
		}
	}
	return false
}
