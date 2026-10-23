package collector

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRedirectScopeAndBudget(t *testing.T) {
	for _, tt := range []struct {
		name, location, status string
		budget, requests       int
	}{
		{"public", "/profile/", "SUCCESS", 3, 2},
		{"budget", "/profile/", "FAILED", 1, 1},
		{"loop", "/profile", "FAILED", 3, 1},
		{"private", "http://127.0.0.1/", "BLOCKED", 3, 1},
		{"downgrade", "http://example.org/profile/", "BLOCKED", 3, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				code := 200
				h := http.Header{"Content-Type": {"text/html"}}
				if calls == 1 {
					code = 302
					h.Set("Location", tt.location)
				}
				return &http.Response{StatusCode: code, Header: h, Body: io.NopCloser(strings.NewReader("<main>Profile</main>")), Request: req}, nil
			})}
			r := fetchPublic(context.Background(), client, "https://example.org/profile", tt.budget, nil)
			if r.Status != tt.status || calls != tt.requests || r.Requests != calls {
				t.Fatalf("unexpected response: %+v, calls=%d", r, calls)
			}
		})
	}
}

func TestDirectRedirectAndProfileMarkers(t *testing.T) {
	body := `<link rel="canonical" href="https://dev.to/anton/"><meta property="og:site_name" content="DEV Community"><meta property="og:title" content="Anton - DEV Community Profile"><h1 class="crayons-title">Anton</h1>`
	for _, target := range []string{"/anton/", "/someone-else", "https://other.example/anton", "/login"} {
		t.Run(target, func(t *testing.T) {
			calls := 0
			d := NewDirect(time.Second)
			d.client = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				h, code := http.Header{"Content-Type": {"text/html"}}, 200
				if calls == 1 {
					h.Set("Location", target)
					code = 302
				}
				return &http.Response{StatusCode: code, Header: h, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
			})}
			r := d.Collect(context.Background(), Query{Username: "anton", Platform: "devto", Budget: 3, Limit: 1})
			if target == "/anton/" {
				if r.Status != "SUCCESS" || calls != 2 || len(r.Profiles) != 1 || r.Receipts[0].URL != r.Profiles[0].SourceURL {
					t.Fatalf("valid redirect failed: %+v", r)
				}
			} else if r.Status != "BLOCKED" || calls != 1 {
				t.Fatalf("changed identity followed: %+v", r)
			}
		})
	}
	a := DirectAdapters()[1]
	_, status, _ := parseDirect([]byte(body+`<meta property="og:url" content="https://dev.to/other">`), a, "anton", "https://dev.to/anton")
	if status != "FAILED" {
		t.Fatal("conflicting profile URL accepted")
	}
}

func TestPageContentExtraction(t *testing.T) {
	p := NewPage(time.Second)
	p.client = mockClient(`<title>Anton</title><nav>Navigation</nav><main><h1>Public biography</h1><p>Writes software.</p><script>secretScript</script><style>secretStyle</style><span hidden>hiddenText</span><div aria-hidden="true">hiddenAria</div><form>loginForm</form><a href="/about">About</a><a href="https://github.com/anton">GitHub</a><a href="http://127.0.0.1/">Local</a><a href="javascript:alert(1)">Bad</a></main><footer>Footer</footer>`, "text/html", 200)
	r := p.Collect(context.Background(), Query{URL: "https://example.org/profile", Budget: 1})
	if len(r.Profiles) != 1 {
		t.Fatalf("no page: %+v", r)
	}
	facts := map[string]string{}
	for _, f := range r.Profiles[0].Facts {
		facts[f.Type] = f.Value
	}
	for _, forbidden := range []string{"Navigation", "secretScript", "secretStyle", "hiddenText", "hiddenAria", "loginForm", "Footer"} {
		if strings.Contains(facts["page_text"], forbidden) {
			t.Fatalf("included excluded content: %s", forbidden)
		}
	}
	if !strings.Contains(facts["page_text"], "Writes software.") || facts["page_links"] != "https://example.org/about\nhttps://github.com/anton" || len(r.Profiles[0].ResponseSHA256) != 64 {
		t.Fatalf("invalid extraction: %+v", r.Profiles[0])
	}
	p.client = mockClient("<main>"+strings.Repeat("界", 20000)+"</main>", "text/html", 200)
	r = p.Collect(context.Background(), Query{URL: "https://example.org", Budget: 1})
	for _, f := range r.Profiles[0].Facts {
		if f.Type == "page_text" && (len([]rune(f.Value)) > 8012 || !strings.HasSuffix(f.Value, "[truncated]")) {
			t.Fatal("text bound failed")
		}
	}
}

