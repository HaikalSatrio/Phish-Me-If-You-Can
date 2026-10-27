package collector

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"time"
)

func lookupPublicIPs(ctx context.Context, host string) ([]netip.Addr, error) {
	s := accessFrom(ctx)
	if s == nil || s.opts.DNSResolver != "cloudflare" || len(s.opts.DNSHosts) > 0 && !slices.Contains(s.opts.DNSHosts, strings.ToLower(host)) {
		return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	}
	if address, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{address}, nil
	}
	// Fixed verified HTTPS resolver, using its ordinary TLS validation and system bootstrap DNS.
	// No alternate hostnames, untrusted resolver URLs, or insecure TLS fallback.
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	endpoint := "https://cloudflare-dns.com/dns-query?" + url.Values{"name": {host}, "type": {"A"}}.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Accept", "application/dns-json")
	response, err := client.Do(req)
	if err != nil {
		return nil, &net.DNSError{Err: "verified HTTPS DNS unavailable", Name: host}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || response.StatusCode != 200 || len(body) > 65536 {
		return nil, &net.DNSError{Err: "invalid HTTPS DNS response", Name: host}
	}
	return parseHTTPSAddresses(body, host)
}

func parseHTTPSAddresses(body []byte, host string) ([]netip.Addr, error) {
	var data struct {
		Status int
		Answer []struct {
			Type int
			Data string
		}
	}
	if json.Unmarshal(body, &data) != nil || data.Status != 0 {
		return nil, &net.DNSError{Err: "HTTPS DNS returned no usable answer", Name: host}
	}
	addresses := []netip.Addr{}
	seen := map[netip.Addr]bool{}
	for _, answer := range data.Answer {
		if answer.Type != 1 {
			continue
		}
		address, err := netip.ParseAddr(answer.Data)
		if err != nil || !address.Is4() || !publicIP(address) {
			return nil, errNonPublicDNS
		}
		if !seen[address] {
			addresses = append(addresses, address)
			seen[address] = true
		}
	}
	if len(addresses) == 0 {
		return nil, &net.DNSError{Err: "no IPv4 HTTPS DNS answers", Name: host}
	}
	return addresses, nil
}
