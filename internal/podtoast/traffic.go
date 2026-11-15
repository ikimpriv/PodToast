package podtoast

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TrafficConfig deliberately selects one IPv4 TCP port, never the entire pod.
// Delay applies once per connection; rate is per direction per connection.
// No TLS termination, certificate generation or payload storage is performed.
type TrafficConfig struct {
	Direction       string  `json:"direction"`
	Port            int     `json:"port"`
	PeerCIDR        string  `json:"peerCIDR"`
	DurationSeconds int     `json:"durationSeconds"`
	DelayMS         int     `json:"delayMS"`
	RateKiB         int     `json:"rateKiB"`
	LossPercent     float64 `json:"lossPercent"`
	ResetAfterMS    int     `json:"resetAfterMS"`
}

func (c TrafficConfig) Validate() error {
	if c.Direction != "inbound" && c.Direction != "outbound" && c.Direction != "both" {
		return fmt.Errorf("direction must be inbound, outbound or both")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("select one TCP port (1–65535)")
	}
	if c.DurationSeconds < 5 || c.DurationSeconds > 600 {
		return fmt.Errorf("duration must be 5–600 seconds")
	}
	if c.DelayMS < 0 || c.DelayMS > 30000 || c.RateKiB < 0 || c.RateKiB > 1048576 || c.ResetAfterMS < 0 || c.ResetAfterMS > 600000 {
		return fmt.Errorf("invalid delay, bandwidth or reset duration")
	}
	if !(c.LossPercent >= 0 && c.LossPercent <= 100) {
		return fmt.Errorf("packet loss must be 0–100 percent")
	}
	ip, _, e := net.ParseCIDR(c.PeerCIDR)
	if e != nil || ip.To4() == nil {
		return fmt.Errorf("peer must be an IPv4 CIDR, e.g. 0.0.0.0/0")
	}
	return nil
}

type TrafficConnection struct {
	ID                               uint64    `json:"id"`
	Client                           string    `json:"client"`
	Destination                      string    `json:"destination"`
	Opened                           time.Time `json:"opened"`
	Closed                           bool      `json:"closed"`
	Sent, Received                   uint64
	HTTPMethod, HTTPPath, HTTPStatus string
	Error                            string `json:"error,omitempty"`
}
type TrafficStatus struct {
	Active                                     bool                `json:"active"`
	Config                                     TrafficConfig       `json:"config"`
	Expires                                    time.Time           `json:"expires"`
	Connections                                []TrafficConnection `json:"connections"`
	Accepted, Rejected, Sent, Received, Resets uint64
	CleanupError                               string `json:"cleanupError,omitempty"`
	Note                                       string `json:"note"`
}
type TrafficServer struct {
	mu      sync.Mutex
	control sync.Mutex
	state   TrafficStatus
	stop    func() error
}

func (s *TrafficServer) snapshot() TrafficStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.state
	out.Connections = append([]TrafficConnection(nil), s.state.Connections...)
	return out
}
func (s *TrafficServer) Close() error {
	s.control.Lock()
	defer s.control.Unlock()
	var err error
	if s.stop != nil {
		err = s.stop()
		s.stop = nil
	}
	s.mu.Lock()
	s.state.Active = false
	if err != nil {
		s.state.CleanupError = err.Error()
	}
	s.mu.Unlock()
	return err
}
func (s *TrafficServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET" && r.URL.Path == "/v1/hello":
		reply(w, Capability{Version: Version, NetworkScope: "pod", Traffic: true})
	case r.Method == "GET" && r.URL.Path == "/v1/traffic":
		reply(w, s.snapshot())
	case r.Method == "POST" && r.URL.Path == "/v1/traffic/stop":
		if err := s.Close(); err != nil {
			fail(w, err)
		} else {
			reply(w, s.snapshot())
		}
	case r.Method == "POST" && r.URL.Path == "/v1/traffic/start":
		var c TrafficConfig
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		d.DisallowUnknownFields()
		if err := d.Decode(&c); err != nil {
			fail(w, err)
			return
		}
		if err := c.Validate(); err != nil {
			fail(w, err)
			return
		}
		s.control.Lock()
		defer s.control.Unlock()
		if s.stop != nil {
			fail(w, fmt.Errorf("stop the current experiment first"))
			return
		}
		stop, err := s.start(r.Context(), c)
		if err != nil {
			fail(w, err)
			return
		}
		s.stop = stop
		reply(w, s.snapshot())
	default:
		http.NotFound(w, r)
	}
}
func trafficWait(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// Inspect only the first HTTP/1 header in each direction. Keep no bodies,
// cookies, authorization headers or URL query strings. TLS bytes are ignored.
func trafficHTTP(prefix []byte, response bool) (method, path, status string) {
	if !strings.Contains(string(prefix), "\r\n\r\n") {
		return
	}
	line, _, _ := strings.Cut(string(prefix), "\r\n")
	p := strings.SplitN(line, " ", 3)
	if len(p) != 3 {
		return
	}
	if response {
		if strings.HasPrefix(p[0], "HTTP/1.") {
			if n, e := strconv.Atoi(p[1]); e == nil && n >= 100 && n <= 599 {
				status = p[1]
			}
		}
		return
	}
	if !strings.HasPrefix(p[2], "HTTP/1.") {
		return
	}
	switch p[0] {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE":
	default:
		return
	}
	path, _, _ = strings.Cut(p[1], "?")
	if p[0] == "CONNECT" {
		path = "(tunnel)"
	} else if parsed, err := url.ParseRequestURI(p[1]); err == nil {
		path = parsed.EscapedPath()
	}
	path = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return '\uFFFD'
		}
		return r
	}, path)
	if len(path) > 512 {
		path = path[:512]
	}
	return p[0], path, ""
}
