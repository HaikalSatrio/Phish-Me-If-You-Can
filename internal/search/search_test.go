package search

import (
	"context"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/collector"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/scoring"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

type fakeCollector struct {
	result collector.Result
	calls  int
}

func (f *fakeCollector) Collect(_ context.Context, _ collector.Query) collector.Result {
	f.calls++
	return f.result
}

func TestSearchReviewAndScore(t *testing.T) {
	var cfg Config
	var policy domain.Policy
	if err := storage.Read("../../configs/collector-v1.json", &cfg); err != nil {
		t.Fatal(err)
	}
	if err := storage.Read("../../configs/experimental-v1.json", &policy); err != nil {
		t.Fatal(err)
	}
	f := &fakeCollector{result: collector.Result{Status: "SUCCESS", Requests: 1, Profiles: []collector.Profile{{ID: 42, Login: "anton-test", Name: "Anton", Company: "Example", Email: "anton@example.org"}}, Events: []collector.Event{{Operation: "github:user:anton-test", Status: "SUCCESS"}}}}
	c, err := Run(context.Background(), f, Input{Name: "Anton", Username: "anton-test"}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Search.Status != "REVIEW_REQUIRED" || len(c.Evidence) != 4 {
		t.Fatalf("unexpected session %+v", c)
	}
	r, err := scoring.Calculate(c, policy)
	if err != nil {
		t.Fatal(err)
	}
	if r.PES != nil {
		t.Fatal("unreviewed search was scored")
	}
	c.Candidates[0].Decision = "CONFIRM"
	r, err = scoring.Calculate(c, policy)
	if err != nil || r.PES == nil {
		t.Fatalf("confirmed profile not scored: %v", err)
	}
	for c.Search.Attempts < cfg.MaxAttempts {
		c, err = Run(context.Background(), f, Input{Company: "Wrong Company"}, cfg, &c)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(c.Candidates[0].Contradictions) != 1 || c.Candidates[0].Decision != "PENDING" {
		t.Fatal("refinement did not reset and compare identity")
	}
	before := f.calls
	if _, err = Run(context.Background(), f, Input{}, cfg, &c); err == nil || f.calls != before {
		t.Fatal("exhausted session made a request")
	}
}

func TestPartialCoverageAndEmpty(t *testing.T) {
	var cfg Config
	if err := storage.Read("../../configs/collector-v1.json", &cfg); err != nil {
		t.Fatal(err)
	}
	f := &fakeCollector{result: collector.Result{Status: "EMPTY", Requests: 1, Events: []collector.Event{{Operation: "github:search", Status: "SUCCESS"}}}}
	c, err := Run(context.Background(), f, Input{Name: "Anton"}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Search.Status != "EMPTY" || len(c.Evidence) != 0 {
		t.Fatal("empty collection fabricated facts")
	}
	f.result.Incomplete = true
	c, err = Run(context.Background(), f, Input{Name: "Anton"}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Search.Status != "PARTIAL" || c.Coverage[len(c.Coverage)-1].Status != "NOT_CHECKED" {
		t.Fatal("incomplete scope hidden")
	}
}

func TestWebMetadataCannotBecomePES(t *testing.T) {
	var cfg Config
	var policy domain.Policy
	if err := storage.Read("../../configs/collector-v1.json", &cfg); err != nil {
		t.Fatal(err)
	}
	if err := storage.Read("../../configs/experimental-v1.json", &policy); err != nil {
		t.Fatal(err)
	}
	f := &fakeCollector{result: collector.Result{Status: "SUCCESS", Requests: 1, Events: []collector.Event{{Operation: "google:web", Status: "SUCCESS"}}, Profiles: []collector.Profile{{Key: "web-test", URL: "https://example.org", SourceURL: "https://www.google.com/search?q=Anton", Facts: []collector.Fact{{Type: "search_title", Value: "Anton"}}}}}}
	c, err := Run(context.Background(), f, Input{Name: "Anton", Provider: "google"}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Candidates[0].DisplayName != "" || c.Candidates[0].Label != "Anton" {
		t.Fatal("page title treated as name")
	}
	c.Candidates[0].Decision = "CONFIRM"
	r, err := scoring.Calculate(c, policy)
	if err != nil {
		t.Fatal(err)
	}
	if r.PES != nil {
		t.Fatal("metadata-only profile was scored")
	}
}

func TestBlockedSearchRemainsBlocked(t *testing.T) {
	var cfg Config
	if err := storage.Read("../../configs/collector-v1.json", &cfg); err != nil {
		t.Fatal(err)
	}
	f := &fakeCollector{result: collector.Result{Status: "BLOCKED", Incomplete: true, Requests: 1, Events: []collector.Event{{Operation: "google:web", Status: "BLOCKED", Detail: "Challenge"}}}}
	c, err := Run(context.Background(), f, Input{Name: "Anton", Provider: "google"}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Search.Status != "BLOCKED" {
		t.Fatalf("failure hidden as %s", c.Search.Status)
	}
}