func TestDiscoveryRelevanceAndRanking(t *testing.T) {
	q := Query{Name: "Anton Test", Username: "antont", Company: "Example Corp", Location: "Jakarta", Budget: 1, Limit: 1}
	if !relevantResult(q, "Anton Test writes software") || !relevantResult(q, "https://example.org/antont") || relevantResult(q, "Someone Else Example Corp Jakarta") {
		t.Fatal("primary identity relevance failed")
	}
	if strings.Contains(discoveryPhrase(q), "Jakarta") || strings.Contains(discoveryPhrase(q), "Example Corp") {
		t.Fatal("optional hints restrict discovery")
	}
	b := NewBing(time.Second)
	b.client = mockClient(`<rss><channel><title>Results</title><item><title>Anton Test</title><link>https://example.org/a</link></item><item><title>Anton Test</title><description>Example Corp Jakarta</description><link>https://example.org/b</link></item></channel></rss>`, "application/rss+xml", 200)
	r := b.Collect(context.Background(), q)
	if len(r.Profiles) != 1 || r.Profiles[0].URL != "https://example.org/b" {
		t.Fatalf("ranking did not precede limit: %+v", r)
	}
	body := []byte(`<div class="result"><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.org%2Fantont">Anton Test</a><div class="result__snippet">Public profile</div></div>`)
	profiles, status := parseDuckDuckGo(body, "https://html.duckduckgo.com/html/?q=Anton", q, "web")
	if status != "SUCCESS" || len(profiles) != 1 || profiles[0].URL != "https://example.org/antont" {
		t.Fatalf("unexpected DDG parse: %+v %s", profiles, status)
	}
	if _, status := parseDuckDuckGo(append(body, []byte(`<form id="challenge-form"></form>`)...), "https://html.duckduckgo.com", q, "web"); status != "BLOCKED" {
		t.Fatal("challenge accepted")
	}
	if _, status := parseDuckDuckGo(body, "https://html.duckduckgo.com", q, "github"); status != "FAILED" {
		t.Fatal("off-platform result accepted")
	}
}

type scriptedCollector struct {
	result  Result
	queries []Query
}

func (s *scriptedCollector) Collect(_ context.Context, q Query) Result {
	s.queries = append(s.queries, q)
	return s.result
}

func TestDiscoveryFallbackAndPageProvenance(t *testing.T) {
	blocked := &scriptedCollector{result: Result{Status: "BLOCKED", Requests: 1, Events: []Event{{"first:search", "BLOCKED", "Rate limited"}}}}
	index := &scriptedCollector{result: Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{Key: "web-one", URL: "https://example.org/anton", SourceURL: "https://search.example/query", Facts: []Fact{{"search_title", "Anton"}}}}}}
	page := &scriptedCollector{result: Result{Status: "SUCCESS", Requests: 2, Profiles: []Profile{{SourceURL: "https://example.org/anton/", ResponseSHA256: strings.Repeat("a", 64), Rule: "public-page-v2", Facts: []Fact{{"page_text", "Public content"}}}}}}
	d := Discovery{providers: []Collector{blocked, index}, page: page}
	r := d.Collect(context.Background(), Query{Name: "Anton", Budget: 4, Limit: 1})
	if len(r.Profiles) != 1 || r.Requests != 4 || index.queries[0].Budget != 3 || page.queries[0].Budget != 2 || r.Profiles[0].SourceURL != "https://search.example/query" || r.Profiles[0].FactSources["page_text"].URL != "https://example.org/anton/" {
		t.Fatalf("invalid fallback or provenance: %+v", r)
	}
	page.queries = nil
	r = d.Collect(context.Background(), Query{Name: "Anton", Budget: 2, Limit: 1})
	if len(page.queries) != 0 || len(r.Profiles) != 1 || r.Events[len(r.Events)-1].Status != "NOT_CHECKED" {
		t.Fatal("page fetch exceeded budget or lost index evidence")
	}
}

