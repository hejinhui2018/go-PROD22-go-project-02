package service

import (
	"fleetforge/internal/domain"
	"fleetforge/internal/ports"
	"fleetforge/internal/store"
	"sync"
	"testing"
)

func TestSingleAgentOwnsClaimUnderConcurrentRequests(t *testing.T) {
	s := New(&store.MemoryStore{}, ports.NewFakeClock(), &ports.SequenceID{})
	release, err := s.CreateRelease("2.1.0", []string{"edge-17"}, 1, 1, 1, domain.RollbackPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RegisterDevice("edge-17", "2.0.0", []string{"delta"}); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	var firstErr error
	for _, agent := range []string{"agent-a", "agent-b"} {
		wg.Add(1)
		go func(agent string) {
			defer wg.Done()
			<-start
			if _, err := s.Claim("edge-17", agent); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			} else if err != ErrNoTask {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(agent)
	}
	close(start)
	wg.Wait()

	if firstErr != nil {
		t.Fatalf("unexpected claim error: %v", firstErr)
	}
	if successes != 1 {
		t.Fatalf("concurrent claim successes=%d, want 1", successes)
	}
	task := s.State.Tasks[s.ListTasks(release.ID)[0].ID]
	if task.Status != domain.TaskLeased {
		t.Fatalf("task status=%s, want leased", task.Status)
	}
	claimEvents := 0
	for _, event := range s.State.Audit {
		if event.Type == "task.updated" && event.Aggregate == task.ID {
			claimEvents++
		}
	}
	if claimEvents != 1 {
		t.Fatalf("claim events=%d, want 1", claimEvents)
	}
}
