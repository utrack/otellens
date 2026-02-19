# otellens

`otellens` is a custom OpenTelemetry Collector exporter designed for **live, on-demand debugging** in high-volume environments.

## Problem

Collector live streams are hard to debug safely:

- Enabling debug logs floods storage.
- Reproducing pipelines locally is often unrealistic.

This project provides an exporter sink that drops data by default and only captures targeted telemetry when an API client asks for it.

## Constraints and design principles

- **No retention by default**: if there are no active debug sessions, incoming telemetry is rejected immediately.
- **Low overhead hot path**: exporter checks an atomic flag and returns.
- **On-demand capture**: HTTP request creates a temporary filter session.
- **Bounded capture**: session ends after `max_batches`, timeout, or client disconnect.
- **Streaming output**: NDJSON stream over HTTP for immediate consumption.

## How it works

1. Collector sends logs/metrics/traces to `otellens` exporter.
2. Exporter checks active filter registry.
3. If no active sessions: drop batch (`nil` return).
4. If active sessions exist: match filters and push compact payload envelopes to session queues.
5. API handler streams envelopes to the caller and deregisters session on completion.

## HTTP API

### `POST /v1/capture/stream`

Request example:

```json
{
  "signals": ["metrics"],
  "metric_names": ["A"],
  "attribute_names": ["service.name", "http.route"],
  "max_batches": 15,
  "timeout_seconds": 30
}
```

`attribute_names` matches on OTEL attribute keys found in parsed attribute maps across signal structures:

- metrics: resource/scope/datapoint attributes
- traces: resource/scope/span/span-event attributes
- logs: resource/scope/log-record attributes

Response type: `application/x-ndjson`

Each line is either:

- a telemetry `Envelope`, or
- a terminal `StreamEnd` event.

## Collector usage

This module exposes `otellens.NewFactory()` so it can be wired into a custom Collector distribution.

Exporter type name: `otellens`

Example config section:

```yaml
exporters:
  otellens:
    http_addr: ":18080"
    max_concurrent_sessions: 256
    default_session_timeout: 30s
    session_buffer_size: 64
```

## Project layout

- `exporter.go`: public collector factory entrypoint.
- `internal/exporter`: collector exporter and shared runtime.
- `internal/capture`: filter/session/registry domain.
- `internal/httpapi`: NDJSON streaming API.
- `internal/model`: wire payload contracts.

## Local development

```bash
go mod tidy
go test './...'
```

## Build custom otelcol Docker image

The repository includes:

- `Dockerfile` (multi-stage build)
- `build/otelcol-builder.yaml` (OCB manifest)
- `build/otelcol-config.yaml` (runtime collector config)

Build image:

```bash
docker build -t otellens-otelcol:latest .
```

Run image:

```bash
docker run --rm -p 4317:4317 -p 4318:4318 -p 9411:9411 -p 13133:13133 -p 18080:18080 otellens-otelcol:latest
```

Included receivers in the default config:

- OTLP gRPC: `4317`
- OTLP HTTP: `4318`
- Zipkin: `9411`
- Prometheus scrape receiver (collector self-scrape): `localhost:8888`
- Health check extension: `13133`

Open capture stream:

```bash
curl -N -X POST http://localhost:18080/v1/capture/stream \
  -H 'content-type: application/json' \
  -d '{"signals":["metrics"],"metric_names":["A"],"max_batches":15,"timeout_seconds":30}'
```

## Publish Docker image from GitHub Actions

Workflow file:

- `.github/workflows/docker-publish.yml`

It builds and pushes to GHCR on:

- pushes to `main`
- version tags like `v1.0.0`
- manual runs (`workflow_dispatch`)

Published image:

```text
ghcr.io/<github-owner>/otellens
```

To make it public, open the package page in GitHub Container Registry and set visibility to **Public**.

## Notes

This project intentionally emits compact summaries rather than full pdata dumps in v1 to keep CPU and memory overhead predictable.
