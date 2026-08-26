package scheduler

import (
	"fleetforge/internal/domain"
	"sort"
	"time"
)

type PlanItem struct {
	ReleaseID, TaskID, DeviceID string
	Priority                    int
	Status                      domain.TaskStatus
	ReadyAt                     time.Time
}

type Planner interface {
	ListReleases() []*domain.Release
	ListTasks(string) []*domain.Task
}

func BuildPlan(s Planner, now time.Time) []PlanItem {
	items := []PlanItem{}
	for _, r := range s.ListReleases() {
		if !r.Ready(s.ListTasks(r.ID), now) {
			continue
		}
		for _, t := range s.ListTasks(r.ID) {
			if t.Status != domain.TaskQueued && t.Status != domain.TaskRejected {
				continue
			}
			if !t.NextRetry.IsZero() && now.Before(t.NextRetry) {
				continue
			}
			items = append(items, PlanItem{ReleaseID: r.ID, TaskID: t.ID, DeviceID: t.DeviceID, Priority: r.MaxConcurrent - r.Capacity(s.ListTasks(r.ID)), Status: t.Status, ReadyAt: t.NextRetry})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority == items[j].Priority {
			return items[i].ReadyAt.Before(items[j].ReadyAt)
		}
		return items[i].Priority < items[j].Priority
	})
	return items
}

func GroupByDevice(plan []PlanItem) map[string][]PlanItem {
	out := map[string][]PlanItem{}
	for _, p := range plan {
		out[p.DeviceID] = append(out[p.DeviceID], p)
	}
	return out
}
