package exporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// sinkExporter is a shared implementation used by all signal-specific exporters.
type sinkExporter struct {
	runtime *runtime
}

func newSinkExporter(cfg *Config) *sinkExporter {
	return &sinkExporter{runtime: acquireRuntime(cfg)}
}

func (e *sinkExporter) start(context.Context, component.Host) error {
	return e.runtime.start()
}

func (e *sinkExporter) shutdown(ctx context.Context) error {
	return e.runtime.release(ctx)
}

func (e *sinkExporter) pushMetrics(_ context.Context, md pmetric.Metrics) error {
	e.runtime.registry.PublishMetrics(md)
	return nil
}

func (e *sinkExporter) pushTraces(_ context.Context, td ptrace.Traces) error {
	e.runtime.registry.PublishTraces(td)
	return nil
}

func (e *sinkExporter) pushLogs(_ context.Context, ld plog.Logs) error {
	e.runtime.registry.PublishLogs(ld)
	return nil
}
