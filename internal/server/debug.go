package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleDepsDebug(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	depGraph := s.depGraph
	s.mu.RUnlock()

	if depGraph == nil {
		http.Error(w, `{"error":"dependency graph unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(depGraph.Export())
}
