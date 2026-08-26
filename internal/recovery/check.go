package recovery

import (
	"fleetforge/internal/store"
	"fmt"
)

func Check(s store.State) error {
	for id, t := range s.Tasks {
		if t.ReleaseID == "" || t.DeviceID == "" {
			return fmt.Errorf("task %s has missing relation", id)
		}
		if _, ok := s.Releases[t.ReleaseID]; !ok {
			return fmt.Errorf("task %s release missing", id)
		}
	}
	return nil
}
