package collector

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

func TestResearchFollowsObservedLinksInTwoHops(t *testing.T) {
	bodies := map[string]string{
		"https://github.com/lailak":   `<title>lailak - GitHub</title><main><a rel="me" href="https://laila.example/">Website</a><a href="https://github.com/another/tutorial">Fork</a></main>`,
		"https://laila.example/":      `<title>Laila</title><main><a href="/about">About</a></main>`,
		"https://laila.example/about": `<title>Laila Kartika</title><main><h1>Laila Kartika</h1><p>Laila Kartika works at Orbit Labs.</p><a href="https://github.com/lailak">GitHub</a><a href="/">Home</a></main><script type="application/ld+json">{"@type":"Person","name":"Laila Kartika","worksFor":{"name":"Orbit Labs"},"alumniOf":{"name":"Example University"}}</script>`,
	}
	visited := map[string]int{}
	page := NewPage(time.Second)
	page.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		visited[req.URL.String()]++
		body, ok := bodies[req.URL.String()]
		if !ok {
			t.Fatalf("unplanned fetch: %s", req.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})}
	d := Discovery{page: page}
	r := d.Collect(context.Background(), Query{Name: "Laila", Accounts: []domain.AccountClue{{Platform: "github", Username: "lailak"}}, Budget: 10, Limit: 5})
	if r.Requests != 3 || len(r.Profiles) != 3 || len(r.Research) != 2 {
		t.Fatalf("chain missing: %+v", r)
	}
	bio := r.Profiles[2]
	if bio.Name != "Laila Kartika" || bio.LinkedFrom != "https://laila.example/" || len(bio.ResponseSHA256) != 64 {
		t.Fatalf("lost observed identity or provenance: %+v", bio)
	}
	if r.Profiles[0].Name != "" {
		t.Fatal("linked name silently assigned to account")
	}
	for _, n := range visited {
		if n != 1 {
			t.Fatal("link cycle fetched again")
		}
	}
}

func TestResearchHypothesisSearchDoesNotInventObservedName(t *testing.T) {
	page := &anchorPage{response: func(q Query) Result {
		return Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{URL: q.URL, SourceURL: q.URL, Facts: []Fact{{"page_title", "LailaKartika"}}}}}
	}}
	index := &anchorPage{response: func(q Query) Result {
		if q.Username != "" || q.Name == "Laila" {
			return Result{Status: "EMPTY", Requests: 1}
		}
		if q.Name != "Laila Kartika" || q.Platform != "" || len(q.Accounts) != 0 {
			t.Fatalf("hypothesis still restricted to handle: %+v", q)
		}
		return Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{URL: "https://directory.example/laila", SourceURL: "https://search.example/", Facts: []Fact{{"search_title", "Laila Kartika"}}}}}
	}}
	d := Discovery{page: page, providers: []Collector{index}}
	q := Query{Name: "Laila", Accounts: []domain.AccountClue{{Platform: "github", Username: "LailaKartika"}}, Budget: 10, Limit: 5}
	r := d.Collect(context.Background(), q)
	if len(index.queries) != 5 || len(r.Profiles) != 2 || r.Requests != 7 {
		t.Fatalf("follow-up search missing: %+v", r)
	}
	for _, p := range r.Profiles {
		if p.Name != "" {
			t.Fatal("query promoted to observed name")
		}
	}
	if !strings.Contains(r.Profiles[1].DiscoveryBasis, "hypothesis") || r.Research[len(r.Research)-2].Query != "Laila Kartika" || q.Accounts[0].Username != "LailaKartika" {
		t.Fatalf("hypothesis provenance lost: %+v", r)
	}
}

