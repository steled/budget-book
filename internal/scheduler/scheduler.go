// Package scheduler runs the recurring-template catch-up job: once at
// startup and then once every 24h for as long as the process runs. There is
// no external cron — a simple in-process time.Ticker is enough for a
// single-instance, single-user app.
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Store is the subset of database.Store the scheduler depends on.
type Store interface {
	ProcessDueRecurringTemplates(asOf time.Time) (int, error)
}

// Scheduler periodically books due recurring transactions.
type Scheduler struct {
	store    Store
	logger   *slog.Logger
	interval time.Duration
	now      func() time.Time
}

// New returns a Scheduler that checks for due recurring templates once a
// day.
func New(store Store, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		store:    store,
		logger:   logger,
		interval: 24 * time.Hour,
		now:      time.Now,
	}
}

// Run executes one catch-up pass immediately, then again every interval
// until ctx is cancelled. It blocks and should be run in its own goroutine.
func (s *Scheduler) Run(ctx context.Context) {
	s.runOnce()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce()
		}
	}
}

func (s *Scheduler) runOnce() {
	created, err := s.store.ProcessDueRecurringTemplates(s.now())
	if err != nil {
		s.logger.Error("recurring scheduler run failed", "error", err)
		return
	}
	if created > 0 {
		s.logger.Info("recurring scheduler booked transactions", "count", created)
	}
}
