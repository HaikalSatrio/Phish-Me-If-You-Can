package collector

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

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
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !publicIP(ip) {
				return nil, fmt.Errorf("non-public DNS destination")
			}
		}
		for _, ip := range ips {
			conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
		}
		return nil, fmt.Errorf("public destination unreachable")
	}
	return &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}

func readPublic(ctx context.Context, client *http.Client, raw string, headers map[string]string) ([]byte, string, string, string) {
	if _, err := cleanURL(raw); err != nil {
		return nil, "BLOCKED", "URL is not an allowed public destination", ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, "FAILED", "Invalid request", ""
	}
	req.Header.Set("User-Agent", "PMIYC/0.3 public-exposure-research")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "FAILED", "Request failed, was cancelled, or destination was not public", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, "EMPTY", "Public resource not found", ""
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 429 || (resp.StatusCode >= 300 && resp.StatusCode < 400) {
		return nil, "BLOCKED", fmt.Sprintf("HTTP %d; access restriction or redirect not followed", resp.StatusCode), ""
	}
	if resp.StatusCode != 200 {
		return nil, "FAILED", fmt.Sprintf("HTTP %d", resp.StatusCode), ""
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil || len(b) > 2<<20 {
		return nil, "FAILED", "Unreadable or oversized response", ""
	}
	return b, "SUCCESS", "Public response retrieved", resp.Header.Get("Content-Type")
}
