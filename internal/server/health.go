package server

import (
	"encoding/json"
	"net/http"
	"time"

	foundryversion "github.com/sphireinc/foundry/internal/commands/version"
	"github.com/sphireinc/foundry/internal/managed"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	meta := foundryversion.Current("")
	report := managed.BuildHealthReport(s.cfg, managed.HealthVersion{
		Version: meta.DisplayVersion,
		Commit:  meta.Commit,
	}, timeNow())
	statusCode := http.StatusOK
	if report.Status != managed.HealthStatusHealthy {
		statusCode = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(statusCode)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(report)
}

var timeNow = func() time.Time {
	return time.Now().UTC()
}
