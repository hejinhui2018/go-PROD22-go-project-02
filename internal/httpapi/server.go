package httpapi

import (
	"encoding/json"
	"fleetforge/internal/domain"
	"fleetforge/internal/service"
	"net/http"
	"strings"
)

type Server struct{ S *service.Service }

func (s *Server) Handler() http.Handler { return http.HandlerFunc(s.handle) }
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.Trim(r.URL.Path, "/")
	if path == "healthz" {
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "recovery": s.S.RecoveryInfo(), "counts": map[string]int{"releases": len(s.S.State.Releases), "devices": len(s.S.State.Devices), "tasks": len(s.S.State.Tasks)}})
		return
	}
	if path == "readyz" {
		reply(w, map[string]any{"ready": true}, nil)
		return
	}
	if path == "v1/audit" && r.Method == "GET" {
		reply(w, s.S.AuditSummary(), nil)
		return
	}
	if path == "v1/metrics" && r.Method == "GET" {
		reply(w, s.S.MetricsSnapshot(), nil)
		return
	}
	if path == "v1/storage/check" && r.Method == "GET" {
		reply(w, s.S.DurableCheck(), nil)
		return
	}
	if path == "v1/releases" && r.Method == "GET" {
		reply(w, s.S.ReleasePage(0, 20), nil)
		return
	}
	if r.Method == "POST" && path == "v1/devices" {
		var x struct {
			ID, Firmware string
			Capabilities []string
		}
		if err := json.NewDecoder(r.Body).Decode(&x); err != nil {
			reply(w, nil, err)
			return
		}
		d, e := s.S.RegisterDevice(x.ID, x.Firmware, x.Capabilities)
		reply(w, d, e)
		return
	}
	if r.Method == "POST" && path == "v1/releases" {
		var x struct {
			Version                              string
			Devices                              []string
			BatchSize, MaxConcurrent, RetryLimit int
			Rollback                             domain.RollbackPolicy
		}
		if err := json.NewDecoder(r.Body).Decode(&x); err != nil {
			reply(w, nil, err)
			return
		}
		v, e := s.S.CreateRelease(x.Version, x.Devices, x.BatchSize, x.MaxConcurrent, x.RetryLimit, x.Rollback)
		reply(w, v, e)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) == 3 && parts[0] == "v1" && parts[1] == "releases" {
		if r.Method == "GET" {
			v, ok := s.S.State.Releases[parts[2]]
			if !ok {
				reply(w, nil, notFound())
				return
			}
			reply(w, v, nil)
			return
		}
	}
	if len(parts) == 4 && parts[0] == "v1" && parts[1] == "releases" && parts[3] == "plan" && r.Method == "GET" {
		v, e := s.S.ReleasePlan(parts[2])
		reply(w, v, e)
		return
	}
	if len(parts) == 4 && parts[0] == "v1" && parts[1] == "releases" && parts[3] == "rollback-tasks" && r.Method == "POST" {
		var x struct{ Reason string }
		_ = json.NewDecoder(r.Body).Decode(&x)
		v, e := s.S.QueueRollback(parts[2], x.Reason)
		reply(w, v, e)
		return
	}
	if len(parts) == 4 && parts[0] == "v1" && parts[1] == "releases" && r.Method == "POST" {
		reply(w, nil, s.S.SetRelease(parts[2], parts[3]))
		return
	}
	if len(parts) == 4 && parts[0] == "v1" && parts[1] == "agents" && parts[3] == "claim" {
		var x struct{ Agent string }
		if err := json.NewDecoder(r.Body).Decode(&x); err != nil {
			reply(w, nil, err)
			return
		}
		v, e := s.S.Claim(parts[2], x.Agent)
		reply(w, v, e)
		return
	}
	if len(parts) == 4 && parts[0] == "v1" && parts[1] == "agents" && parts[3] == "claim-rollback" {
		var x struct{ Agent string }
		_ = json.NewDecoder(r.Body).Decode(&x)
		v, e := s.S.ClaimRollback(parts[2], x.Agent)
		reply(w, v, e)
		return
	}
	if len(parts) == 4 && parts[0] == "v1" && parts[1] == "tasks" && r.Method == "POST" {
		var x struct {
			Reason, Agent, Detail string
			Percent               int
		}
		if err := json.NewDecoder(r.Body).Decode(&x); err != nil {
			reply(w, nil, err)
			return
		}
		if parts[3] == "progress" {
			reply(w, nil, s.S.ReportProgress(parts[2], x.Agent, x.Percent, x.Detail))
			return
		}
		if parts[3] == "retry" {
			reply(w, nil, s.S.RetryTask(parts[2]))
			return
		}
		if parts[3] == "start" {
			reply(w, nil, s.S.StartTask(parts[2], x.Agent))
			return
		}
		if parts[3] == "reject" {
			reply(w, nil, s.S.RejectTask(parts[2], x.Agent, x.Reason))
			return
		}
		v, e := s.S.UpdateTask(parts[2], parts[3], x.Reason)
		reply(w, v, e)
		return
	}
	reply(w, nil, notFound())
}
func notFound() error { return serviceError("not found") }

type serviceError string

func (e serviceError) Error() string { return string(e) }
func reply(w http.ResponseWriter, v any, e error) {
	if e != nil {
		code := http.StatusBadRequest
		if e == service.ErrNotFound || strings.Contains(e.Error(), "not found") {
			code = http.StatusNotFound
		}
		if e == service.ErrConflict || strings.Contains(e.Error(), "lease") || strings.Contains(e.Error(), "transition") {
			code = http.StatusConflict
		}
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
		return
	}
	json.NewEncoder(w).Encode(v)
}
