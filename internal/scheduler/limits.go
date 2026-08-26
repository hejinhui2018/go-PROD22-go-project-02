package scheduler

import "fleetforge/internal/domain"

func Active(tasks []*domain.Task) int {
	n := 0
	for _, t := range tasks {
		if t.Status == domain.TaskLeased || t.Status == domain.TaskInstalling || t.Status == domain.TaskAwaiting {
			n++
		}
	}
	return n
}
