package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

type fakeStore struct {
	calls int32
	err   error
}

func (f *fakeStore) ProcessDueRecurringTemplates(asOf time.Time) (int, error) {
	atomic.AddInt32(&f.calls, 1)
	return 1, f.err
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunOnceCalledImmediately(t *testing.T) {
	store := &fakeStore{}
	s := New(store, testLogger())
	s.interval = time.Hour

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	s.Run(ctx)

	if got := atomic.LoadInt32(&store.calls); got != 1 {
		t.Errorf("ProcessDueRecurringTemplates called %d times, want 1", got)
	}
}

func TestRunTicksOnInterval(t *testing.T) {
	store := &fakeStore{}
	s := New(store, testLogger())
	s.interval = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Millisecond)
	defer cancel()
	s.Run(ctx)

	if got := atomic.LoadInt32(&store.calls); got < 3 {
		t.Errorf("ProcessDueRecurringTemplates called %d times, want at least 3", got)
	}
}

func TestRunOnceLogsErrorWithoutPanicking(t *testing.T) {
	store := &fakeStore{err: errors.New("boom")}
	s := New(store, testLogger())
	s.interval = time.Hour

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	s.Run(ctx)
}
