package server

import (
	"bufio"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"
)

// Fixed status classes avoid unbounded labels from user-controlled URLs.
type httpMetrics struct {
	completed  [6]atomic.Uint64
	inFlight   atomic.Int64
	durationNS atomic.Int64
}

func (s *Server) wrapMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__metrics" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		s.metrics.inFlight.Add(1)
		rec := &metricsWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			s.metrics.inFlight.Add(-1)
			if elapsed := time.Since(start); elapsed > 0 {
				s.metrics.durationNS.Add(elapsed.Nanoseconds())
			}
			class := rec.status / 100
			if class > 5 {
				class = 0
			}
			s.metrics.completed[class].Add(1)
		}()
		next.ServeHTTP(rec, r)
	})
}

type metricsWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *metricsWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *metricsWriter) WriteHeader(code int) {
	if !w.wroteHeader && code >= 200 {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}
func (w *metricsWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}
func (w *metricsWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *metricsWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}
func (w *metricsWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

func (s *Server) metricsHandler(token string) http.Handler {
	expected := sha256.Sum256([]byte("Bearer " + token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if token == "" {
			http.NotFound(w, r)
			return
		}
		supplied := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if subtle.ConstantTimeCompare(expected[:], supplied[:]) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		if r.Method == http.MethodHead {
			return
		}
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		write := func(format string, args ...any) bool {
			_, err := fmt.Fprintf(w, format, args...)
			return err == nil
		}
		if !write("# HELP foundry_http_requests_total Completed HTTP requests excluding metrics scrapes.\n# TYPE foundry_http_requests_total counter\n") {
			return
		}
		if !write("foundry_http_requests_total{status_class=\"other\"} %d\n", s.metrics.completed[0].Load()) {
			return
		}
		for class := 1; class <= 5; class++ {
			if !write("foundry_http_requests_total{status_class=\"%dxx\"} %d\n", class, s.metrics.completed[class].Load()) {
				return
			}
		}
		if !write("# HELP foundry_http_request_duration_seconds_total Total HTTP request duration excluding metrics scrapes.\n# TYPE foundry_http_request_duration_seconds_total counter\nfoundry_http_request_duration_seconds_total %g\n", float64(s.metrics.durationNS.Load())/float64(time.Second)) {
			return
		}
		if !write("# HELP foundry_http_requests_in_flight Active HTTP requests excluding metrics scrapes.\n# TYPE foundry_http_requests_in_flight gauge\nfoundry_http_requests_in_flight %d\n", s.metrics.inFlight.Load()) {
			return
		}
		if !write("# HELP foundry_go_goroutines Current goroutines.\n# TYPE foundry_go_goroutines gauge\nfoundry_go_goroutines %d\n", runtime.NumGoroutine()) {
			return
		}
		if !write("# HELP foundry_go_heap_alloc_bytes Allocated heap bytes.\n# TYPE foundry_go_heap_alloc_bytes gauge\nfoundry_go_heap_alloc_bytes %d\n", mem.HeapAlloc) {
			return
		}
		if !write("# HELP foundry_go_gc_cycles_total Completed garbage collection cycles.\n# TYPE foundry_go_gc_cycles_total counter\nfoundry_go_gc_cycles_total %d\n", mem.NumGC) {
			return
		}
	})
}
