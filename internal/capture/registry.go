package capture

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/utrack/otellens/internal/model"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var ErrSessionLimitReached = errors.New("session limit reached")

// RegisterRequest defines runtime knobs for creating a session.
type RegisterRequest struct {
	Filter         Filter
	VerboseMetrics bool
	MaxBatches     int
	BufferSize     int
}

// Registry stores active capture sessions and routes matching telemetry batches.
type Registry struct {
	maxSessions int

	mu       sync.RWMutex
	sessions map[string]*Session

	hasActive atomic.Bool
}

// NewRegistry creates a registry with a hard cap on active sessions.
func NewRegistry(maxSessions int) *Registry {
	if maxSessions <= 0 {
		maxSessions = 128
	}
	return &Registry{
		maxSessions: maxSessions,
		sessions:    make(map[string]*Session),
	}
}

func buildMatchingMetricsPayload(filter Filter, verboseMetrics bool, md pmetric.Metrics) (model.MetricsPayload, bool) {
	payload := model.MetricsPayload{Metrics: make([]model.Metric, 0)}

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resourceMatched := false

		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				if !filter.MatchMetric(rm.Resource().Attributes(), sm.Scope().Attributes(), metric) {
					continue
				}
				payload.Metrics = append(payload.Metrics, model.BuildMetric(rm.Resource().Attributes(), sm.Scope(), metric, verboseMetrics))
				payload.MetricCount++
				resourceMatched = true
			}
		}

		if resourceMatched {
			payload.ResourceMetrics++
		}
	}

	if payload.MetricCount == 0 {
		return model.MetricsPayload{}, false
	}

	return payload, true
}

// HasActiveSessions returns true if at least one filter is currently registered.
func (r *Registry) HasActiveSessions() bool {
	return r.hasActive.Load()
}

// Register creates a new session and removes it automatically when ctx is cancelled.
func (r *Registry) Register(ctx context.Context, req RegisterRequest) (*Session, error) {
	if req.MaxBatches <= 0 {
		req.MaxBatches = 1
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.sessions) >= r.maxSessions {
		return nil, ErrSessionLimitReached
	}

	sessionID := uuid.NewString()
	session := newSession(sessionID, req.Filter, req.VerboseMetrics, req.MaxBatches, req.BufferSize)
	r.sessions[sessionID] = session
	r.hasActive.Store(true)

	go func() {
		<-ctx.Done()
		r.Deregister(sessionID)
	}()

	return session, nil
}

// Deregister closes and removes a session.
func (r *Registry) Deregister(sessionID string) {
	r.mu.Lock()
	session, ok := r.sessions[sessionID]
	if ok {
		delete(r.sessions, sessionID)
	}
	r.hasActive.Store(len(r.sessions) > 0)
	r.mu.Unlock()

	if ok {
		session.Close()
	}
}

// PublishMetrics routes one metrics batch to all matching sessions.
func (r *Registry) PublishMetrics(md pmetric.Metrics) {
	if !r.HasActiveSessions() {
		return
	}

	sessions := r.snapshotSessions()

	for _, session := range sessions {
		payload, ok := buildMatchingMetricsPayload(session.Filter(), session.VerboseMetrics(), md)
		if !ok {
			continue
		}
		built := payload

		envelope := model.Envelope{
			SessionID:  session.ID(),
			Signal:     model.SignalMetrics,
			BatchIndex: session.SentBatches() + 1,
			CapturedAt: time.Now().UTC(),
			Payload:    &built,
		}

		_, completed := session.Emit(envelope)
		if completed {
			r.Deregister(session.ID())
		}
	}
}

// PublishTraces routes one traces batch to all matching sessions.
func (r *Registry) PublishTraces(td ptrace.Traces) {
	if !r.HasActiveSessions() {
		return
	}

	sessions := r.snapshotSessions()
	var payload *model.TracesPayload

	for _, session := range sessions {
		if !session.Filter().MatchTraces(td) {
			continue
		}
		if payload == nil {
			built := model.BuildTracesPayload(td)
			payload = &built
		}

		envelope := model.Envelope{
			SessionID:  session.ID(),
			Signal:     model.SignalTraces,
			BatchIndex: session.SentBatches() + 1,
			CapturedAt: time.Now().UTC(),
			Payload:    payload,
		}

		_, completed := session.Emit(envelope)
		if completed {
			r.Deregister(session.ID())
		}
	}
}

// PublishLogs routes one logs batch to all matching sessions.
func (r *Registry) PublishLogs(ld plog.Logs) {
	if !r.HasActiveSessions() {
		return
	}

	sessions := r.snapshotSessions()
	var payload *model.LogsPayload

	for _, session := range sessions {
		if !session.Filter().MatchLogs(ld) {
			continue
		}
		if payload == nil {
			built := model.BuildLogsPayload(ld)
			payload = &built
		}

		envelope := model.Envelope{
			SessionID:  session.ID(),
			Signal:     model.SignalLogs,
			BatchIndex: session.SentBatches() + 1,
			CapturedAt: time.Now().UTC(),
			Payload:    payload,
		}

		_, completed := session.Emit(envelope)
		if completed {
			r.Deregister(session.ID())
		}
	}
}

func (r *Registry) snapshotSessions() []*Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sessions := make([]*Session, 0, len(r.sessions))
	for _, session := range r.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}
