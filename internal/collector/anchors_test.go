package collector

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

type anchorPage struct {
	queries  []Query
	response func(Query) Result
}

func (p *anchorPage) Collect(_ context.Context, q Query) Result {
	p.queries = append(p.queries, q)
	return p.response(q)
}

func TestAccountAnchorsPrecedeUnavailableIndexes(t *testing.T) {
	page := &anchorPage{response: func(q Query) Result {
		handle := "ayu_ig"
		if strings.Contains(q.URL, "github.com") {
			handle = "ayu-dev"
		}
		return Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{URL: q.URL, SourceURL: q.URL, Rule: "public-page-v4", Facts: []Fact{{"page_title", "Profile (@" + handle + ")"}}}}}
	}}
	index := &scriptedCollector{result: Result{Status: "FAILED", Requests: 1}}
	d := Discovery{page: page, providers: []Collector{index}}
	q := Query{Name: "Ayu", Accounts: []domain.AccountClue{{Platform: "instagram", Username: "ayu_ig"}, {Platform: "github", Username: "ayu-dev"}}, Budget: 4, Limit: 5}
	r := d.Collect(context.Background(), q)
	if len(r.Profiles) != 2 || r.Requests != 4 || r.Status != "SUCCESS" || len(page.queries) != 2 || len(index.queries) != 2 {
		t.Fatalf("account collection failed: %+v", r)
	}
	if index.queries[0].Platform != "instagram" || index.queries[0].Username != "ayu_ig" || index.queries[1].Platform != "github" || index.queries[1].Username != "ayu-dev" {
		t.Fatalf("platform clues mixed: %+v", index.queries)
	}
	for _, p := range r.Profiles {
		if p.Login != "" || p.Name != "" {
			t.Fatal("supplied handle promoted to observed identity")
		}
	}
}

func TestAnchorDeduplicationRetainsSeparateProvenance(t *testing.T) {
	page := &anchorPage{response: func(q Query) Result {
		return Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{URL: q.URL, SourceURL: q.URL, Rule: "public-page-v4", ResponseSHA256: strings.Repeat("a", 64), Facts: []Fact{{"page_title", "Ayu (@ayu_ig)"}}}}}
	}}
	index := &scriptedCollector{result: Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{Key: "web-index", URL: "https://instagram.com/ayu_ig/", SourceURL: "https://search.example/query", Facts: []Fact{{"search_title", "Ayu"}, {"search_snippet", "Known profile"}}}}}}
	d := Discovery{page: page, providers: []Collector{index}}
	r := d.Collect(context.Background(), Query{Name: "Ayu", Username: "ayu_ig", Platform: "instagram", Budget: 3, Limit: 5})
	if len(r.Profiles) != 1 || len(page.queries) != 1 || r.Requests != 2 {
		t.Fatalf("duplicate profile or fetch: %+v", r)
	}
	p := r.Profiles[0]
	if p.FactSources["search_snippet"].URL != "https://search.example/query" || p.SourceURL != "https://www.instagram.com/ayu_ig" || p.ResponseSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("provenance lost: %+v", p)
	}
}

func TestAnchorDoesNotAcceptGenericOrRedirectedPages(t *testing.T) {
	for _, tc := range []struct{ url, title string }{
		{"https://www.instagram.com/ayu_ig", "Instagram"},
		{"https://www.instagram.com/other", "Ayu (@ayu_ig)"},
		{"https://example.org/ayu_ig", "Ayu (@ayu_ig)"},
		{"https://www.instagram.com/ayu_ig", "Ayu (@ayu_ig2)"},
		{"https://www.instagram.com/ayu_ig", "User not found: @ayu_ig"},
	} {
		page := &anchorPage{response: func(Query) Result {
			return Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{URL: tc.url, Facts: []Fact{{"page_title", tc.title}}}}}
		}}
		d := Discovery{page: page}
		r := d.Collect(context.Background(), Query{Username: "ayu_ig", Platform: "instagram", Budget: 1, Limit: 1})
		if len(r.Profiles) != 0 || r.Status != "EMPTY" {
			t.Fatalf("generic or wrong account retained: %+v", r)
		}
	}
}

func TestAnchorBudgetCancellationAndPublicURL(t *testing.T) {
	page := &anchorPage{response: func(q Query) Result { return Result{Status: "FAILED", Requests: 1} }}
	d := Discovery{page: page}
	for _, raw := range []string{"http://127.0.0.1/", "file:///profile", "https://user:pass@example.org/"} {
		r := d.Collect(context.Background(), Query{URL: raw, Limit: 1, Budget: 1})
		if r.Status != "BLOCKED" || len(page.queries) != 0 {
			t.Fatalf("invalid URL reached collector: %+v", r)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d.Collect(ctx, Query{URL: "https://example.org/", Limit: 1, Budget: 1})
	if len(page.queries) != 0 {
		t.Fatal("cancelled request fetched")
	}
	r := d.Collect(context.Background(), Query{Accounts: []domain.AccountClue{{Platform: "github", Username: "ayu"}, {Platform: "devto", Username: "ayu"}}, Limit: 5, Budget: 1})
	if len(page.queries) != 1 || r.Requests != 1 || r.Events[len(r.Events)-1].Status != "NOT_CHECKED" {
		t.Fatalf("budget not enforced: %+v", r)
	}
}

func TestAccountClueValidation(t *testing.T) {
	got, err := NormalizeAccounts([]domain.AccountClue{{Platform: "Instagram", Username: "@ayu_ig"}, {Platform: "instagram", Username: "ayu_ig"}, {Platform: "github", Username: "ayu-dev"}})
	if err != nil || len(got) != 2 || got[0].Username != "ayu_ig" {
		t.Fatalf("normalization: %+v %v", got, err)
	}
	for _, clue := range []domain.AccountClue{{Platform: "github", Username: "../admin"}, {Platform: "all", Username: "ayu"}, {Platform: "bluesky", Username: "ayu"}, {Platform: "instagram", Username: "https://example.org"}} {
		if _, err := NormalizeAccounts([]domain.AccountClue{clue}); err == nil {
			t.Fatalf("invalid account accepted: %+v", clue)
		}
	}
	many := []domain.AccountClue{}
	for i := 0; i < 9; i++ {
		many = append(many, domain.AccountClue{Platform: "github", Username: fmt.Sprintf("user%d", i)})
	}
	if _, err := NormalizeAccounts(many); err == nil {
		t.Fatal("unbounded account clues")
	}
}

func TestAccountRankingUsesItsOwnPlatform(t *testing.T) {
	profiles := []Profile{{URL: "https://example.org/ayu"}, {URL: "https://github.com/ayu"}}
	rankResults(profiles, Query{Accounts: []domain.AccountClue{{Platform: "github", Username: "ayu"}}})
	if profiles[0].URL != "https://github.com/ayu" {
		t.Fatal("known account did not outrank an incidental path match")
	}
}
