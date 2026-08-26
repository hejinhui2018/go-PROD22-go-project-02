package health

import "fleetforge/internal/store"

type Report struct {
	Releases, Devices, Tasks int
	Durable                  bool
}

func Check(s store.State) Report { return Report{len(s.Releases), len(s.Devices), len(s.Tasks), true} }
