package collector

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestUsernameClueDoesNotBecomeNameOnlyMatch(t *testing.T) {
	query := Query{Name: "Haikal", Username: "@haikal_satrio"}
	for _, text := range []string{"Haikal Hassan", "Haikal Satrio Wibowo", "https://instagram.com/haikal_satrio2/", "https://instagram.com/xhaikal_satrio/", "Haikal Satrio"} {
		if relevantResult(query, text) {
			t.Fatalf("unrelated result matched username: %s", text)
		}
	}
	for _, text := range []string{"Haikal (@haikal_satrio)", "https://instagram.com/haikal_satrio/", "https://github.com/haikal_satrio"} {
		if !relevantResult(query, text) {
			t.Fatalf("exact mention rejected: %s", text)
		}
	}
	if discoveryPhrase(query) != `"haikal_satrio"` {
		t.Fatalf("unexpected fallback query: %s", discoveryPhrase(query))
	}
	profiles := []Profile{{URL: "https://example.org/article", Facts: []Fact{{Type: "search_snippet", Value: "haikal_satrio Example Corp Jakarta"}}}, {URL: "https://instagram.com/haikal_satrio/"}}
	query.Company, query.Location = "Example Corp", "Jakarta"
	rankResults(profiles, query)
	if profiles[0].URL != "https://instagram.com/haikal_satrio/" {
		t.Fatal("exact account path did not outrank incidental mention")
	}
}

func TestBingRetainsUsernameConstraintAcrossNameCollisions(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("q") != `"haikal_satrio" (site:instagram.com)` {
			t.Errorf("unexpected query: %s", r.URL.Query().Get("q"))
		}
		body := `<rss><channel><title>Results</title><item><title>Haikal Hassan</title><link>https://instagram.com/haikalhassan/</link></item><item><title>Haikal</title><link>https://instagram.com/haikal_satrio/</link></item></channel></rss>`
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/rss+xml"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
	b := &Bing{client: client, endpoint: "https://www.bing.com/search"}
	got := b.Collect(context.Background(), Query{Name: "Haikal", Username: "haikal_satrio", Platform: "instagram", Limit: 5, Budget: 2})
	if len(got.Profiles) != 1 || got.Profiles[0].URL != "https://instagram.com/haikal_satrio/" {
		t.Fatalf("wrong candidate survived: %+v", got)
	}
}
