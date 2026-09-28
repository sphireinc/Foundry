package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsEndpoint(t *testing.T) {
	s := &Server{}
	for _, tc := range []struct {
		token, auth, method string
		code                int
	}{
		{"", "", "GET", 404},
		{"secret", "", "GET", 401},
		{"secret", "Bearer wrong", "GET", 401},
		{"secret", "Bearer secret", "POST", 405},
		{"secret", "Bearer secret", "HEAD", 200},
		{"secret", "Bearer secret", "GET", 200},
	} {
		r := httptest.NewRequest(tc.method, "/__metrics", nil)
		r.Header.Set("Authorization", tc.auth)
		w := httptest.NewRecorder()
		s.metricsHandler(tc.token).ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%+v: status %d", tc, w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("metrics must not be cached")
		}
		if tc.method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD body")
		}
		if tc.method == "GET" && tc.code == 200 {
			if !strings.Contains(w.Body.String(), "# TYPE foundry_http_requests_total counter") {
				t.Fatal(w.Body.String())
			}
			if strings.Contains(w.Body.String(), "secret") {
				t.Fatal("token leaked")
			}
		}
	}
}

func TestMetricsRecordRequests(t *testing.T) {
	s := &Server{}
	handler := s.wrapMetrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__metrics" {
			s.metricsHandler("secret").ServeHTTP(w, r)
			return
		}
		if s.metrics.inFlight.Load() != 1 {
			t.Error("missing in-flight request")
		}
		w.WriteHeader(103)
		w.WriteHeader(404)
		w.WriteHeader(500) // only the first final status counts
		_, _ = w.Write([]byte("missing"))
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/private-user-path?secret=hidden", nil))
	if s.metrics.completed[4].Load() != 1 || s.metrics.completed[5].Load() != 0 || s.metrics.inFlight.Load() != 0 || s.metrics.durationNS.Load() == 0 {
		t.Fatal("incorrect metrics")
	}
	r := httptest.NewRequest("GET", "/__metrics", nil)
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if strings.Contains(w.Body.String(), "private-user") || strings.Contains(w.Body.String(), "hidden") {
		t.Fatal("unbounded or sensitive labels")
	}
	if s.metrics.completed[2].Load() != 0 {
		t.Fatal("scrape counted")
	}
}

func TestMetricsMuxAndStreaming(t *testing.T) {
	t.Setenv("FOUNDRY_METRICS_TOKEN", "secret")
	cfg := testServerConfig(t)
	s := New(cfg, stubLoader{}, nil, nil, &hookRecorder{}, false)
	w := httptest.NewRecorder()
	s.newMux().ServeHTTP(w, httptest.NewRequest("GET", "/hook", nil))
	if w.Code != 200 || s.metrics.completed[2].Load() != 1 {
		t.Fatal("mux not instrumented")
	}
	r := httptest.NewRequest("GET", "/__metrics", nil)
	r.Header.Set("Authorization", "Bearer secret")
	w = httptest.NewRecorder()
	s.newMux().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	streamed := s.wrapMetrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Error(err)
		}
		w.WriteHeader(500)
	}))
	streamed.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/stream", nil))
	if s.metrics.completed[2].Load() != 2 {
		t.Fatal("flush did not record implicit 200")
	}
	t.Setenv("FOUNDRY_METRICS_TOKEN", "")
	w = httptest.NewRecorder()
	s.newMux().ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal("disabled endpoint accessible")
	}
}
