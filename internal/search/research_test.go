package search

import (
	"context"
	"strings"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/collector"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func TestDiscoveredFullNameAndEducationKeepTheirOwnSources(t *testing.T) {
	var cfg Config
	if err := storage.Read("../../configs/collector-v1.json", &cfg); err != nil {
		t.Fatal(err)
	}
	profile := collector.Profile{Key: "bio-laila", Name: "Laila Kartika", URL: "https://laila.example/about", SourceURL: "https://search.example/", LinkedFrom: "https://github.com/lailak", DiscoveryBasis: "linked biography; identity unverified", Facts: []collector.Fact{
		{Type: "page_text", Value: "Laila Kartika works as Engineer at Orbit Labs."},
		{Type: "person_jsonld", Value: `[{"name":"Laila Kartika","schools":["Example University"]}]`},
		{Type: "page_links", Value: "https://github.com/lailak"},
	}, FactSources: map[string]collector.FactSource{
		"name":          {URL: "https://laila.example/about", ResponseSHA256: strings.Repeat("a", 64), Rule: "public-page-v5"},
		"page_text":     {URL: "https://laila.example/about", ResponseSHA256: strings.Repeat("a", 64), Rule: "public-page-v5"},
		"person_jsonld": {URL: "https://laila.example/about", ResponseSHA256: strings.Repeat("a", 64), Rule: "public-page-v5"},
		"page_links":    {URL: "https://laila.example/about", ResponseSHA256: strings.Repeat("a", 64), Rule: "public-page-v5"},
	}}
	f := &fakeCollector{result: collector.Result{Status: "SUCCESS", Requests: 3, Profiles: []collector.Profile{
		{Key: "seed-laila", URL: "https://github.com/lailak", SourceURL: "https://github.com/lailak", Facts: []collector.Fact{{Type: "page_title", Value: "lailak - GitHub"}}}, profile,
	}, Research: []domain.ResearchStep{{Action: "follow_link", FromURL: "https://github.com/lailak", URL: profile.URL, Reason: "linked biography", Status: "SUCCESS", Requests: 1}}}}
	c, err := Run(context.Background(), f, Input{Name: "Laila", Provider: "web", Accounts: []domain.AccountClue{{Platform: "github", Username: "lailak"}}}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	seed, bio := c.Candidates[0], c.Candidates[1]
	if seed.DisplayName != "" || seed.Company != "" || len(seed.EducationClaims) != 0 {
		t.Fatal("biography claims silently transferred to supplied account")
	}
	if bio.DisplayName != "Laila Kartika" || bio.Company != "Orbit Labs" || len(bio.EducationClaims) != 1 || bio.EducationClaims[0].Value != "Example University" {
		t.Fatalf("useful fields lost: %+v", bio)
	}
	if bio.Decision != "PENDING" || bio.LinkedFrom != profile.LinkedFrom || len(c.Search.Research) != 1 {
		t.Fatal("identity was confirmed or trail lost")
	}
	if len(bio.Matches) != 1 || !strings.Contains(bio.Matches[0], "bio-laila-page_links") || bio.Confidence != 0 {
		t.Fatal("backlink lost or treated as verified identity")
	}
	for _, relation := range c.Relations {
		if relation.Subject != "Laila Kartika" || relation.CandidateID != bio.ID {
			t.Fatalf("partial input replaced observed subject: %+v", relation)
		}
	}
	for _, e := range c.Evidence {
		if e.CandidateID == bio.ID && e.Type == "name" && (e.Sources[0].URL != profile.URL || e.Sources[0].ResponseSHA256 == "") {
			t.Fatal("name attributed to search engine instead of biography")
		}
	}
	c.Candidates[1].EducationClaims[0].Value = "Invented University"
	if _, err := evidence.Normalize(c); err == nil {
		t.Fatal("forged education summary accepted")
	}
}
