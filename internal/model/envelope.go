package model

import "time"

// SignalType identifies an OpenTelemetry signal family.
type SignalType string

const (
	SignalMetrics SignalType = "metrics"
	SignalTraces  SignalType = "traces"
	SignalLogs    SignalType = "logs"
)

// Envelope is a single NDJSON event streamed to API clients.
type Envelope struct {
	SessionID  string      `json:"session_id"`
	Signal     SignalType  `json:"signal"`
	BatchIndex uint64      `json:"batch_index"`
	CapturedAt time.Time   `json:"captured_at"`
	Payload    interface{} `json:"payload"`
}

// StreamEnd is emitted when a capture session ends.
type StreamEnd struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	Sent      uint64 `json:"sent"`
	Dropped   uint64 `json:"dropped"`
}

// MetricsPayload is a detailed metrics batch projection.
type MetricsPayload struct {
	ResourceMetrics int      `json:"resource_metrics"`
	MetricCount     int      `json:"metric_count"`
	Metrics         []Metric `json:"metrics"`
}

// Metric is a detailed metric projection including resource, scope, and datapoints.
type Metric struct {
	Name               string                 `json:"name"`
	Description        string                 `json:"description,omitempty"`
	Unit               string                 `json:"unit,omitempty"`
	Type               string                 `json:"type"`
	ResourceAttributes map[string]interface{} `json:"resource_attributes,omitempty"`
	Scope              Scope                  `json:"scope"`
	DataPoints         []MetricDataPoint      `json:"data_points"`
}

// Scope captures instrumentation scope identity and attributes.
type Scope struct {
	Name       string                 `json:"name,omitempty"`
	Version    string                 `json:"version,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// MetricDataPoint captures normalized datapoint details and aggregation fields.
type MetricDataPoint struct {
	StartTimeUnixNano uint64                 `json:"start_time_unix_nano,omitempty"`
	TimeUnixNano      uint64                 `json:"time_unix_nano,omitempty"`
	Attributes        map[string]interface{} `json:"attributes,omitempty"`
	Value             interface{}            `json:"value,omitempty"`
	Count             uint64                 `json:"count,omitempty"`
	Sum               float64                `json:"sum,omitempty"`
	QuantileValues    []QuantileValue        `json:"quantile_values,omitempty"`
	Flags             uint32                 `json:"flags,omitempty"`
}

// QuantileValue represents one summary quantile value pair.
type QuantileValue struct {
	Quantile float64 `json:"quantile"`
	Value    float64 `json:"value"`
}

// TracesPayload is a concise traces batch projection.
type TracesPayload struct {
	ResourceSpans int      `json:"resource_spans"`
	SpanCount     int      `json:"span_count"`
	SpanNames     []string `json:"span_names"`
}

// LogsPayload is a concise logs batch projection.
type LogsPayload struct {
	ResourceLogs int      `json:"resource_logs"`
	LogCount     int      `json:"log_count"`
	Bodies       []string `json:"bodies"`
}
