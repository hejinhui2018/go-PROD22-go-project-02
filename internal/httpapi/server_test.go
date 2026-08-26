package httpapi

import (
	"fleetforge/internal/ports"
	"fleetforge/internal/service"
	"fleetforge/internal/store"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	s := service.New(&store.MemoryStore{}, ports.RealClock{}, &ports.SequenceID{})
	r := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/healthz", nil)
	(&Server{S: s}).Handler().ServeHTTP(r, req)
	if r.Code != 200 {
		t.Fatal(r.Code)
	}
}
