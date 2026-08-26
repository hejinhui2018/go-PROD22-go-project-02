package domain

import "testing"

func TestTransitions(t *testing.T) {
	if !ValidTransition(TaskQueued, TaskLeased) || ValidTransition(TaskCompleted, TaskQueued) {
		t.Fatal("bad transition matrix")
	}
}
