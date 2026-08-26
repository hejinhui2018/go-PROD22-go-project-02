package service

import (
	"fleetforge/internal/domain"
	"fleetforge/internal/health"
	"fleetforge/internal/recovery"
	"fleetforge/internal/store"
	"time"
)

type Metrics struct {
	At                                                          time.Time `json:"at"`
	Releases, Devices, Tasks, Active, Queued, Completed, Failed int
	Recovery                                                    recovery.Report `json:"recovery"`
	Healthy                                                     bool            `json:"healthy"`
}

func (s *Service) MetricsSnapshot() Metrics {
	m := Metrics{At: s.Clock.Now(), Releases: len(s.State.Releases), Devices: len(s.State.Devices), Tasks: len(s.State.Tasks), Recovery: recovery.Inspect(s.State)}
	m.Healthy = health.Healthy(health.Detailed(s.State, m.At))
	for _, t := range s.State.Tasks {
		switch t.Status {
		case domain.TaskQueued, domain.TaskRejected:
			m.Queued++
		case domain.TaskLeased, domain.TaskInstalling, domain.TaskAwaiting, domain.TaskRollingBack:
			m.Active++
		case domain.TaskCompleted, domain.TaskRolledBack:
			m.Completed++
		case domain.TaskFailed:
			m.Failed++
		}
	}
	return m
}

func (s *Service) DurableCheck() any {
	if d, ok := s.St.(*store.DurableStore); ok {
		return d.Check()
	}
	return map[string]any{"durable": false, "events": len(s.State.Audit)}
}