func TestResearchRejectsUnsafeAndIrrelevantLinks(t *testing.T) {
	page := NewPage(time.Second)
	page.client = mockClient(`<title>Laila</title><main><h1>Laila</h1><a rel="me" href="http://127.0.0.1/">Local</a><a href="https://other.example/">Random</a><a href="https://github.com/foreign/tutorial">Fork</a><a rel="me" href="https://laila.example/">Website</a><span hidden><a rel="me" href="https://hidden.example/">Hidden</a></span></main>`, "text/html", 200)
	r := page.Collect(context.Background(), Query{URL: "https://github.com/laila", Budget: 1})
	links := researchLinks(r.Profiles[0])
	if len(links) != 1 || links[0].URL != "https://laila.example/" {
		t.Fatalf("unsafe/noisy links: %+v", links)
	}
	for _, handle := range []string{"laila123", "unrelated_name", "laila", "laila@example.com"} {
		if handleHypothesis(handle, "Laila") != "" {
			t.Fatalf("unsupported alias: %s", handle)
		}
	}
}

func TestResearchBudgetAndCancellationBoundTheChain(t *testing.T) {
	for _, budget := range []int{1, 8, 10} {
		page := &anchorPage{response: func(q Query) Result {
			return Result{Status: "SUCCESS", Requests: q.Budget, Profiles: []Profile{{URL: q.URL, SourceURL: q.URL, Facts: []Fact{{"page_title", "LailaKartika"}, {"profile_links", `[{"url":"https://laila.example/about","reason":"bio"}]`}}}}}
		}}
		d := Discovery{page: page}
		r := d.Collect(context.Background(), Query{URL: "https://laila.example/", Budget: budget, Limit: 5})
		if r.Requests > budget || len(page.queries) != 1 {
			t.Fatalf("over budget: %+v", r)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r = d.Collect(ctx, Query{URL: "https://laila.example/", Budget: budget, Limit: 5})
		if r.Requests != 0 || len(page.queries) != 1 {
			t.Fatal("cancelled research fetched")
		}
	}
}

func TestObservedNameIsNotTakenFromUnrelatedStructuredPerson(t *testing.T) {
	page := NewPage(time.Second)
	page.client = mockClient(`<title>Conference</title><main><h1>Conference</h1></main><script type="application/ld+json">{"@type":"Person","name":"Other Person"}</script>`, "text/html", 200)
	r := page.Collect(context.Background(), Query{URL: "https://conference.example/", Budget: 1})
	if r.Profiles[0].Name != "" {
		t.Fatal("speaker or author assigned as page subject")
	}
}

func TestResearchDoesNotRetryUnavailableProviderForEveryClue(t *testing.T) {
	page := &anchorPage{response: func(q Query) Result {
		return Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{URL: q.URL, SourceURL: q.URL, Facts: []Fact{{"page_title", "LailaKartika"}}}}}
	}}
	index := &scriptedCollector{result: Result{Status: "FAILED", Requests: 1, Events: []Event{{"searxng:web", "FAILED", "SearXNG service unavailable or timed out"}}}}
	d := Discovery{page: page, providers: []Collector{index}}
	r := d.Collect(context.Background(), Query{Name: "Laila", Accounts: []domain.AccountClue{{Platform: "github", Username: "LailaKartika"}}, Budget: 20, Limit: 5})
	if len(index.queries) != 1 || r.Requests != 2 {
		t.Fatalf("unavailable provider retried: %+v", r)
	}
	if operationalFailure(Result{Status: "FAILED", Events: []Event{{"bing:web", "FAILED", "No usable matching links"}}}) {
		t.Fatal("query-specific absence disabled the provider")
	}
}

func TestBiographyNavigationOutranksAdditionalSocialAccounts(t *testing.T) {
	page := NewPage(time.Second)
	page.client = mockClient(`<title>Laila</title><nav><a href="/about">About</a><a href="https://other.example/about">About</a></nav><main>Posts</main>`, "text/html", 200)
	r := page.Collect(context.Background(), Query{URL: "https://laila.example/", Budget: 1})
	seed := Profile{URL: "https://github.com/laila", Facts: []Fact{{"profile_links", `[{"url":"https://bsky.app/profile/laila.example","reason":"profile self-link"}]`}}}
	parent, link, ok := nextResearchLink(append([]Profile{seed}, r.Profiles...), map[string]bool{})
	if !ok || link.URL != "https://laila.example/about" || parent.URL != "https://laila.example/" {
		t.Fatalf("biography lost to navigation or lower-value links: %+v", link)
	}
	if len(researchLinks(r.Profiles[0])) != 1 {
		t.Fatal("external About navigation retained")
	}
}
