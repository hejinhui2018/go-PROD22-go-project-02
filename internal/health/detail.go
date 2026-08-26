package health

import (
	"fleetforge/internal/domain"
	"fleetforge/internal/recovery"
	"fleetforge/internal/store"
	"sort"
	"strconv"
	"time"
)

type Detail struct {
	Report    Report          `json:"counts"`
	Recovery  recovery.Report `json:"recovery"`
	ByRelease map[string]int  `json:"by_release"`
	Offline   []string        `json:"offline_devices"`
	Stale     []string        `json:"stale_devices"`
	CheckedAt time.Time       `json:"checked_at"`
}

func Detailed(s store.State, now time.Time) Detail {
	out := Detail{Report: Check(s), Recovery: recovery.Inspect(s), ByRelease: map[string]int{}, CheckedAt: now}
	for _, t := range s.Tasks {
		if t.Status == domain.TaskLeased || t.Status == domain.TaskInstalling || t.Status == domain.TaskAwaiting {
			out.ByRelease[t.ReleaseID]++
		}
	}
	for id, d := range s.Devices {
		if !d.Online {
			out.Offline = append(out.Offline, id)
		}
		if !d.LastSeen.IsZero() && now.Sub(d.LastSeen) > 10*time.Minute {
			out.Stale = append(out.Stale, id)
		}
	}
	sort.Strings(out.Offline)
	sort.Strings(out.Stale)
	return out
}
func Healthy(d Detail) bool { return d.Recovery.Orphaned == 0 && len(d.Recovery.Errors) == 0 }
func (r Report) String() string {
	return "releases=" + strconv.Itoa(r.Releases) + " devices=" + strconv.Itoa(r.Devices) + " tasks=" + strconv.Itoa(r.Tasks)
}
