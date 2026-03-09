package server

import (
	"net/http"

	"github.com/sphireinc/foundry/internal/feed"
)

func (s *Server) handleRSS(w http.ResponseWriter, r *http.Request) {
	_ = r
	s.mu.RLock()
	graph := s.graph
	s.mu.RUnlock()

	if graph == nil {
		http.Error(w, "site graph unavailable", http.StatusServiceUnavailable)
		return
	}

	payload, err := feed.BuildRSS(s.cfg, graph)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	_, _ = w.Write(payload)
}

func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	_ = r
	s.mu.RLock()
	graph := s.graph
	s.mu.RUnlock()

	if graph == nil {
		http.Error(w, "site graph unavailable", http.StatusServiceUnavailable)
		return
	}

	payload, err := feed.BuildSitemap(s.cfg, graph)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write(payload)
}
