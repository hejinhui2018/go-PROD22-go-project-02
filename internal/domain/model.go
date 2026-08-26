package domain

import "time"

type RollbackPolicy struct {
	MaxFailures int  `json:"max_failures"`
	Auto        bool `json:"auto"`
}
type Release struct {
	ID, Version                          string
	Devices                              []string
	BatchSize, MaxConcurrent, RetryLimit int
	PauseWindow                          time.Duration
	Rollback                             RollbackPolicy
	Status                               ReleaseStatus
	CreatedAt, UpdatedAt                 time.Time
	Completed                            int
	Failed                               int
}
type Device struct {
	ID, Firmware           string
	Capabilities           []string
	RegisteredAt, LastSeen time.Time
	Online                 bool
}
type Task struct {
	ID, ReleaseID, DeviceID string
	Status                  TaskStatus
	LeaseOwner              string
	LeaseUntil              time.Time
	Attempts                int
	NextRetry               time.Time
	UpdatedAt               time.Time
	Error                   string
}

func (r *Release) CanPause() bool {
	return r.Status != StatusCompleted && r.Status != StatusRolledBack && r.Status != StatusFailed
}
