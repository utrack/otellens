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
	Signals             map[model.SignalType]struct{}
	MetricNames         map[string]struct{}
	MetricNamesExclude  map[string]struct{}
	SpanNames           map[string]struct{}
	SpanNamesExclude    map[string]struct{}
	AttributeNames      map[string]struct{}
	AttributeExclude    map[string]struct{}
	BucketCountsCount   *int
	ExplicitBoundsCount *int
	LogBodyContains     string
	MinSeverityNumber   plog.SeverityNumber
	ResourceAttributes  map[string]string
}

// MatchMetrics checks whether at least one metric in a batch matches this filter.
func (f Filter) MatchMetrics(md pmetric.Metrics) bool {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				if f.MatchMetric(rm.Resource().Attributes(), sm.Scope().Attributes(), metrics.At(k)) {
					return true
				}
			}
		}
	}

	return false
}

// MatchMetric checks whether one metric candidate with resource/scope context matches this filter.
func (f Filter) MatchMetric(resourceAttrs pcommon.Map, scopeAttrs pcommon.Map, metric pmetric.Metric) bool {
	if !f.acceptsSignal(model.SignalMetrics) {
		return false
	}
	if !f.matchResourceAttrs(resourceAttrs) {
		return false
	}
	if len(f.MetricNames) > 0 {
		if _, ok := f.MetricNames[metric.Name()]; !ok {
			return false
		}
	}
	if _, excluded := f.MetricNamesExclude[metric.Name()]; excluded {
		return false
	}
	if !f.matchMetricAttributeNames(resourceAttrs, scopeAttrs, metric) {
		return false
	}
	if !f.matchMetricDataPointCounts(metric) {
		return false
	}

	return true
}

func (f Filter) matchMetricDataPointCounts(metric pmetric.Metric) bool {
	if f.BucketCountsCount == nil && f.ExplicitBoundsCount == nil {
		return true
	}
	if metric.Type() != pmetric.MetricTypeHistogram {
		return false
	}

	dps := metric.Histogram().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		if f.BucketCountsCount != nil && dp.BucketCounts().Len() != *f.BucketCountsCount {
			continue
		}
		if f.ExplicitBoundsCount != nil && dp.ExplicitBounds().Len() != *f.ExplicitBoundsCount {
			continue
		}
		return true
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

		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ss := ilss.At(j)
			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if len(f.SpanNames) > 0 {
					if _, ok := f.SpanNames[span.Name()]; !ok {
						continue
					}
				}
				if _, excluded := f.SpanNamesExclude[span.Name()]; excluded {
					continue
				}
				if !f.matchTraceAttributeNames(rs.Resource().Attributes(), ss.Scope().Attributes(), span) {
					continue
				}
				return true
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
			sl := sls.At(j)
			logs := sl.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				record := logs.At(k)
				if !f.matchLogAttributeNames(rl.Resource().Attributes(), sl.Scope().Attributes(), record.Attributes()) {
					continue
				}
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

func (f Filter) matchMetricAttributeNames(resourceAttrs pcommon.Map, scopeAttrs pcommon.Map, metric pmetric.Metric) bool {
	if f.containsAnyKey(resourceAttrs, f.AttributeExclude) ||
		f.containsAnyKey(scopeAttrs, f.AttributeExclude) ||
		f.metricDataPointsContainAnyKey(metric, f.AttributeExclude) {
		return false
	}
	if len(f.AttributeNames) == 0 {
		return true
	}
	if f.containsAnyKey(resourceAttrs, f.AttributeNames) {
		return true
	}
	if f.containsAnyKey(scopeAttrs, f.AttributeNames) {
		return true
	}
	return f.metricDataPointsContainAnyKey(metric, f.AttributeNames)
}

func (f Filter) metricDataPointsContainAnyKey(metric pmetric.Metric, keys map[string]struct{}) bool {
	if len(keys) == 0 {
		return false
	}
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			if f.containsAnyKey(dps.At(i).Attributes(), keys) {
				return true
			}
		}
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			if f.containsAnyKey(dps.At(i).Attributes(), keys) {
				return true
			}
		}
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			if f.containsAnyKey(dps.At(i).Attributes(), keys) {
				return true
			}
		}
	case pmetric.MetricTypeSummary:
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			if f.containsAnyKey(dps.At(i).Attributes(), keys) {
				return true
			}
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := metric.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			if f.containsAnyKey(dps.At(i).Attributes(), keys) {
				return true
			}
		}
	}

	return false
}

func (f Filter) matchTraceAttributeNames(resourceAttrs pcommon.Map, scopeAttrs pcommon.Map, span ptrace.Span) bool {
	if f.containsAnyKey(resourceAttrs, f.AttributeExclude) ||
		f.containsAnyKey(scopeAttrs, f.AttributeExclude) ||
		f.containsAnyKey(span.Attributes(), f.AttributeExclude) {
		return false
	}
	events := span.Events()
	for i := 0; i < events.Len(); i++ {
		if f.containsAnyKey(events.At(i).Attributes(), f.AttributeExclude) {
			return false
		}
	}
	if len(f.AttributeNames) == 0 {
		return true
	}
	if f.containsAnyKey(resourceAttrs, f.AttributeNames) {
		return true
	}
	if f.containsAnyKey(scopeAttrs, f.AttributeNames) {
		return true
	}
	if f.containsAnyKey(span.Attributes(), f.AttributeNames) {
		return true
	}
	for i := 0; i < events.Len(); i++ {
		if f.containsAnyKey(events.At(i).Attributes(), f.AttributeNames) {
			return true
		}
	}

	return false
}

func (f Filter) matchLogAttributeNames(resourceAttrs pcommon.Map, scopeAttrs pcommon.Map, logAttrs pcommon.Map) bool {
	if f.containsAnyKey(resourceAttrs, f.AttributeExclude) ||
		f.containsAnyKey(scopeAttrs, f.AttributeExclude) ||
		f.containsAnyKey(logAttrs, f.AttributeExclude) {
		return false
	}
	if len(f.AttributeNames) == 0 {
		return true
	}
	if f.containsAnyKey(resourceAttrs, f.AttributeNames) {
		return true
	}
	if f.containsAnyKey(scopeAttrs, f.AttributeNames) {
		return true
	}
	return f.containsAnyKey(logAttrs, f.AttributeNames)
}

func (f Filter) containsAnyKey(attrs pcommon.Map, keys map[string]struct{}) bool {
	if len(keys) == 0 {
		return false
	}
	for key := range keys {
		if _, ok := attrs.Get(key); ok {
			return true
		}
	}
	return false
}
