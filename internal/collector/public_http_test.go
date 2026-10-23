package collector

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDialCheckedIPsFallsBackBeforeFirstAddressTimesOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	firstStopped := make(chan struct{})
	winner, peer := net.Pipe()
	defer peer.Close()
	conn, err := dialCheckedIPs(ctx, "tcp", "443", []netip.Addr{netip.MustParseAddr("2606:4700:4700::1111"), netip.MustParseAddr("1.1.1.1")}, func(ctx context.Context, network, address string) (net.Conn, error) {
		if strings.HasPrefix(address, "[") {
			<-ctx.Done()
			close(firstStopped)
			return nil, ctx.Err()
		}
		return winner, nil
	})
	if err != nil || conn != winner {
		winner.Close()
		t.Fatalf("IPv4 fallback failed: %v", err)
	}
	conn.Close()
	select {
	case <-firstStopped:
	case <-time.After(time.Second):
		t.Fatal("losing dial was not cancelled")
	}
}

func TestDialCheckedIPsRejectsWholePrivateAnswer(t *testing.T) {
	called := false
	_, err := dialCheckedIPs(context.Background(), "tcp", "443", []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("127.0.0.1")}, func(context.Context, string, string) (net.Conn, error) { called = true; return nil, nil })
	if called || !errors.Is(err, errNonPublicDNS) {
		t.Fatal("private DNS answer reached dialer")
	}
}

func TestNetworkDiagnosticsDoNotExposeRequestSecrets(t *testing.T) {
	for _, tt := range []struct {
		err             error
		status, message string
	}{
		{errNonPublicDNS, "BLOCKED", "non-public"},
		{&net.DNSError{Err: "secret", Name: "hidden.example"}, "FAILED", "DNS lookup"},
		{&net.OpError{Op: "dial", Err: errors.New("secret")}, "FAILED", "Connection failed"},
		{context.DeadlineExceeded, "FAILED", "timed out"},
		{context.Canceled, "FAILED", "cancelled"},
	} {
		status, detail := networkFailure(&url.Error{Op: "Get", URL: "https://example.org/?token=secret", Err: tt.err})
		if status != tt.status || !strings.Contains(detail, tt.message) || strings.Contains(detail, "secret") || strings.Contains(detail, "hidden.example") {
			t.Fatalf("unexpected diagnostic: %s %s", status, detail)
		}
	}
}