func TestDiscoveryUnionsProviderProfilesAndDedupes(t *testing.T) {
	first := &scriptedCollector{result: Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{
		{Key: "web-a", URL: "https://example.org/a", SourceURL: "https://search.example/a"},
		{Key: "web-shared", URL: "https://example.org/shared", SourceURL: "https://search.example/shared"},
	}}}
	second := &scriptedCollector{result: Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{
		{Key: "web-shared", URL: "https://example.org/shared", SourceURL: "https://search.example/shared"},
		{Key: "web-b", URL: "https://example.org/b", SourceURL: "https://search.example/b"},
	}}}
	page := &scriptedCollector{result: Result{Status: "FAILED"}}
	d := Discovery{providers: []Collector{first, second}, page: page}
	r := d.Collect(context.Background(), Query{Name: "Anton", Budget: 5, Limit: 5})
	if len(r.Profiles) != 3 || r.Requests != 2 || r.Status != "SUCCESS" {
		t.Fatalf("providers were not unioned: %+v", r)
	}
	if r.Profiles[0].Key != "web-a" || r.Profiles[1].Key != "web-shared" || r.Profiles[2].Key != "web-b" {
		t.Fatalf("duplicate or missing profiles: %+v", r.Profiles)
	}
	if first.queries[0].Budget != 5 || second.queries[0].Budget != 4 || second.queries[0].Limit != 3 {
		t.Fatalf("budget or remaining limit not shared: %+v %+v", first.queries[0], second.queries[0])
	}
	if len(page.queries) != 3 {
		t.Fatalf("page fetches not attempted: %d", len(page.queries))
	}
}

func TestDevToPublicProfile(t *testing.T) {
	for _, tt := range []struct {
		body, status string
		code         int
	}{
		{`{"id":42,"username":"anton","name":"Anton","location":"Jakarta","summary":"Developer","website_url":"https://example.org"}`, "SUCCESS", 200},
		{`{"id":42,"username":"other"}`, "FAILED", 200},
		{`<html>challenge</html>`, "FAILED", 200},
		{`{}`, "EMPTY", 404},
		{`{}`, "BLOCKED", 429},
	} {
		d := NewDevTo(time.Second)
		d.client = mockClient(tt.body, "application/json", tt.code)
		r := d.Collect(context.Background(), Query{Username: "anton", Budget: 1, Limit: 1})
		if r.Status != tt.status || r.Requests != 1 || len(r.Receipts) != 1 {
			t.Fatalf("unexpected API result: %+v", r)
		}
		if tt.status == "SUCCESS" && (len(r.Profiles) != 1 || r.Profiles[0].Location != "Jakarta" || r.Profiles[0].ResponseSHA256 != r.Receipts[0].ResponseSHA256) {
			t.Fatal("missing fields or provenance")
		}
	}
}

func TestAutoContinuesAfterFailedPlatform(t *testing.T) {
	g := &scriptedCollector{result: Result{Status: "BLOCKED", Requests: 1}}
	d := &scriptedCollector{result: Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{Key: "devto-42", Login: "anton"}}}}
	b := &scriptedCollector{result: Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{Key: "bluesky-42", Login: "anton.bsky.social"}}}}
	a := PublicProfiles{collectors: map[string]Collector{"github": g, "devto": d, "bluesky": b}}
	r := a.Collect(context.Background(), Query{Username: "anton", Budget: 3, Limit: 5})
	if r.Status != "SUCCESS" || !r.Incomplete || len(r.Profiles) != 2 || r.Requests != 3 || d.queries[0].Budget != 2 || b.queries[0].Username != "anton.bsky.social" {
		t.Fatalf("unexpected auto result: %+v", r)
	}
	b.queries = nil
	r = a.Collect(context.Background(), Query{Username: "anton", Budget: 3, Limit: 1})
	if len(r.Profiles) != 1 || r.Requests != 2 || len(b.queries) != 0 {
		t.Fatal("candidate limit exceeded")
	}
}
