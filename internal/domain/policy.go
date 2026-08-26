package domain

import "time"

type BatchWindow struct{ Start, End time.Time }

func (w BatchWindow) Open(now time.Time) bool {
	return w.Start.IsZero() || (now.Equal(w.Start) || now.After(w.Start)) && (w.End.IsZero() || now.Before(w.End))
}
func (r *Release) Capacity(tasks []*Task) int {
	active := 0
	for _, t := range tasks {
		if t.ReleaseID == r.ID && (t.Status == TaskLeased || t.Status == TaskInstalling || t.Status == TaskAwaiting) {
			active++
		}
	}
	left := r.MaxConcurrent - active
	if left < 0 {
		return 0
	}
	if left > r.BatchSize {
		return r.BatchSize
	}
	return left
}
func (r *Release) ShouldRollback() bool { return r.Rollback.Auto && r.Failed > r.Rollback.MaxFailures }
func (r *Release) Terminal() bool {
	return r.Status == StatusCompleted || r.Status == StatusRolledBack || r.Status == StatusFailed
}
