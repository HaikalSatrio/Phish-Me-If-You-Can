package collector

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var errNonPublicDNS = errors.New("non-public DNS destination")

type dialResult struct {
	conn net.Conn
	err  error
}

// Race already validated addresses so an unreachable IPv6 route cannot consume the whole request timeout.
func dialCheckedIPs(ctx context.Context, network, port string, ips []netip.Addr, dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	if len(ips) == 0 {
		return nil, &net.DNSError{Err: "no addresses returned", IsNotFound: true}
	}
	for _, ip := range ips {
		if !publicIP(ip) {
			return nil, errNonPublicDNS
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan dialResult)
	for i, ip := range ips {
		go func(delay time.Duration, address string) {
			if delay > 0 {
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				}
			}
			conn, err := dial(ctx, network, address)
			select {
			case results <- dialResult{conn, err}:
			case <-ctx.Done():
				if conn != nil {
					conn.Close()
				}
			}
		}(time.Duration(i)*250*time.Millisecond, net.JoinHostPort(ip.String(), port))
	}
	var lastErr error
	for range ips {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-results:
			if result.err == nil {
				return result.conn, nil
			}
			lastErr = result.err
		}
	}
	return nil, fmt.Errorf("public destination unreachable: %w", lastErr)
}

func networkFailure(err error) (string, string) {
	var dnsError *net.DNSError
	var certificateError *tls.CertificateVerificationError
	var networkError net.Error
	var operationError *net.OpError
	var deferred *accessError
	switch {
	case errors.As(err, &deferred):
		return "BLOCKED", deferred.Error()
	case errors.Is(err, errNonPublicDNS):
		return "BLOCKED", "DNS returned a non-public address; destination was not fetched"
	case errors.Is(err, context.Canceled):
		return "FAILED", "Request cancelled"
	case errors.As(err, &dnsError):
		return "FAILED", "DNS lookup failed; check the network resolver"
	case errors.As(err, &certificateError):
		return "FAILED", "TLS certificate verification failed"
	case errors.As(err, &networkError) && networkError.Timeout():
		return "FAILED", "Request timed out"
	case errors.As(err, &operationError):
		return "FAILED", "Connection failed after DNS resolution; check network access to the source"
	default:
		return "FAILED", "Network request failed"
	}
}

func publicIP(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return false
	}
	for _, s := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "64:ff9b::/96", "2002::/16"} {
		if netip.MustParsePrefix(s).Contains(a) {
			return false
		}
	}
	return true
}

func cleanURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || (u.Port() != "" && u.Port() != "443" && u.Port() != "80") {
		return nil, fmt.Errorf("only public HTTP(S) URLs on standard ports are supported")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return nil, fmt.Errorf("local URLs are not supported")
	}
	if a, err := netip.ParseAddr(host); err == nil && !publicIP(a) {
		return nil, fmt.Errorf("non-public address")
	}
	u.Fragment = ""
	return u, nil
}

func publicClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{Proxy: nil, MaxIdleConns: 10, IdleConnTimeout: 30 * time.Second, TLSHandshakeTimeout: timeout, ResponseHeaderTimeout: timeout}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := lookupPublicIPs(ctx, host)
		if err != nil {
			return nil, err
		}
		return dialCheckedIPs(ctx, network, port, ips, (&net.Dialer{Timeout: timeout}).DialContext)
	}
	return &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}

func readPublic(ctx context.Context, client *http.Client, raw string, headers map[string]string) ([]byte, string, string, string) {
	body, status, detail, content, _ := readPublicOnce(ctx, client, raw, headers)
	return body, status, detail, content
}

func readPublicOnce(ctx context.Context, client *http.Client, raw string, headers map[string]string) ([]byte, string, string, string, string) {
	if _, err := cleanURL(raw); err != nil {
		return nil, "BLOCKED", "URL is not an allowed public destination", "", ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, "FAILED", "Invalid request", "", ""
	}
	req.Header.Set("User-Agent", "PMIYC/0.3 public-exposure-research")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := doAccess(client, req)
	if err != nil {
		status, detail := networkFailure(err)
		return nil, status, detail, "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, "EMPTY", "Public resource not found", "", ""
	}
	if resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 303 || resp.StatusCode == 307 || resp.StatusCode == 308 {
		return nil, "BLOCKED", fmt.Sprintf("HTTP %d redirect", resp.StatusCode), "", resp.Header.Get("Location")
	}
	if resp.StatusCode == 429 {
		return nil, "BLOCKED", "HTTP 429 rate limit; retry later", "", ""
	}
	if resp.StatusCode == 401 {
		return nil, "BLOCKED", "HTTP 401 authentication required", "", ""
	}
	if resp.StatusCode == 403 || (resp.StatusCode >= 300 && resp.StatusCode < 400) {
		return nil, "BLOCKED", fmt.Sprintf("HTTP %d; access restriction or redirect not followed", resp.StatusCode), "", ""
	}
	if resp.StatusCode != 200 {
		return nil, "FAILED", fmt.Sprintf("HTTP %d", resp.StatusCode), "", ""
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil || len(b) > 2<<20 {
		return nil, "FAILED", "Unreadable or oversized response", "", ""
	}
	return b, "SUCCESS", "Public response retrieved", resp.Header.Get("Content-Type"), ""
}

type publicResponse struct {
	Body                         []byte
	URL, Status, Detail, Content string
	Requests                     int
}

// Each redirect is validated before fetching and consumes the caller's budget.
func fetchPublic(ctx context.Context, client *http.Client, raw string, budget int, allowRedirect func(*url.URL) bool) publicResponse {
	r := publicResponse{URL: raw, Status: "FAILED", Detail: "Request budget exhausted"}
	visited := map[string]bool{}
	for r.Requests < budget && r.Requests < 5 {
		if ctx.Err() != nil {
			r.Detail = "Request cancelled or session timed out"
			return r
		}
		u, err := cleanURL(r.URL)
		if err != nil {
			r.Status, r.Detail = "BLOCKED", "URL is not an allowed public destination"
			return r
		}
		visited[u.String()] = true
		var location string
		r.Body, r.Status, r.Detail, r.Content, location = readPublicOnce(ctx, client, u.String(), nil)
		r.Requests++
		if location == "" {
			return r
		}
		next, err := url.Parse(location)
		if err != nil {
			r.Status, r.Detail = "FAILED", "Invalid redirect URL"
			return r
		}
		next, err = cleanURL(u.ResolveReference(next).String())
		if err != nil || (u.Scheme == "https" && next.Scheme != "https") || (allowRedirect != nil && !allowRedirect(next)) {
			r.Status, r.Detail = "BLOCKED", "Redirect destination is outside the allowed public scope"
			return r
		}
		if visited[next.String()] {
			r.Status, r.Detail = "FAILED", "Redirect loop detected"
			return r
		}
		r.URL = next.String()
	}
	r.Status, r.Detail = "FAILED", "Redirect limit or request budget reached"
	return r
}
