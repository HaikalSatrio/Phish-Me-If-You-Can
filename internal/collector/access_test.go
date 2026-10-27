package collector

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func accessRequest(ctx context.Context) *http.Request {
	r, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.org/search?token=secret", nil)
	return r
}

func accessResponse(code int, retry string) *http.Response {
	h := http.Header{}
	if retry != "" {
		h.Set("Retry-After", retry)
	}
	return &http.Response{StatusCode: code, Header: h, Body: io.NopCloser(strings.NewReader("{}"))}
}

func TestAccess429SurvivesNewSessionAndDoesNotRetry(t *testing.T) {
	opts := &AccessOptions{StateDir: t.TempDir(), MaxRetries: 2}
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return accessResponse(429, "120"), nil })}
	first := WithAccess(context.Background(), opts, 5)
	response, err := doAccess(client, accessRequest(first))
	if err != nil || response.StatusCode != 429 || calls != 1 {
		t.Fatalf("unexpected first request: %v %d", err, calls)
	}
	closeAccessResponse(response)
	second := WithAccess(context.Background(), opts, 5)
	_, err = doAccess(client, accessRequest(second))
	var blocked *accessError
	count, events, _ := AccessSnapshot(second)
	if !errors.As(err, &blocked) || calls != 1 || count != 0 || len(events) != 1 || events[0].Outcome != "DEFERRED" {
		t.Fatalf("rate limit ignored: %v %+v", err, events)
	}
}

func TestAccessRetryCountsAgainstActualBudget(t *testing.T) {
	for _, budget := range []int{1, 2} {
		ctx := WithAccess(context.Background(), &AccessOptions{StateDir: t.TempDir(), MaxRetries: 1}, budget)
		calls := 0
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return accessResponse(503, "0"), nil
			}
			return accessResponse(200, ""), nil
		})}
		r, err := doAccess(client, accessRequest(ctx))
		count, _, _ := AccessSnapshot(ctx)
		if count != budget || calls != budget || budget == 2 && (err != nil || r.StatusCode != 200) || budget == 1 && err == nil {
			t.Fatalf("budget=%d calls=%d count=%d err=%v", budget, calls, count, err)
		}
		closeAccessResponse(r)
	}
}

func TestAccessLongRetryAfterPreservesDeadlineAndCooldown(t *testing.T) {
	opts := &AccessOptions{StateDir: t.TempDir(), MaxRetries: 2}
	deadline, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	ctx := WithAccess(deadline, opts, 5)
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return accessResponse(503, "120"), nil })}
	r, err := doAccess(client, accessRequest(ctx))
	closeAccessResponse(r)
	if err != nil || calls != 1 {
		t.Fatal("did not return original temporary failure")
	}
	_, err = doAccess(client, accessRequest(WithAccess(context.Background(), opts, 5)))
	if err == nil || calls != 1 {
		t.Fatal("Retry-After lost across sessions")
	}
}

func TestAccessTLSFailureIsNotRetriedAndSecretsStayOutOfTrace(t *testing.T) {
	ctx := WithAccess(context.Background(), &AccessOptions{StateDir: t.TempDir(), MaxRetries: 2}, 5)
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, &tls.CertificateVerificationError{Err: x509.HostnameError{Host: "private-token-secret"}}
	})}
	_, err := doAccess(client, accessRequest(ctx))
	_, events, _ := AccessSnapshot(ctx)
	if err == nil || calls != 1 || len(events) != 1 || strings.Contains(events[0].Reason, "secret") {
		t.Fatalf("unsafe TLS handling: %+v", events)
	}
}

func TestAccessSpacingSharedByIndependentSessions(t *testing.T) {
	opts := &AccessOptions{StateDir: t.TempDir(), SearchIntervalMS: 60}
	var mu sync.Mutex
	var times []time.Time
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		mu.Lock()
		times = append(times, time.Now())
		mu.Unlock()
		return accessResponse(200, ""), nil
	})}
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			r, err := doAccess(client, accessRequest(WithAccess(ctx, opts, 2)))
			closeAccessResponse(r)
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(times) != 3 {
		t.Fatal(times)
	}
	for i := 1; i < len(times); i++ {
		if times[i].Sub(times[i-1]) < 45*time.Millisecond {
			t.Fatal("requests started too close together", times)
		}
	}
}

func TestHTTPSDNSRejectsNonPublicAndMalformedAnswers(t *testing.T) {
	for _, body := range []string{`{"Status":0,"Answer":[{"type":1,"data":"127.0.0.1"}]}`, `{"Status":0,"Answer":[{"type":1,"data":"1.1.1.1"},{"type":1,"data":"10.0.0.1"}]}`, `{"Status":3}`, `not json`} {
		if _, err := parseHTTPSAddresses([]byte(body), "example.org"); err == nil {
			t.Fatal("unsafe answer accepted", body)
		}
	}
	ips, err := parseHTTPSAddresses([]byte(`{"Status":0,"Answer":[{"type":5,"data":"alias.example"},{"type":1,"data":"1.1.1.1"}]}`), "example.org")
	if err != nil || len(ips) != 1 || ips[0].String() != "1.1.1.1" {
		t.Fatal(ips, err)
	}
}

