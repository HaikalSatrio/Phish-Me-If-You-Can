package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSearXNGConfigurationAndPublicResults(t *testing.T) {
	for _, raw := range []string{"", "file:///tmp/search", "http://10.0.0.1", "https://user:pass@example.org", "https://example.org/?key=secret", "http://example.org", "https://example.org/#fragment"} {
		if _, err := NewSearXNG(raw, time.Second); err == nil {
			t.Fatalf("invalid service accepted: %s", raw)
		}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/search" || req.URL.Query().Get("format") != "json" || strings.Contains(req.URL.Query().Get("q"), "Jakarta") {
			t.Error("invalid search query")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"url":"http://127.0.0.1/private","title":"Anton"},{"url":"https://example.org/anton","title":"Anton","content":"Developer"},{"url":"https://example.org/anton","title":"Anton"},{"url":"https://example.org/other","title":"Someone else"}],"unresponsive_engines":[["example","timeout"]]}`))
	}))
	defer s.Close()
	c, err := NewSearXNG(s.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	r := c.Collect(context.Background(), Query{Name: "Anton", Location: "Jakarta", Limit: 5, Budget: 1})
	if r.Status != "SUCCESS" || len(r.Profiles) != 1 || r.Requests != 1 || !strings.Contains(r.Events[0].Detail, "1 upstream") {
		t.Fatalf("unexpected results: %+v", r)
	}
	if _, err := cleanURL(s.URL); err == nil {
		t.Fatal("local service exception leaked into page fetching")
	}
}

func TestSearXNGFailureAndRedirect(t *testing.T) {
	for _, code := range []int{200, 302, 403, 429} {
		c, err := NewSearXNG("http://127.0.0.1:8888", time.Second)
		if err != nil {
			t.Fatal(err)
		}
		c.client = mockClient(`<html>Not JSON</html>`, "text/html", code)
		r := c.Collect(context.Background(), Query{Name: "Anton", Budget: 1, Limit: 5})
		want := "FAILED"
		if code == 403 || code == 429 {
			want = "BLOCKED"
		}
		if r.Status != want || len(r.Profiles) != 0 {
			t.Fatalf("invalid service response accepted: %+v", r)
		}
	}
	requests := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		http.Redirect(w, req, "/elsewhere", http.StatusFound)
	}))
	defer s.Close()
	c, _ := NewSearXNG(s.URL, time.Second)
	r := c.Collect(context.Background(), Query{Name: "Anton", Budget: 3, Limit: 5})
	if requests != 1 || r.Status != "FAILED" {
		t.Fatal("service redirect followed")
	}
}

func TestDiscoveryDefaultsToLocalServiceAndReplacesOverride(t *testing.T) {
	d, err := NewConfiguredDiscovery(time.Second, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.providers) != 3 || d.providers[0].(*SearXNG).endpoint != "http://127.0.0.1:8888/search" {
		t.Fatal("local service missing from defaults")
	}
	d, err = NewConfiguredDiscovery(time.Second, "https://search.example.org/base")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.providers) != 3 || d.providers[0].(*SearXNG).endpoint != "https://search.example.org/base/search" {
		t.Fatal("override duplicated the local service")
	}
	d, err = NewConfiguredDiscoveryWithOptions(time.Second, "", SearXNGOptions{Pages: 2, Language: "id-ID", SafeSearch: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := d.providers[0].(*SearXNG).opts; got.Pages != 2 || got.Language != "id-ID" || got.SafeSearch != 1 {
		t.Fatalf("configured options were not applied: %+v", got)
	}
}

func TestSearXNGRejectsInvalidSearchOptions(t *testing.T) {
	for _, opts := range []SearXNGOptions{
		{Pages: 4},
		{SafeSearch: 3},
		{TimeRange: "hour"},
		{Language: "id ID"},
		{Engines: []string{"google,bing"}},
	} {
		if _, err := NewSearXNGWithOptions("http://127.0.0.1:8888", time.Second, opts); err == nil {
			t.Fatalf("invalid search options accepted: %+v", opts)
		}
	}
}

func TestSearchPhrasesPrioritizeIdentity(t *testing.T) {
	q := Query{Name: "Anton Test", Username: "antont", Company: "Example Corp", Location: "Jakarta"}
	phrases := searchPhrases(q)
	if len(phrases) != 4 || phrases[0] != `("Anton Test" OR "antont")` {
		t.Fatalf("unexpected primary phrase: %q", phrases)
	}
	for _, hint := range []string{"Example Corp", "Jakarta"} {
		if strings.Contains(phrases[0], hint) {
			t.Fatalf("optional hint %q restricted the primary query", hint)
		}
	}
	if !strings.Contains(phrases[1], "Example Corp") || !strings.Contains(phrases[2], "Jakarta") {
		t.Fatalf("hint variants missing: %q", phrases)
	}
	if !strings.Contains(phrases[3], "Example Corp") || !strings.Contains(phrases[3], "Jakarta") {
		t.Fatalf("combined hint variant missing: %q", phrases)
	}
	if got := searchPhrases(Query{Name: "Anton Test"}); len(got) != 1 || got[0] != `"Anton Test"` {
		t.Fatalf("name-only phrase failed: %q", got)
	}
	if got := searchPhrases(Query{Username: "antont"}); len(got) != 1 || got[0] != `"antont"` {
		t.Fatalf("username fallback failed: %q", got)
	}
	if len(searchPhrases(Query{})) != 0 {
		t.Fatal("empty identity produced a query")
	}
}

func TestSearXNGQueryExpansionAndParameters(t *testing.T) {
	var queries []url.Values
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		queries = append(queries, req.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"url":"https://example.org/anton","title":"Anton Test","content":"Developer"}],"unresponsive_engines":[]}`))
	}))
	defer s.Close()
	c, err := NewSearXNGWithOptions(s.URL, time.Second, SearXNGOptions{Language: "id-ID", Engines: []string{"google", "bing"}, TimeRange: "month", Pages: 2})
	if err != nil {
		t.Fatal(err)
	}
	r := c.Collect(context.Background(), Query{Name: "Anton Test", Company: "Example Corp", Budget: 4, Limit: 5})
	if r.Status != "SUCCESS" || r.Requests != 4 || len(r.Profiles) != 1 {
		t.Fatalf("unexpected expansion result: %+v", r)
	}
	if len(queries) != 4 {
		t.Fatalf("unexpected request count: %d", len(queries))
	}
	first, second, third, fourth := queries[0], queries[1], queries[2], queries[3]
	if first.Get("q") != `"Anton Test"` || first.Get("pageno") != "" || first.Get("language") != "id-ID" || first.Get("engines") != "google,bing" || first.Get("time_range") != "month" || first.Get("safesearch") != "0" {
		t.Fatalf("search parameters missing: %v", first)
	}
	if second.Get("pageno") != "2" {
		t.Fatalf("pagination missing: %v", second)
	}
	if !strings.Contains(third.Get("q"), "Example Corp") || third.Get("pageno") != "" || fourth.Get("pageno") != "2" {
		t.Fatalf("expanded phrase or pagination missing: %v %v", third, fourth)
	}
}
