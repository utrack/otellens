# otellens architecture

## Purpose

`otellens` is an OpenTelemetry Collector exporter used as a live-debug sink. It supports logs, traces, and metrics, while minimizing baseline runtime cost.

## Core domain entities

### Filter

A filter defines matching criteria for one capture session:

- accepted signal families
- metric names
- span names
- log body substring
- minimum log severity
- resource attributes

### Session

A session represents one API-driven debug stream.

- Bounded by `max_batches`
- Bounded by timeout/cancellation
- Uses non-blocking enqueue with a bounded channel
- Tracks sent and dropped counters

### Registry

The registry stores active sessions and routes incoming batches.

- Holds a thread-safe session map
- Exposes an atomic `hasActive` flag for hot-path skip
- Supports automatic lifecycle cleanup via context cancellation

## Runtime topology

1. Collector pipeline invokes exporter `Consume*` methods.
2. Runtime forwards batches to capture registry.
3. Registry computes payload summary only for matching sessions.
4. API handler streams NDJSON to client until termination.

## Performance strategy

### No-session path

- Atomic check only
- No allocation beyond normal call frame
- Immediate return

### Active-session path

- Copy session pointers snapshot under read lock
- Evaluate predicates per session
- Build payload once per signal batch (lazy)
- Non-blocking send into per-session queue

## Safety controls

- Maximum concurrent sessions
- Session timeout
- Queue bounds per session
- Session removal on disconnect/cancel

## API contract considerations

- Stream endpoint is unauthenticated in v1 (intended for trusted/internal environments)
- NDJSON enables incremental reads and low buffering
- Each stream ends with a terminal event containing sent/dropped counters
