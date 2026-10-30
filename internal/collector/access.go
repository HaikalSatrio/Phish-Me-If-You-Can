package collector

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

// AccessOptions is opt-in. State contains host scheduling only, never queries or credentials.
type AccessOptions struct {
	StateDir          string   `json:"state_dir"`
	DNSResolver       string   `json:"dns_resolver,omitempty"`
	DNSHosts          []string `json:"dns_hosts,omitempty"`
	ClassicalTLSHosts []string `json:"classical_tls_hosts,omitempty"`
	MinIntervalMS     int      `json:"min_interval_ms"`
	SearchIntervalMS  int      `json:"search_interval_ms"`
	MaxRetries        int      `json:"max_retries"`
	MaxCooldownWaitMS int      `json:"max_cooldown_wait_ms,omitempty"`
}

func (o AccessOptions) Validate() error {
	if strings.TrimSpace(o.StateDir) == "" || o.MinIntervalMS < 0 || o.MinIntervalMS > 60000 || o.SearchIntervalMS < 0 || o.SearchIntervalMS > 60000 || o.MaxRetries < 0 || o.MaxRetries > 2 {
		return fmt.Errorf("invalid access limits or missing access.state_dir")
	}
	if o.DNSResolver != "" && o.DNSResolver != "system" && o.DNSResolver != "cloudflare" {
		return fmt.Errorf("access.dns_resolver must be system or cloudflare")
	}
	if o.MaxCooldownWaitMS < 0 || o.MaxCooldownWaitMS > 30000 {
		return fmt.Errorf("access.max_cooldown_wait_ms must be between 0 and 30000")
	}
	for _, host := range append(append([]string(nil), o.DNSHosts...), o.ClassicalTLSHosts...) {
		if host != strings.ToLower(strings.TrimSpace(host)) || !validSearchToken(host, ".-") {
			return fmt.Errorf("access host lists must contain lowercase hostnames")
		}
	}
	return nil
}

type accessKey struct{}
type accessState struct {
	Next   time.Time `json:"next"`
	Last   time.Time `json:"last,omitempty"`
	Until  time.Time `json:"until"`
	Reason string    `json:"reason,omitempty"`
}

type accessSession struct {
	opts     AccessOptions
	budget   int
	mu       sync.Mutex
	requests int
	events   []domain.AccessEvent
}

type accessError struct{ reason string }

func (e *accessError) Error() string { return e.reason }

func WithAccess(ctx context.Context, opts *AccessOptions, budget int) context.Context {
	if opts == nil {
		return ctx
	}
	return context.WithValue(ctx, accessKey{}, &accessSession{opts: *opts, budget: budget})
}

func accessFrom(ctx context.Context) *accessSession {
	s, _ := ctx.Value(accessKey{}).(*accessSession)
	return s
}

func AccessSnapshot(ctx context.Context) (int, []domain.AccessEvent, bool) {
	s := accessFrom(ctx)
	if s == nil {
		return 0, nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests, append([]domain.AccessEvent(nil), s.events...), true
}

func waitAccess(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= delay {
		return &accessError{"Provider wait exceeds remaining session deadline; no request sent"}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *accessSession) edit(ctx context.Context, host string, change func(*accessState) error) error {
	if err := os.MkdirAll(s.opts.StateDir, 0700); err != nil {
		return &accessError{"Cannot create access state directory; no request sent"}
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(host)))
	path := filepath.Join(s.opts.StateDir, key+".json")
	lock := path + ".lock"
	start := time.Now()
	for {
		file, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			file.Close()
			break
		}
		if !os.IsExist(err) || time.Since(start) > 2*time.Second {
			return &accessError{"Access state lock unavailable; no request sent"}
		}
		if err := waitAccess(ctx, 20*time.Millisecond); err != nil {
			return err
		}
	}
	defer os.Remove(lock)
	state := accessState{}
	if body, err := os.ReadFile(path); err == nil {
		if len(body) > 16384 || json.Unmarshal(body, &state) != nil {
			return &accessError{"Invalid access state; no request sent"}
		}
	} else if !os.IsNotExist(err) {
		return &accessError{"Cannot read access state; no request sent"}
	}
	if err := change(&state); err != nil {
		return err
	}
	body, _ := json.Marshal(state)
	if err := os.WriteFile(path+".tmp", body, 0600); err != nil {
		return &accessError{"Cannot save access state; no request sent"}
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		return &accessError{"Cannot commit access state; no request sent"}
	}
	return nil
}

