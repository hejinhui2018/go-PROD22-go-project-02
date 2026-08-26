package service

import (
	"fleetforge/internal/domain"
	"sort"
)

type Page struct {
	Items  any `json:"items"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Total  int `json:"total"`
}

func clampPage(offset, limit, total int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return offset, end
}
func (s *Service) ReleasePage(offset, limit int) Page {
	items := s.ListReleases()
	offset, end := clampPage(offset, limit, len(items))
	return Page{Items: items[offset:end], Offset: offset, Limit: end - offset, Total: len(items)}
}
func (s *Service) TaskPage(release string, offset, limit int) Page {
	items := s.ListTasks(release)
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.Before(items[j].UpdatedAt) })
	offset, end := clampPage(offset, limit, len(items))
	return Page{Items: items[offset:end], Offset: offset, Limit: end - offset, Total: len(items)}
}
func (s *Service) TasksByStatus(status domain.TaskStatus) []*domain.Task {
	out := []*domain.Task{}
	for _, t := range s.State.Tasks {
		if t.Status == status {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.Before(out[j].UpdatedAt) })
	return out
}
func (s *Service) ReleaseTaskCounts(id string) map[domain.TaskStatus]int {
	out := map[domain.TaskStatus]int{}
	for _, t := range s.State.Tasks {
		if t.ReleaseID == id {
			out[t.Status]++
		}
	}
	return out
}
