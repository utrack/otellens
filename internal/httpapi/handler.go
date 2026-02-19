package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/utrack/otellens/internal/capture"
	"github.com/utrack/otellens/internal/model"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

const defaultSessionTimeout = 30 * time.Second

// Handler exposes HTTP endpoints for live capture sessions.
type Handler struct {
	registry *capture.Registry
	logger   *zap.Logger
}

func NewHandler(registry *capture.Registry, logger *zap.Logger) *Handler {
	return &Handler{registry: registry, logger: logger}
}

// RegisterRoutes registers HTTP routes for the API server.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", h.handleRoot)
	mux.HandleFunc("/ui", h.handleUI)
	mux.HandleFunc("/v1/capture/stream", h.handleStream)
	mux.HandleFunc("/healthz", h.handleHealth)
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req StreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := validateRequest(req); err != nil {
		h.writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	if req.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutSeconds)*time.Second)
		defer cancel()
	} else {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultSessionTimeout)
		defer cancel()
	}

	session, err := h.registry.Register(ctx, capture.RegisterRequest{
		Filter:     requestToFilter(req),
		MaxBatches: req.MaxBatches,
		BufferSize: req.MaxBatches,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, capture.ErrSessionLimitReached) {
			status = http.StatusTooManyRequests
		}
		h.writeErr(w, status, err.Error())
		return
	}
	defer h.registry.Deregister(session.ID())

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeErr(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	for {
		select {
		case <-ctx.Done():
			_ = enc.Encode(model.StreamEnd{
				Type:      "end",
				SessionID: session.ID(),
				Sent:      session.SentBatches(),
				Dropped:   session.DroppedBatches(),
			})
			flusher.Flush()
			return
		case event, ok := <-session.Events():
			if !ok {
				_ = enc.Encode(model.StreamEnd{
					Type:      "end",
					SessionID: session.ID(),
					Sent:      session.SentBatches(),
					Dropped:   session.DroppedBatches(),
				})
				flusher.Flush()
				return
			}
			if err := enc.Encode(event); err != nil {
				h.logger.Debug("failed to stream event", zap.Error(err), zap.String("session_id", session.ID()))
				return
			}
			flusher.Flush()
		}
	}
}

func validateRequest(req StreamRequest) error {
	if req.MaxBatches <= 0 {
		return errors.New("max_batches must be > 0")
	}
	if req.TimeoutSeconds < 0 {
		return errors.New("timeout_seconds must be >= 0")
	}
	return nil
}

func requestToFilter(req StreamRequest) capture.Filter {
	signals := make(map[model.SignalType]struct{}, len(req.Signals))
	for _, signal := range req.Signals {
		signals[signal] = struct{}{}
	}

	metricNames := make(map[string]struct{}, len(req.MetricNames))
	for _, metricName := range req.MetricNames {
		metricNames[metricName] = struct{}{}
	}

	spanNames := make(map[string]struct{}, len(req.SpanNames))
	for _, spanName := range req.SpanNames {
		spanNames[spanName] = struct{}{}
	}

	attributeNames := make(map[string]struct{}, len(req.AttributeNames))
	for _, attributeName := range req.AttributeNames {
		attributeNames[attributeName] = struct{}{}
	}

	return capture.Filter{
		Signals:            signals,
		MetricNames:        metricNames,
		SpanNames:          spanNames,
		AttributeNames:     attributeNames,
		LogBodyContains:    req.LogBodyContains,
		MinSeverityNumber:  plog.SeverityNumber(req.MinSeverityNumber),
		ResourceAttributes: req.ResourceAttributes,
	}
}

func (h *Handler) writeErr(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(StreamError{Error: message})
}
