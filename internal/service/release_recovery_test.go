package service

import (
	"errors"
	"fleetforge/internal/domain"
	"fleetforge/internal/ports"
	"fleetforge/internal/store"
	"testing"
	"time"
)

func TestPausedReleaseLeaseRecoveryPreservesDispatchWindow(t *testing.T) {
	clock := ports.NewFakeClock()
	svc := New(&store.MemoryStore{}, clock, &ports.SequenceID{})
	release, err := svc.CreateRelease("2.0", []string{"device-a"}, 1, 1, 2, domain.RollbackPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	svc.State.Releases[release.ID].PauseWindow = 10 * time.Minute
	if _, err := svc.RegisterDevice("device-a", "1.0", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Claim("device-a", "agent-a"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetRelease(release.ID, "pause"); err != nil {
		t.Fatal(err)
	}

	clock.Advance(6 * time.Minute)
	svc.ReapExpired()
	if err := svc.SetRelease(release.ID, "resume"); err != nil {
		t.Fatal(err)
	}
	clock.Advance(2 * time.Minute)

	decision := svc.DispatchDecision(release.ID)
	if decision.Allowed {
		t.Fatalf("dispatch reopened before the pause window ended: %+v", decision)
	}
	if _, err := svc.Claim("device-a", "agent-a"); !errors.Is(err, ErrNoTask) {
		t.Fatalf("claim error = %v, want no task during the pause window", err)
	}
}
