package service

import (
	"fleetforge/internal/domain"
	"fleetforge/internal/ports"
	"fleetforge/internal/store"
	"testing"
	"time"
)

func TestLeaseExpiry(t *testing.T) {
	c := ports.NewFakeClock()
	s := New(&store.MemoryStore{}, c, &ports.SequenceID{})
	s.CreateRelease("2.0", []string{"d"}, 1, 1, 1, domain.RollbackPolicy{})
	s.RegisterDevice("d", "1", nil)
	t1, _ := s.Claim("d", "a")
	c.Advance(6 * time.Minute)
	s.ReapExpired()
	if s.State.Tasks[t1.ID].Status != domain.TaskQueued {
		t.Fatal("lease not requeued")
	}
}