func TestRetryAfterSupportsHTTPDate(t *testing.T) {
	date := time.Now().Add(2 * time.Minute).UTC().Format(http.TimeFormat)
	if wait := retryAfter(date, time.Second); wait < 119*time.Second || wait > 120*time.Second {
		t.Fatal(wait)
	}
	if retryAfter("invalid", 17*time.Second) != 17*time.Second {
		t.Fatal("fallback lost")
	}
}

func TestAccessWaitsForShortCooldownButHonorsDeadline(t *testing.T) {
	for _, shortDeadline := range []bool{false, true} {
		opts := &AccessOptions{StateDir: t.TempDir(), MaxCooldownWaitMS: 1000}
		ctx := WithAccess(context.Background(), opts, 2)
		cooldownAccess(ctx, "https://example.org", 100*time.Millisecond, "temporary upstream failure")
		if shortDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, 20*time.Millisecond)
			defer cancel()
		}
		calls := 0
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return accessResponse(200, ""), nil
		})}
		start := time.Now()
		r, err := doAccess(client, accessRequest(ctx))
		closeAccessResponse(r)
		if shortDeadline {
			if err == nil || calls != 0 {
				t.Fatalf("deadline ignored: %v calls=%d", err, calls)
			}
		} else if err != nil || calls != 1 || time.Since(start) < 80*time.Millisecond {
			t.Fatalf("cooldown ignored: %v calls=%d", err, calls)
		}
	}
}

func TestHTTPSDNSHostScopePreservesSystemLiteralLookup(t *testing.T) {
	ctx := WithAccess(context.Background(), &AccessOptions{StateDir: t.TempDir(), DNSResolver: "cloudflare", DNSHosts: []string{"html.duckduckgo.com"}}, 1)
	ips, err := lookupPublicIPs(ctx, "127.0.0.1")
	if err != nil || len(ips) != 1 || !ips[0].IsLoopback() {
		t.Fatal(ips, err)
	}
	// Scope does not relax the public-address guard used by the actual dialer.
	if publicIP(ips[0]) {
		t.Fatal("loopback accepted for public access")
	}
}

func TestRetryAfterHugeValueDoesNotOverflow(t *testing.T) {
	if wait := retryAfter("18446744073709551615", time.Second); wait < 24*time.Hour {
		t.Fatalf("large cooldown shortened: %v", wait)
	}
}

func TestClassicalTLSKeepsCertificateVerificationAndOriginalTransport(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer server.Close()
	for _, trusted := range []bool{false, true} {
		client := &http.Client{Timeout: time.Second}
		if trusted {
			client = server.Client()
		}
		original := client.Transport
		compatible, cleanup, err := classicalTLSClient(client)
		if err != nil {
			t.Fatal(err)
		}
		transport := compatible.Transport.(*http.Transport)
		if transport.TLSClientConfig.InsecureSkipVerify || transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
			t.Fatal("TLS validation weakened")
		}
		response, err := compatible.Get(server.URL)
		closeAccessResponse(response)
		cleanup()
		if trusted && err != nil || !trusted && err == nil {
			t.Fatalf("trusted=%v err=%v", trusted, err)
		}
		if client.Transport != original {
			t.Fatal("caller transport changed")
		}
		if original != nil && len(original.(*http.Transport).TLSClientConfig.CurvePreferences) != 0 {
			t.Fatal("original TLS config changed")
		}
	}
}

func TestAccessExtendedCooldownDoesNotRestartWaitBudget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ctx = WithAccess(ctx, &AccessOptions{StateDir: t.TempDir(), MaxCooldownWaitMS: 100}, 1)
	cooldownAccess(ctx, "https://example.org", 70*time.Millisecond, "first cooldown")
	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(30 * time.Millisecond)
		cooldownAccess(ctx, "https://example.org", 140*time.Millisecond, "extended cooldown")
	}()
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return accessResponse(200, ""), nil })}
	r, err := doAccess(client, accessRequest(ctx))
	closeAccessResponse(r)
	<-done
	if err == nil || calls != 0 {
		t.Fatal("renewed cooldown bypassed cumulative wait budget", err, calls)
	}
}

func TestAccessQueuedSessionGetsTurnBeforeFastSessionRepeats(t *testing.T) {
	opts := &AccessOptions{StateDir: t.TempDir(), SearchIntervalMS: 100}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var mu sync.Mutex
	var order []string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		order = append(order, r.Header.Get("X-Test-Caller"))
		mu.Unlock()
		return accessResponse(200, ""), nil
	})}
	call := func(label string) {
		r := accessRequest(WithAccess(ctx, opts, 1))
		r.Header.Set("X-Test-Caller", label)
		response, err := doAccess(client, r)
		closeAccessResponse(response)
		if err != nil {
			t.Error(err)
		}
	}
	call("first")
	done := make(chan struct{})
	go func() { defer close(done); call("queued") }()
	// Wait for the queued request's reservation before submitting a later one.
	probe := accessFrom(WithAccess(ctx, opts, 1))
	reserved := false
	for !reserved && ctx.Err() == nil {
		if err := probe.edit(ctx, "example.org", func(state *accessState) error {
			reserved = state.Next.Sub(state.Last) >= 190*time.Millisecond
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if !reserved {
			time.Sleep(time.Millisecond)
		}
	}
	if !reserved {
		t.Fatal("queued request did not reserve a turn")
	}
	call("repeat")
	<-done
	if strings.Join(order, ",") != "first,queued,repeat" {
		t.Fatal("queued process lost its turn", order)
	}
}
