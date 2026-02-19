package model

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// BuildMetricsPayload creates a detailed metrics projection for streaming clients.
func BuildMetricsPayload(md pmetric.Metrics) MetricsPayload {
	metricsOut := make([]Metric, 0, md.MetricCount())

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			scope := sms.At(j).Scope()

			metrics := sms.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metricsOut = append(metricsOut, BuildMetric(rms.At(i).Resource().Attributes(), scope, metrics.At(k), false))
			}
		}
	}

	return MetricsPayload{
		ResourceMetrics: rms.Len(),
		MetricCount:     md.MetricCount(),
		Metrics:         metricsOut,
	}
}

// BuildMetric creates a detailed projection for one metric candidate with its resource/scope context.
// If verboseMetrics is true, histogram datapoints include bucket_counts and explicit_bounds.
func BuildMetric(resourceAttrs pcommon.Map, scope pcommon.InstrumentationScope, metric pmetric.Metric, verboseMetrics bool) Metric {
	return Metric{
		Name:               metric.Name(),
		Description:        metric.Description(),
		Unit:               metric.Unit(),
		Type:               metric.Type().String(),
		ResourceAttributes: mapFromAttrs(resourceAttrs),
		Scope: Scope{
			Name:       scope.Name(),
			Version:    scope.Version(),
			Attributes: mapFromAttrs(scope.Attributes()),
		},
		DataPoints: buildMetricDataPoints(metric, verboseMetrics),
	}
}

func buildMetricDataPoints(metric pmetric.Metric, verboseMetrics bool) []MetricDataPoint {
	points := make([]MetricDataPoint, 0)

	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		points = make([]MetricDataPoint, 0, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			points = append(points, numberDataPointToModel(dp))
		}
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		points = make([]MetricDataPoint, 0, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			points = append(points, numberDataPointToModel(dp))
		}
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		points = make([]MetricDataPoint, 0, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			entry := MetricDataPoint{
				StartTimeUnixNano: uint64(dp.StartTimestamp()),
				TimeUnixNano:      uint64(dp.Timestamp()),
				Attributes:        mapFromAttrs(dp.Attributes()),
				Count:             dp.Count(),
				Sum:               dp.Sum(),
				Flags:             uint32(dp.Flags()),
			}
			if verboseMetrics {
				entry.BucketCounts = uint64Slice(dp.BucketCounts())
				entry.ExplicitBounds = float64Slice(dp.ExplicitBounds())
			}
			points = append(points, entry)
		}
	case pmetric.MetricTypeSummary:
		dps := metric.Summary().DataPoints()
		points = make([]MetricDataPoint, 0, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			qv := dp.QuantileValues()
			quantiles := make([]QuantileValue, 0, qv.Len())
			for j := 0; j < qv.Len(); j++ {
				item := qv.At(j)
				quantiles = append(quantiles, QuantileValue{Quantile: item.Quantile(), Value: item.Value()})
			}
			points = append(points, MetricDataPoint{
				StartTimeUnixNano: uint64(dp.StartTimestamp()),
				TimeUnixNano:      uint64(dp.Timestamp()),
				Attributes:        mapFromAttrs(dp.Attributes()),
				Count:             dp.Count(),
				Sum:               dp.Sum(),
				QuantileValues:    quantiles,
				Flags:             uint32(dp.Flags()),
			})
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := metric.ExponentialHistogram().DataPoints()
		points = make([]MetricDataPoint, 0, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			points = append(points, MetricDataPoint{
				StartTimeUnixNano: uint64(dp.StartTimestamp()),
				TimeUnixNano:      uint64(dp.Timestamp()),
				Attributes:        mapFromAttrs(dp.Attributes()),
				Count:             dp.Count(),
				Sum:               dp.Sum(),
				Flags:             uint32(dp.Flags()),
			})
		}
	}

	return points
}

func numberDataPointToModel(dp pmetric.NumberDataPoint) MetricDataPoint {
	out := MetricDataPoint{
		StartTimeUnixNano: uint64(dp.StartTimestamp()),
		TimeUnixNano:      uint64(dp.Timestamp()),
		Attributes:        mapFromAttrs(dp.Attributes()),
		Flags:             uint32(dp.Flags()),
	}

	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		out.Value = dp.IntValue()
	case pmetric.NumberDataPointValueTypeDouble:
		out.Value = dp.DoubleValue()
	}

	return out
}

func uint64Slice(src pcommon.UInt64Slice) []uint64 {
	if src.Len() == 0 {
		return nil
	}
	out := make([]uint64, 0, src.Len())
	for i := 0; i < src.Len(); i++ {
		out = append(out, src.At(i))
	}
	return out
}

func float64Slice(src pcommon.Float64Slice) []float64 {
	if src.Len() == 0 {
		return nil
	}
	out := make([]float64, 0, src.Len())
	for i := 0; i < src.Len(); i++ {
		out = append(out, src.At(i))
	}
	return out
}

func mapFromAttrs(attrs pcommon.Map) map[string]interface{} {
	out := make(map[string]interface{}, attrs.Len())
	attrs.Range(func(key string, value pcommon.Value) bool {
		out[key] = valueToAny(value)
		return true
	})
	if len(out) == 0 {
		return nil
	}
	return out
}

func valueToAny(value pcommon.Value) interface{} {
	switch value.Type() {
	case pcommon.ValueTypeStr:
		return value.Str()
	case pcommon.ValueTypeBool:
		return value.Bool()
	case pcommon.ValueTypeInt:
		return value.Int()
	case pcommon.ValueTypeDouble:
		return value.Double()
	case pcommon.ValueTypeMap:
		return mapFromAttrs(value.Map())
	case pcommon.ValueTypeSlice:
		slice := value.Slice()
		out := make([]interface{}, 0, slice.Len())
		for i := 0; i < slice.Len(); i++ {
			out = append(out, valueToAny(slice.At(i)))
		}
		return out
	default:
		return value.AsString()
	}
}

// BuildTracesPayload creates a lightweight summary for traces batches.
func BuildTracesPayload(td ptrace.Traces) TracesPayload {
	names := make([]string, 0)
	seen := make(map[string]struct{})

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				name := spans.At(k).Name()
				if _, ok := seen[name]; ok {
					continue
				}
				seen[name] = struct{}{}
				names = append(names, name)
			}
		}
	}

	return TracesPayload{
		ResourceSpans: rss.Len(),
		SpanCount:     td.SpanCount(),
		SpanNames:     names,
	}
}

// BuildLogsPayload creates a lightweight summary for logs batches.
func BuildLogsPayload(ld plog.Logs) LogsPayload {
	bodies := make([]string, 0)

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		sls := rls.At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			logs := sls.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				if len(bodies) >= 10 {
					break
				}
				body := logs.At(k).Body()
				if body.Type() == 0 {
					continue
				}
				bodies = append(bodies, fmt.Sprintf("%v", body.AsString()))
			}
		}
	}

	return LogsPayload{
		ResourceLogs: rls.Len(),
		LogCount:     ld.LogRecordCount(),
		Bodies:       bodies,
	}
}