func (s *accessSession) gate(ctx context.Context, req *http.Request) error {
	host := strings.ToLower(req.URL.Host)
	interval := time.Duration(s.opts.MinIntervalMS) * time.Millisecond
	if strings.Contains(req.URL.Path, "search") || strings.Contains(host, "duckduckgo.com") {
		interval = time.Duration(s.opts.SearchIntervalMS) * time.Millisecond
	}
	if host == "www.instagram.com" || host == "instagram.com" {
		interval = max(interval, 3*time.Second)
	}
	// Reserve a slot once. Polling Next without a reservation lets a fast process
	// repeatedly take another process's turn and exhaust its session deadline.
	var slot time.Time
	var initialCooldown time.Duration
	err := s.edit(ctx, host, func(state *accessState) error {
		now := time.Now()
		if now.Before(state.Until) {
			initialCooldown = state.Until.Sub(now)
			if initialCooldown > time.Duration(s.opts.MaxCooldownWaitMS)*time.Millisecond {
				return &accessError{"Provider cooldown until " + state.Until.UTC().Format(time.RFC3339) + ": " + state.Reason + "; no request sent"}
			}
		}
		slot = now
		if state.Next.After(slot) {
			slot = state.Next
		}
		if state.Until.After(slot) {
			slot = state.Until
		}
		if deadline, ok := ctx.Deadline(); ok && !slot.Before(deadline) {
			return &accessError{"Provider queue exceeds remaining session deadline; no request sent"}
		}
		state.Next = slot.Add(interval)
		return nil
	})
	if err != nil {
		return err
	}
	if err := waitAccess(ctx, time.Until(slot)); err != nil {
		return err
	}
	var cooldownWaited time.Duration
	cooldownWaited = initialCooldown
	for {
		var delay time.Duration
		cooling := false
		err := s.edit(ctx, host, func(state *accessState) error {
			now := time.Now()
			if now.Before(state.Until) {
				if remaining := time.Until(state.Until); remaining <= time.Duration(s.opts.MaxCooldownWaitMS)*time.Millisecond-cooldownWaited {
					delay = remaining
					cooling = true
					return nil
				}
				return &accessError{"Provider cooldown until " + state.Until.UTC().Format(time.RFC3339) + ": " + state.Reason + "; no request sent"}
			}
			// A delayed wake-up must still preserve spacing between actual starts.
			if earliest := state.Last.Add(interval); now.Before(earliest) {
				delay = earliest.Sub(now)
				return nil
			}
			state.Last = now
			if next := now.Add(interval); next.After(state.Next) {
				state.Next = next
			}
			return nil
		})
		if err != nil {
			return err
		}
		if delay <= 0 {
			return nil
		}
		started := time.Now()
		if err := waitAccess(ctx, delay); err != nil {
			return err
		}
		if cooling {
			cooldownWaited += time.Since(started)
		}
	}
}

func cooldownAccess(ctx context.Context, raw string, duration time.Duration, reason string) {
	if duration <= 0 {
		return
	}
	s := accessFrom(ctx)
	if s == nil {
		return
	}
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return
	}
	// Persist even if the request's deadline has just expired.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	err = s.edit(writeCtx, strings.ToLower(req.URL.Host), func(state *accessState) error {
		until := time.Now().Add(duration)
		if until.After(state.Until) {
			state.Until, state.Reason = until, reason
		}
		return nil
	})
	if err != nil {
		s.record(req, "STATE_ERROR", 0, "Could not persist provider cooldown", 0)
	}
}

func (s *accessSession) record(req *http.Request, outcome string, code int, reason string, elapsed time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, domain.AccessEvent{At: time.Now().UTC().Format(time.RFC3339Nano), Host: req.URL.Host,
		Outcome: outcome, HTTPStatus: code, Reason: reason, DurationMS: elapsed.Milliseconds()})
}

func retryAfter(raw string, fallback time.Duration) time.Duration {
	if seconds, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64); err == nil {
		// Saturate instead of shortening an unusually long server cooldown.
		const largest = time.Duration(1<<63 - 1)
		if seconds > uint64(largest/time.Second) {
			return largest
		}
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(raw); err == nil {
		return max(0, time.Until(date))
	}
	return fallback
}

type noAccessRetryKey struct{}

