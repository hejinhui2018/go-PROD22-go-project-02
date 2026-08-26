package recovery

import (
	"fleetforge/internal/domain"
	"fleetforge/internal/store"
	"fmt"
	"sort"
)

type Report struct {
	Releases, Devices, Tasks   int
	Terminal, Active, Orphaned int
	Errors                     []string
}

func Inspect(s store.State) Report {
	out := Report{Releases: len(s.Releases), Devices: len(s.Devices), Tasks: len(s.Tasks)}
	for id, r := range s.Releases {
		if r.Terminal() {
			out.Terminal++
		}
		if r.Status == domain.StatusInstalling || r.Status == domain.StatusPreflight || r.Status == domain.StatusAwaiting {
			out.Active++
		}
		if id == "" {
			out.Errors = append(out.Errors, "release has empty id")
		}
	}
	for id, t := range s.Tasks {
		if _, ok := s.Releases[t.ReleaseID]; !ok {
			out.Orphaned++
			out.Errors = append(out.Errors, fmt.Sprintf("task %s references missing release", id))
		}
		if _, ok := s.Devices[t.DeviceID]; !ok {
			out.Orphaned++
			out.Errors = append(out.Errors, fmt.Sprintf("task %s references missing device", id))
		}
	}
	sort.Strings(out.Errors)
	return out
}

func Repairable(s store.State) bool { return len(Inspect(s).Errors) == 0 }
