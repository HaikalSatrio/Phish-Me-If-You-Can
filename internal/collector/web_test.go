package collector

import (
	"context"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func mockClient(body, content string, code int) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": []string{content}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
}

func TestGoogleHTMLParsing(t *testing.T) {
	body := `<html><body><a href="/url?q=https%3A%2F%2Fexample.org%2Fanton"><h3>Anton &amp; Company</h3></a><a href="https://example.org/anton"><h3>Duplicate</h3></a><a href="https://example.org/unrelated">Navigation</a><a href="http://127.0.0.1/"><h3>Local</h3></a></body></html>`
	profiles, status := parseGoogle([]byte(body), "https://www.google.com/search?q=Anton", 5, "web")
	if status != "SUCCESS" || len(profiles) != 1 || profiles[0].Facts[0].Value != "Anton & Company" {
		t.Fatalf("unexpected parsed results: %+v %s", profiles, status)
	}
	for _, body := range []string{`<p>unusual traffic from your computer network</p>`, `<form><input type=password></form>`} {
		if _, status := parseGoogle([]byte(body), "https://www.google.com", 5, "web"); status != "BLOCKED" {
			t.Fatal("challenge accepted")
		}
	}
	if _, status := parseGoogle([]byte(`<html>unknown layout</html>`), "https://www.google.com", 5, "web"); status != "FAILED" {
		t.Fatal("unknown layout treated as empty")
	}
}

func TestPublicURLBoundaries(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1/", "http://[::1]/", "http://169.254.169.254/", "http://10.0.0.1/", "http://localhost/", "file:///etc/passwd", "https://user:pass@example.org/", "http://example.org:8080/"} {
		if _, err := cleanURL(raw); err == nil {
			t.Fatalf("unsafe URL accepted: %s", raw)
		}
	}
	for _, raw := range []string{"100.64.0.1", "::ffff:127.0.0.1", "192.168.1.1", "2001:db8::1"} {
		if publicIP(netip.MustParseAddr(raw)) {
			t.Fatalf("unsafe IP accepted: %s", raw)
		}
	}
	if _, err := cleanURL("https://example.org/about"); err != nil {
		t.Fatal(err)
	}
	if platformFor("instagram.com.evil.example") != "web" || platformFor("www.instagram.com") != "instagram" {
		t.Fatal("incorrect host boundaries")
	}
}

func TestPageMetadataAndMissingKey(t *testing.T) {
	p := NewPage(time.Second)
	p.client = mockClient(`<title>Example &amp; Co</title><meta name="description" content="Public page"><script>fake email</script>`, "text/html", 200)
	r := p.Collect(context.Background(), Query{URL: "https://example.org/", Budget: 1})
	if r.Status != "SUCCESS" || len(r.Profiles) != 1 || len(r.Profiles[0].Facts) != 2 || r.Profiles[0].Name != "" {
		t.Fatalf("unexpected metadata: %+v", r)
	}
	w := NewWeb("brave", "", "", time.Second)
	r = w.Collect(context.Background(), Query{Name: "Anton", Budget: 1, Limit: 5})
	if r.Status != "BLOCKED" || r.Requests != 0 {
		t.Fatal("missing credential made a request")
	}
}

func TestSearchProviderFormats(t *testing.T) {
	for _, provider := range []string{"google", "brave"} {
		body := `{"web":{"results":[{"title":"Anton","url":"https://example.org/anton","description":"Index snippet"}]}}`
		if provider == "google" {
			body = `{"items":[{"title":"Anton","link":"https://example.org/anton","snippet":"Index snippet"}]}`
		}
		w := NewWeb(provider, "secret", "engine-id", time.Second)
		w.client = mockClient(body, "application/json", 200)
		r := w.Collect(context.Background(), Query{Name: "Anton", Limit: 5, Budget: 1})
		if len(r.Profiles) != 1 || strings.Contains(r.Profiles[0].SourceURL, "secret") || r.Profiles[0].Name != "" {
			t.Fatalf("invalid search evidence: %+v", r)
		}
	}
}

func TestAllPlatformsStopOnRestriction(t *testing.T) {
	g := NewGoogleHTML(time.Second)
	g.client = mockClient("", "text/html", 429)
	r := g.Collect(context.Background(), Query{Name: "Anton", Platform: "all", Budget: 20, Limit: 5})
	if r.Requests != 1 || len(r.Events) != len(Platforms()) || r.Events[1].Status != "NOT_CHECKED" {
		t.Fatalf("did not stop after restriction: %+v", r)
	}
	w := NewWeb("brave", "secret", "", time.Second)
	w.client = mockClient("", "application/json", 429)
	r = w.Collect(context.Background(), Query{Name: "Anton", Platform: "all", Budget: 20, Limit: 5})
	if r.Requests != 1 || len(r.Events) != len(Platforms()) {
		t.Fatal("API provider continued after rate limit")
	}
}

func TestBlueskyProfile(t *testing.T) {
	b := NewBluesky(time.Second)
	b.client = mockClient(`{"did":"did:plc:test","handle":"anton.example","displayName":"Anton","description":"Example profile"}`, "application/json", 200)
	r := b.Collect(context.Background(), Query{Username: "anton.example", Budget: 1})
	if len(r.Profiles) != 1 || r.Profiles[0].Login != "anton.example" {
		t.Fatalf("invalid public profile: %+v", r)
	}
}

func TestRefinementQueryIncludesClues(t *testing.T) {
	got := searchPhrase(Query{Name: "Anton", Username: "anton.example", Company: "Example Company", Location: "Jakarta"})
	for _, want := range []string{`"Anton"`, `"anton.example"`, `"Example Company"`, `"Jakarta"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("query omitted clue: %s", got)
		}
	}
}

func TestBrowserNavigationBoundaries(t *testing.T) {
	_, status, _ := renderGoogle(context.Background(), "unused", "https://example.org", time.Second)
	if status != "BLOCKED" {
		t.Fatal("browser allowed arbitrary navigation")
	}
	_, status, _ = renderGoogle(context.Background(), "", "https://www.google.com/search?q=Anton", time.Second)
	if status != "BLOCKED" {
		t.Fatal("missing browser not recorded")
	}
}