func doAccess(client *http.Client, req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	s := accessFrom(ctx)
	if s == nil {
		return client.Do(req)
	}
	if req.URL.Scheme == "https" && slices.Contains(s.opts.ClassicalTLSHosts, strings.ToLower(req.URL.Hostname())) {
		compatible, cleanup, err := classicalTLSClient(client)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		client = compatible
	}
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			s.record(req, "DEFERRED", 0, "Session cancelled before request", 0)
			return nil, err
		}
		s.mu.Lock()
		exhausted := s.requests >= s.budget
		s.mu.Unlock()
		if exhausted {
			err := &accessError{"Session request budget exhausted; no request sent"}
			s.record(req, "DEFERRED", 0, err.Error(), 0)
			return nil, err
		}
		if err := s.gate(ctx, req); err != nil {
			s.record(req, "DEFERRED", 0, err.Error(), 0)
			return nil, err
		}
		s.mu.Lock()
		if s.requests >= s.budget {
			s.mu.Unlock()
			err := &accessError{"Session request budget exhausted; no request sent"}
			s.record(req, "DEFERRED", 0, err.Error(), 0)
			return nil, err
		}
		s.requests++
		s.mu.Unlock()
		start := time.Now()
		response, err := client.Do(req)
		if err != nil {
			_, reason := networkFailure(err)
			s.record(req, "NETWORK_ERROR", 0, reason, time.Since(start))
			var cert *tls.CertificateVerificationError
			if errors.As(err, &cert) {
				cooldownAccess(ctx, req.URL.String(), 30*time.Minute, "TLS verification failed")
				return response, err
			}
			var transient net.Error
			if ctx.Value(noAccessRetryKey{}) != true && attempt < s.opts.MaxRetries && errors.As(err, &transient) && transient.Timeout() && ctx.Err() == nil {
				if waitAccess(ctx, time.Second+time.Duration(rand.IntN(300))*time.Millisecond) == nil {
					continue
				}
			}
			if ctx.Err() == nil {
				cooldownAccess(ctx, req.URL.String(), 30*time.Second, "network failure")
			}
			return response, err
		}
		s.record(req, "RESPONSE", response.StatusCode, "HTTP response received; parse/relevance are evaluated separately", time.Since(start))
		switch response.StatusCode {
		case 429:
			cooldownAccess(ctx, req.URL.String(), retryAfter(response.Header.Get("Retry-After"), 15*time.Minute), "HTTP 429 rate limit")
		case 401, 403:
			cooldownAccess(ctx, req.URL.String(), 15*time.Minute, "HTTP access restriction")
		case 202:
			if strings.Contains(req.URL.Hostname(), "duckduckgo.com") {
				cooldownAccess(ctx, req.URL.String(), 15*time.Minute, "search challenge response")
			}
		case 502, 503, 504:
			delay := retryAfter(response.Header.Get("Retry-After"), time.Second+time.Duration(rand.IntN(300))*time.Millisecond)
			cooldownAccess(ctx, req.URL.String(), delay, "temporary upstream failure")
			if ctx.Value(noAccessRetryKey{}) != true && attempt < s.opts.MaxRetries && req.Method == http.MethodGet && delay <= 30*time.Second {
				if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline)-delay > time.Second {
					closeAccessResponse(response)
					if err := waitAccess(ctx, delay+5*time.Millisecond); err != nil {
						return nil, err
					}
					continue
				}
			}
			cooldownAccess(ctx, req.URL.String(), max(delay, 30*time.Second), "temporary upstream failure")
		}
		return response, nil
	}
}

// Explicit host-scoped compatibility for paths that stall on hybrid key shares.
// Certificate/hostname validation remains intact. No process-wide TLS change.
func classicalTLSClient(client *http.Client) (*http.Client, func(), error) {
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	transport, ok := base.(*http.Transport)
	if !ok {
		return nil, nil, &accessError{"TLS compatibility requires the standard HTTP transport; no request sent"}
	}
	cloned := transport.Clone()
	config := &tls.Config{}
	if cloned.TLSClientConfig != nil {
		config = cloned.TLSClientConfig.Clone()
	}
	config.MinVersion = max(config.MinVersion, tls.VersionTLS12)
	config.CurvePreferences = []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384}
	cloned.TLSClientConfig = config
	copy := *client
	copy.Transport = cloned
	return &copy, cloned.CloseIdleConnections, nil
}

// Small bounded drain utility kept separate from response parsing.
func closeAccessResponse(r *http.Response) {
	if r != nil && r.Body != nil {
		io.Copy(io.Discard, io.LimitReader(r.Body, 4096))
		r.Body.Close()
	}
}
