package audit

import (
	"fleetforge/internal/events"
	"sort"
	"strings"
	"time"
)

type Summary struct {
	Total      int            `json:"total"`
	Since      time.Time      `json:"since"`
	Until      time.Time      `json:"until"`
	Types      map[string]int `json:"types"`
	Actors     map[string]int `json:"actors"`
	Aggregates map[string]int `json:"aggregates"`
	Recent     []events.Event `json:"recent"`
}

func Build(eventsIn []events.Event, since, until time.Time, limit int) Summary {
	out := Summary{Since: since, Until: until, Types: map[string]int{}, Actors: map[string]int{}, Aggregates: map[string]int{}}
	filtered := make([]events.Event, 0, len(eventsIn))
	for _, e := range eventsIn {
		if !since.IsZero() && e.At.Before(since) {
			continue
		}
		if !until.IsZero() && e.At.After(until) {
			continue
		}
		out.Total++
		out.Types[e.Type]++
		out.Aggregates[e.Aggregate]++
		actor := strings.TrimSpace(string(e.Data))
		if actor != "" {
			out.Actors[actor]++
		}
		filtered = append(filtered, e)
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].At.Before(filtered[j].At) })
	if limit <= 0 {
		limit = 20
	}
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	out.Recent = filtered
	return out
}

func Merge(a, b Summary) Summary {
	out := a
	out.Total += b.Total
	if out.Since.IsZero() || (!b.Since.IsZero() && b.Since.Before(out.Since)) {
		out.Since = b.Since
	}
	if b.Until.After(out.Until) {
		out.Until = b.Until
	}
	if out.Types == nil {
		out.Types = map[string]int{}
	}
	for k, v := range b.Types {
		out.Types[k] += v
	}
	if out.Actors == nil {
		out.Actors = map[string]int{}
	}
	for k, v := range b.Actors {
		out.Actors[k] += v
	}
	if out.Aggregates == nil {
		out.Aggregates = map[string]int{}
	}
	for k, v := range b.Aggregates {
		out.Aggregates[k] += v
	}
	out.Recent = append(out.Recent, b.Recent...)
	sort.Slice(out.Recent, func(i, j int) bool { return out.Recent[i].At.Before(out.Recent[j].At) })
	if len(out.Recent) > 20 {
		out.Recent = out.Recent[len(out.Recent)-20:]
	}
	return out
}
