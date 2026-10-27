//go:build live

package collector

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Explicit live diagnostic, excluded from normal tests. Uses the same shared cooldown.
func TestBingTransportProbe(t *testing.T) {
	if os.Getenv("PMIYC_LIVE_ACCESS") != "1" {
		t.Skip("requires explicit live network opt-in")
	}
	for _, h2 := range []bool{true, false} {
		ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
		ctx = WithAccess(ctx, &AccessOptions{StateDir: filepath.Join("..", "..", "data", "access-state"), SearchIntervalMS: 6000, MaxCooldownWaitMS: 30000}, 1)
		start := time.Now()
		var mu sync.Mutex
		var phases []map[string]any
		add := func(phase string, detail any) {
			mu.Lock()
			defer mu.Unlock()
			phases = append(phases, map[string]any{"phase": phase, "ms": time.Since(start).Milliseconds(), "detail": detail})
		}
		trace := &httptrace.ClientTrace{
			TLSHandshakeStart: func() { add("tls_start", true) },
			ConnectStart:      func(_, address string) { add("connect_start", address) },
			ConnectDone:       func(_, _ string, err error) { add("connect_done", err == nil) },
			TLSHandshakeDone: func(state tls.ConnectionState, err error) {
				add("tls_done", map[string]any{"ok": err == nil, "alpn": state.NegotiatedProtocol})
			},
			WroteRequest:         func(info httptrace.WroteRequestInfo) { add("request_written", info.Err == nil) },
			GotFirstResponseByte: func() { add("first_response_byte", true) },
		}
		ctx = httptrace.WithClientTrace(ctx, trace)
		c := NewBing(15 * time.Second)
		c.client.Transport.(*http.Transport).ForceAttemptHTTP2 = h2
		classical := os.Getenv("PMIYC_PROBE_CLASSICAL_TLS") == "1"
		if classical {
			c.client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384}}
		}
		result := c.Collect(ctx, Query{Name: "Simon Willison", Limit: 5, Budget: 1})
		count, events, _ := AccessSnapshot(ctx)
		mu.Lock()
		body, _ := json.Marshal(map[string]any{"classical_tls": classical, "http2_enabled": h2, "phases": phases, "requests": count, "access": events, "status": result.Status, "events": result.Events, "profiles": len(result.Profiles)})
		mu.Unlock()
		t.Log(string(body))
		c.client.CloseIdleConnections()
		cancel()
	}
}
