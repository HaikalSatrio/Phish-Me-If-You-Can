package search

import (
	"context"
	"strings"
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

func TestRepeatedProviderEventsAggregateCoverage(t *testing.T) {
	var cfg Config
	if err := storage.Read("../../configs/collector-v1.json", &cfg); err != nil {
		t.Fatal(err)
	}
	f := &fakeCollector{result: collector.Result{Status: "SUCCESS", Requests: 2, Incomplete: true, Events: []collector.Event{
		{Operation: "searxng:web", Status: "SUCCESS"},
		{Operation: "searxng:web", Status: "SUCCESS"},
		{Operation: "duckduckgo:web", Status: "FAILED"},
	}}}
	c, err := Run(context.Background(), f, Input{Name: "Anton", Provider: "web"}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Coverage) != 3 || c.Coverage[0].Source != "searxng:web" || c.Coverage[0].Status != "SUCCESS" || c.Coverage[1].Status != "FAILED" || c.Coverage[2].Status != "NOT_CHECKED" {
		t.Fatalf("coverage was not aggregated: %+v", c.Coverage)
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

func TestWebCompanyInferenceStaysUnverifiedAndUnscored(t *testing.T) {
	var cfg Config
	var policy domain.Policy
	if err := storage.Read("../../configs/collector-v1.json", &cfg); err != nil {
		t.Fatal(err)
	}
	if err := storage.Read("../../configs/experimental-v1.json", &policy); err != nil {
		t.Fatal(err)
	}
	profileURL := "https://socs.binus.ac.id/people/yohan-muliono/"
	f := &fakeCollector{result: collector.Result{Status: "SUCCESS", Requests: 1, Events: []collector.Event{{Operation: "searxng:web", Status: "SUCCESS"}}, Profiles: []collector.Profile{{
		Key:       "web-yohan",
		URL:       profileURL,
		SourceURL: "https://search.example/query",
		Email:     "ymuliono@binus.edu",
		Facts: []collector.Fact{
			{Type: "search_title", Value: "Yohan Muliono"},
			{Type: "search_snippet", Value: "Yohan Muliono is Head of Cybersecurity Program in Bina Nusantara University, Indonesia."},
			{Type: "indexed_url", Value: profileURL},
		},
	}}}}
	c, err := Run(context.Background(), f, Input{Name: "Yohan Muliono", Provider: "web"}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	candidate := c.Candidates[0]
	if candidate.Company != "Bina Nusantara University" || !candidate.CompanyInferred || candidate.CompanyAmbiguous || candidate.Confidence != 0 {
		t.Fatalf("inferred company changed identity confidence or value: %+v", candidate)
	}
	if len(candidate.CompanyAlternatives) == 0 {
		t.Fatalf("inference alternatives were not retained: %+v", candidate)
	}
	if len(candidate.Contradictions) != 0 {
		t.Fatalf("inferred company was treated as a contradiction: %+v", candidate.Contradictions)
	}
	var inferred *domain.Evidence
	for i := range c.Evidence {
		if c.Evidence[i].ID == "web-yohan-workplace-inferred" {
			inferred = &c.Evidence[i]
		}
	}
	if inferred == nil || inferred.Status != "INFERRED" || inferred.Type != "workplace" || len(inferred.SupportingIDs) == 0 || len(inferred.Sources) != 0 {
		t.Fatalf("invalid inferred evidence: %+v", inferred)
	}
	if inferred.InferenceRule != "role-organization-phrase-v2" || inferred.InferenceQuote == "" || inferred.InferenceQuoteType != "search_snippet" || len(inferred.InferenceAlternatives) == 0 {
		t.Fatalf("missing inference audit trace: %+v", inferred)
	}
	c.Candidates[0].Decision = "CONFIRM"
	r, err := scoring.Calculate(c, policy)
	if err != nil {
		t.Fatal(err)
	}
	if r.IdentityStates["web-yohan"] != "ACCEPTED" {
		t.Fatalf("candidate was not accepted: %+v", r.IdentityStates)
	}
	excluded := false
	for _, id := range r.ExcludedIDs {
		if id == inferred.ID {
			excluded = true
		}
	}
	if !excluded {
		t.Fatal("inferred workplace was not excluded from scoring")
	}
	for _, dimension := range r.Dimensions {
		for _, id := range dimension.EvidenceIDs {
			if id == inferred.ID {
				t.Fatal("inferred workplace entered a PES dimension")
			}
		}
	}
}

func TestWebAmbiguousCompanyInferenceIsMarked(t *testing.T) {
	var cfg Config
	if err := storage.Read("../../configs/collector-v1.json", &cfg); err != nil {
		t.Fatal(err)
	}
	f := &fakeCollector{result: collector.Result{Status: "SUCCESS", Requests: 1, Events: []collector.Event{{Operation: "searxng:web", Status: "SUCCESS"}}, Profiles: []collector.Profile{{
		Key:       "web-nadia",
		URL:       "https://example.org/nadia-pratama",
		SourceURL: "https://search.example/query",
		Facts: []collector.Fact{
			{Type: "search_title", Value: "Nadia Pratama"},
			{Type: "page_text", Value: "Nadia Pratama works at Orbit Labs."},
			{Type: "search_snippet", Value: "Nadia Pratama works at Meridian Group."},
		},
	}}}}
	c, err := Run(context.Background(), f, Input{Name: "Nadia Pratama", Provider: "web"}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	candidate := c.Candidates[0]
	if !candidate.CompanyInferred || !candidate.CompanyAmbiguous || len(candidate.CompanyAlternatives) == 0 {
		t.Fatalf("ambiguity was hidden: %+v", candidate)
	}
	ambiguousNote := false
	for _, note := range candidate.Missing {
		if strings.Contains(note, "ambiguous") {
			ambiguousNote = true
		}
	}
	if !ambiguousNote {
		t.Fatalf("missing ambiguity note: %+v", candidate.Missing)
	}
	var inferred *domain.Evidence
	for i := range c.Evidence {
		if c.Evidence[i].ID == "web-nadia-workplace-inferred" {
			inferred = &c.Evidence[i]
		}
	}
	if inferred == nil || len(inferred.InferenceAlternatives) == 0 || inferred.InferenceAlternatives[0].Rule == "" || inferred.InferenceAlternatives[0].Confidence <= 0 {
		t.Fatalf("ambiguity trace was not persisted: %+v", inferred)
	}
	foundAlternative := false
	for _, alternative := range inferred.InferenceAlternatives {
		if alternative.Value == "Orbit Labs" && alternative.Rule == "employment-phrase-v1" {
			foundAlternative = true
		}
	}
	if !foundAlternative {
		t.Fatalf("alternative audit was not persisted: %+v", inferred.InferenceAlternatives)
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

func TestPageEvidenceKeepsProvenanceAndCannotScore(t *testing.T) {
	var cfg Config
	var policy domain.Policy
	if err := storage.Read("../../configs/collector-v1.json", &cfg); err != nil {
		t.Fatal(err)
	}
	if err := storage.Read("../../configs/experimental-v1.json", &policy); err != nil {
		t.Fatal(err)
	}
	f := &fakeCollector{result: collector.Result{Status: "SUCCESS", Requests: 2, Events: []collector.Event{{Operation: "web:page:1", Status: "SUCCESS"}}, Profiles: []collector.Profile{{Key: "page-test", URL: "https://example.org/anton", SourceURL: "https://search.example/query", Facts: []collector.Fact{{Type: "search_title", Value: "Anton"}, {Type: "page_text", Value: "Public biography"}, {Type: "page_links", Value: "https://example.org/about"}}, FactSources: map[string]collector.FactSource{"page_text": {URL: "https://example.org/anton/", ResponseSHA256: strings.Repeat("a", 64), Rule: "public-page-v2"}}}}}}
	c, err := Run(context.Background(), f, Input{Name: "Anton", Provider: "web"}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range c.Evidence {
		if e.Type == "page_text" && (e.Sources[0].URL != "https://example.org/anton/" || e.Sources[0].Rule != "public-page-v2") {
			t.Fatal("page attributed to search engine")
		}
	}
	c.Candidates[0].Decision, c.Candidates[0].Confidence = "CONFIRM", 1
	r, err := scoring.Calculate(c, policy)
	if err != nil || r.PES != nil {
		t.Fatalf("page content counted as personal facts: %+v %v", r, err)
	}
}

func TestResumeRejectsChangedSearXNGService(t *testing.T) {
	var cfg Config
	if err := storage.Read("../../configs/collector-local-search.json", &cfg); err != nil {
		t.Fatal(err)
	}
	f := &fakeCollector{result: collector.Result{Status: "FAILED"}}
	c, err := Run(context.Background(), f, Input{Name: "Anton", Provider: "web"}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg.SearXNGURL = "http://127.0.0.1:9999"
	if _, err := Run(context.Background(), f, Input{}, cfg, &c); err == nil || f.calls != 1 {
		t.Fatal("resume switched configured service")
	}
}
