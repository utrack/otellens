package otellens

import (
	"go.opentelemetry.io/collector/exporter"

	internalexporter "github.com/utrack/otellens/internal/exporter"
)

// NewFactory exposes the collector exporter factory for custom distributions.
func NewFactory() exporter.Factory {
	return internalexporter.NewFactory()
}
