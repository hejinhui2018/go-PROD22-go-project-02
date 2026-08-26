package cli

import (
	"fleetforge/internal/domain"
	"testing"
)

func TestRunSmokeCompletesAndRecoversRelease(t *testing.T) {
	result, err := RunSmoke(t.TempDir())
	if err != nil {
		t.Fatalf("RunSmoke() error = %v", err)
	}
	if result.Completed != 1 {
		t.Fatalf("completed = %d, want 1", result.Completed)
	}
	if result.Status != domain.StatusCompleted {
		t.Fatalf("status = %q, want %q", result.Status, domain.StatusCompleted)
	}
}
