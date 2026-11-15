//go:build linux

package podtoast

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func trafficTestClient(t *testing.T) (*Client, *exec.Cmd) {
	t.Helper()
	cmd := exec.Command("/podtoast", "traffic")
	in, e := cmd.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	out, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	cmd.Stderr = os.Stderr
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	conn := &StreamConn{Reader: out, Writer: in, CloseFunc: func() error { out.Close(); return in.Close() }}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, e := Connect(ctx, conn)
	if e != nil {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close(); cmd.Wait() })
	return c, cmd
}
func TestTrafficNetworkNamespace(t *testing.T) {
	if os.Getenv("PODTOAST_TRAFFIC_TEST") != "1" {
		t.Skip("isolated NET_ADMIN container only")
	}
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			time.Sleep(300 * time.Millisecond)
		}
		if r.URL.Path == "/bulk" {
			io.WriteString(w, strings.Repeat("x", 32<<10))
			return
		}
		io.WriteString(w, "hello")
	})}
	go server.Serve(listener)
	defer server.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	endpoint := "http://" + listener.Addr().String() + "/hello?token=private"
	request := func() (time.Duration, error) {
		start := time.Now()
		client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
		resp, e := client.Get(endpoint)
		if e == nil {
			_, e = io.ReadAll(resp.Body)
			resp.Body.Close()
		}
		return time.Since(start), e
	}
	if b, e := exec.Command("iptables", "-t", "nat", "-N", "USER_KEEP").CombinedOutput(); e != nil {
		t.Fatalf("NET_ADMIN fixture: %v %s", e, b)
	}
	defer exec.Command("iptables", "-t", "nat", "-X", "USER_KEEP").Run()
	c, cmd := trafficTestClient(t)
	cfg := TrafficConfig{Direction: "outbound", Port: port, PeerCIDR: "127.0.0.0/8", DurationSeconds: 20, DelayMS: 180}
	ctx := context.Background()
	var state TrafficStatus
	if e = c.JSON(ctx, "POST", "/v1/traffic/start", cfg, &state); e != nil {
		t.Fatal(e)
	}
	elapsed, e := request()
	if e != nil || elapsed < 160*time.Millisecond {
		t.Fatalf("delay/proxy: %v %v", elapsed, e)
	}
	time.Sleep(60 * time.Millisecond)
	if e = c.JSON(ctx, "GET", "/v1/traffic", nil, &state); e != nil || state.Accepted != 1 || state.Sent == 0 || state.Received == 0 || len(state.Connections) != 1 || state.Connections[0].HTTPPath != "/hello" || state.Connections[0].HTTPStatus != "200" {
		t.Fatalf("stats: %+v %v", state, e)
	}
	// A second process in the same network namespace must not install rules.
	second, _ := trafficTestClient(t)
	if e = second.JSON(ctx, "POST", "/v1/traffic/start", cfg, &state); e == nil {
		t.Fatal("overlapping session accepted")
	}
	second.Close()
	// Kill the controller, not a graceful stop. The independent watchdog removes rules.
	if e = cmd.Process.Kill(); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		b, _ := exec.Command("iptables", "-t", "nat", "-S").CombinedOutput()
		if !strings.Contains(string(b), "PT_") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("watchdog left rules: %s", b)
		}
		time.Sleep(100 * time.Millisecond)
	}
	elapsed, e = request()
	if e != nil || elapsed > 160*time.Millisecond {
		t.Fatalf("restored: %v %v", elapsed, e)
	}
	third, _ := trafficTestClient(t)
	cfg.DelayMS = 0
	cfg.LossPercent = 100
	if e = third.JSON(ctx, "POST", "/v1/traffic/start", cfg, &state); e != nil {
		t.Fatal(e)
	}
	if _, e = request(); e == nil {
		t.Fatal("100 percent packet loss did not drop packets")
	}
	if e = third.JSON(ctx, "POST", "/v1/traffic/stop", nil, &state); e != nil {
		t.Fatal(e)
	}
	if _, e = request(); e != nil {
		t.Fatal("stop did not restore traffic", e)
	}
	cfg.LossPercent = 0
	cfg.RateKiB = 64
	endpoint = "http://" + listener.Addr().String() + "/bulk"
	if e = third.JSON(ctx, "POST", "/v1/traffic/start", cfg, &state); e != nil {
		t.Fatal(e)
	}
	elapsed, e = request()
	if e != nil || elapsed < 450*time.Millisecond {
		t.Fatalf("rate: %v %v", elapsed, e)
	}
	if e = third.JSON(ctx, "POST", "/v1/traffic/stop", nil, &state); e != nil {
		t.Fatal(e)
	}
	cfg.RateKiB = 0
	cfg.ResetAfterMS = 40
	endpoint = "http://" + listener.Addr().String() + "/slow"
	if e = third.JSON(ctx, "POST", "/v1/traffic/start", cfg, &state); e != nil {
		t.Fatal(e)
	}
	if _, e = request(); e == nil {
		t.Fatal("reset did not break slow response")
	}
	if e = third.JSON(ctx, "POST", "/v1/traffic/stop", nil, &state); e != nil {
		t.Fatal(e)
	}
	cfg.ResetAfterMS = 0
	cfg.DurationSeconds = 5
	if e = third.JSON(ctx, "POST", "/v1/traffic/start", cfg, &state); e != nil {
		t.Fatal(e)
	}
	time.Sleep(5500 * time.Millisecond)
	if e = third.JSON(ctx, "GET", "/v1/traffic", nil, &state); e != nil || state.Active || state.CleanupError != "" {
		t.Fatalf("deadline: %+v %v", state, e)
	}
	if b, e := exec.Command("iptables", "-t", "nat", "-S", "USER_KEEP").CombinedOutput(); e != nil {
		t.Fatalf("unrelated chain changed: %v %s", e, b)
	}

}
