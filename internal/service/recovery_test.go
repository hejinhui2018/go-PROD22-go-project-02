package service

import (
	"fleetforge/internal/domain"
	"fleetforge/internal/ports"
	"fleetforge/internal/store"
	"testing"
)

func TestRecovery(t *testing.T) {
	st := &store.MemoryStore{}
	c := ports.NewFakeClock()
	s := New(st, c, &ports.SequenceID{})
	r, _ := s.CreateRelease("2.0", []string{"d"}, 1, 1, 1, domain.RollbackPolicy{})
	s2 := New(st, c, &ports.SequenceID{})
	if _, ok := s2.GetRelease(r.ID); !ok {
		t.Fatal("not recovered")
	}
}
