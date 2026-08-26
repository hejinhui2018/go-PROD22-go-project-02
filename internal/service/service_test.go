package service

import (
	"fleetforge/internal/domain"
	"fleetforge/internal/ports"
	"fleetforge/internal/store"
	"testing"
)

func setup() *Service {
	c := ports.NewFakeClock()
	return New(&store.MemoryStore{}, c, &ports.SequenceID{})
}
func TestReleaseLifecycle(t *testing.T) {
	s := setup()
	r, e := s.CreateRelease("2.0", []string{"d1"}, 1, 1, 1, domain.RollbackPolicy{})
	if e != nil {
		t.Fatal(e)
	}
	s.RegisterDevice("d1", "1", nil)
	task, e := s.Claim("d1", "a")
	if e != nil {
		t.Fatal(e)
	}
	s.UpdateTask(task.ID, "ack", "")
	s.UpdateTask(task.ID, "complete", "")
	if s.State.Releases[r.ID].Status != domain.StatusCompleted {
		t.Fatalf("status=%s", s.State.Releases[r.ID].Status)
	}
}
