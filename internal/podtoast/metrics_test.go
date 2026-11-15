package podtoast

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsValuesAndUnavailableLimits(t *testing.T) {
	snap := Snapshot{Resources: Section{Value: map[string]any{
		"cpu.stat": Section{Value: "usage_usec 2500000\nthrottled_usec 100000\nnr_throttled 3"},
		"cpu.max":  Section{Value: "150000 100000"}, "memory.current": Section{Value: "1234"}, "memory.max": Section{Value: "max"},
	}}, Storage: Section{Unavailable: "denied"}, Network: Section{Value: map[string]any{"dev": Section{Value: "eth0: 12 0 0 0 0 0 0 0 34 0 0 0 0 0 0 0"}}}}
	var out bytes.Buffer
	writeMetrics(&out, snap)
	for _, want := range []string{"podtoast_cpu_usage_seconds_total 2.5\n", "podtoast_cpu_limit_cores 1.5\n", "podtoast_memory_usage_bytes 1234\n", "podtoast_storage_available 0\n", "podtoast_pod_network_receive_bytes_total 12\n", "# TYPE podtoast_cpu_usage_seconds_total counter"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "podtoast_memory_limit_bytes") {
		t.Fatal("unlimited must not become zero")
	}
}
func TestMetricsEndpointIsolationAndConcurrency(t *testing.T) {
	s := &Server{Root: t.TempDir()}
	for _, tt := range []struct {
		method, path string
		status       int
	}{{"GET", "/metrics", 200}, {"POST", "/metrics", 405}, {"GET", "/v1/read?path=/secret", 404}} {
		w := httptest.NewRecorder()
		s.MetricsHandler().ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
		if w.Code != tt.status {
			t.Fatal(tt, w.Code)
		}
	}
	s.metricsMu.Lock()
	w := httptest.NewRecorder()
	s.MetricsHandler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	s.metricsMu.Unlock()
	if w.Code != http.StatusServiceUnavailable {
		t.Fatal(w.Code)
	}
}
func TestMetricsOverExec(t *testing.T) {
	left, right := net.Pipe()
	go Serve(right, &Server{Root: t.TempDir()})
	c, err := Connect(context.Background(), left)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	b, err := c.Metrics(context.Background())
	if err != nil || !strings.Contains(string(b), "podtoast_resources_available") {
		t.Fatal(string(b), err)
	}
}
