package service

import "fleetforge/internal/domain"

func (s *Service) GetRelease(id string) (*domain.Release, bool) {
	r, ok := s.State.Releases[id]
	return r, ok
}
func (s *Service) ListTasks(release string) []*domain.Task {
	var out []*domain.Task
	for _, t := range s.State.Tasks {
		if t.ReleaseID == release {
			out = append(out, t)
		}
	}
	return out
}

func (s *Service) TaskSnapshot() []*domain.Task {
	out := make([]*domain.Task, 0, len(s.State.Tasks))
	for _, t := range s.State.Tasks {
		out = append(out, t)
	}
	return out
}
