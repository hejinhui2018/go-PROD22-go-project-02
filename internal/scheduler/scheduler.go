package scheduler

import (
	"context"
	"fleetforge/internal/domain"
	"time"
)

type Runtime interface {
	ReapExpired()
	TaskSnapshot() []*domain.Task
}

type Scheduler struct {
	S     Runtime
	Every time.Duration
	Runs  uint64
}

func (s *Scheduler) Run(ctx context.Context) {
	if s.Every <= 0 {
		s.Every = time.Second
	}
	t := time.NewTicker(s.Every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Tick()
		}
	}
}

func (s *Scheduler) Tick() int {
	if s.S == nil {
		return 0
	}
	before := len(s.S.TaskSnapshot())
	s.S.ReapExpired()
	s.Runs++
	return before - len(s.S.TaskSnapshot())
}

type Snapshot struct {
	Runs                                                    uint64
	Queued, Leased, Installing, Awaiting, Completed, Failed int
}

func (s *Scheduler) Status() Snapshot {
	out := Snapshot{Runs: s.Runs}
	if s.S == nil {
		return out
	}
	for _, t := range s.S.TaskSnapshot() {
		switch t.Status {
		case domain.TaskQueued, domain.TaskRejected:
			out.Queued++
		case domain.TaskLeased:
			out.Leased++
		case domain.TaskInstalling:
			out.Installing++
		case domain.TaskAwaiting:
			out.Awaiting++
		case domain.TaskCompleted:
			out.Completed++
		case domain.TaskFailed:
			out.Failed++
		}
	}
	return out
}
