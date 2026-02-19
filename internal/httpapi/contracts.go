package httpapi

import "github.com/utrack/otellens/internal/model"

// StreamRequest defines filters for one on-demand capture session.
type StreamRequest struct {
	Signals            []model.SignalType `json:"signals"`
	MetricNames        []string           `json:"metric_names"`
	SpanNames          []string           `json:"span_names"`
	LogBodyContains    string             `json:"log_body_contains"`
	MinSeverityNumber  int32              `json:"min_severity_number"`
	ResourceAttributes map[string]string  `json:"resource_attributes"`
	MaxBatches         int                `json:"max_batches"`
	TimeoutSeconds     int                `json:"timeout_seconds"`
}

// StreamError is serialized for API-level failures.
type StreamError struct {
	Error string `json:"error"`
}
