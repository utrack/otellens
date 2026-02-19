package capture

import (
	"sync"
	"sync/atomic"

	"github.com/utrack/otellens/internal/model"
)

// Session is a single active API-driven capture stream.
type Session struct {
	id         string
	filter     Filter
	maxBatches uint64

	events chan model.Envelope
	done   chan struct{}
	once   sync.Once

	sentBatches    atomic.Uint64
	droppedBatches atomic.Uint64
}

func newSession(id string, filter Filter, maxBatches int, bufferSize int) *Session {
	if bufferSize <= 0 {
		bufferSize = 32
	}
	return &Session{
		id:         id,
		filter:     filter,
		maxBatches: uint64(maxBatches),
		events:     make(chan model.Envelope, bufferSize),
		done:       make(chan struct{}),
	}
}

// ID returns the immutable session identifier.
func (s *Session) ID() string { return s.id }

// Filter returns session filter definition.
func (s *Session) Filter() Filter { return s.filter }

// Events returns a read-only stream of capture envelopes.
func (s *Session) Events() <-chan model.Envelope { return s.events }

// Done closes when session is terminated.
func (s *Session) Done() <-chan struct{} { return s.done }

// SentBatches returns number of successfully streamed batches.
func (s *Session) SentBatches() uint64 { return s.sentBatches.Load() }

// DroppedBatches returns number of dropped batches due to backpressure.
func (s *Session) DroppedBatches() uint64 { return s.droppedBatches.Load() }

// Emit tries to enqueue one envelope without blocking the hot path.
func (s *Session) Emit(envelope model.Envelope) (streamed bool, completed bool) {
	select {
	case <-s.done:
		return false, true
	default:
	}

	select {
	case s.events <- envelope:
		sent := s.sentBatches.Add(1)
		if s.maxBatches > 0 && sent >= s.maxBatches {
			s.Close()
			return true, true
		}
		return true, false
	default:
		s.droppedBatches.Add(1)
		return false, false
	}
}

// Close ends the session and releases stream resources.
func (s *Session) Close() {
	s.once.Do(func() {
		close(s.done)
		close(s.events)
	})
}
