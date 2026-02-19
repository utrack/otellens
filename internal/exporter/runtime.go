package exporter

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/utrack/otellens/internal/capture"
	"github.com/utrack/otellens/internal/httpapi"
	"go.uber.org/zap"
)

type runtime struct {
	cfg      Config
	registry *capture.Registry
	logger   *zap.Logger
	server   *http.Server

	refs atomic.Int64

	startOnce sync.Once
	startErr  error

	shutdownOnce sync.Once
	shutdownErr  error
}

var (
	runtimesMu sync.Mutex
	runtimes   = make(map[string]*runtime)
)

func acquireRuntime(cfg *Config) *runtime {
	runtimesMu.Lock()
	defer runtimesMu.Unlock()

	rt, ok := runtimes[cfg.HTTPAddr]
	if !ok {
		logger := zap.NewNop()
		registry := capture.NewRegistry(cfg.MaxConcurrentSessions)
		handler := httpapi.NewHandler(registry, logger)
		mux := http.NewServeMux()
		handler.RegisterRoutes(mux)

		rt = &runtime{
			cfg:      *cfg,
			registry: registry,
			logger:   logger,
			server: &http.Server{
				Addr:              cfg.HTTPAddr,
				Handler:           mux,
				ReadHeaderTimeout: 5 * time.Second,
			},
		}
		runtimes[cfg.HTTPAddr] = rt
	}
	if cfg.MaxConcurrentSessions > rt.cfg.MaxConcurrentSessions {
		rt.cfg.MaxConcurrentSessions = cfg.MaxConcurrentSessions
	}
	rt.refs.Add(1)
	return rt
}

func (r *runtime) start() error {
	r.startOnce.Do(func() {
		go func() {
			if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				r.logger.Error("otellens API server failed", zap.Error(err), zap.String("addr", r.server.Addr))
			}
		}()
	})
	return r.startErr
}

func (r *runtime) release(ctx context.Context) error {
	if r.refs.Add(-1) > 0 {
		return nil
	}

	r.shutdownOnce.Do(func() {
		r.shutdownErr = r.server.Shutdown(ctx)
		runtimesMu.Lock()
		delete(runtimes, r.server.Addr)
		runtimesMu.Unlock()
	})

	return r.shutdownErr
}
