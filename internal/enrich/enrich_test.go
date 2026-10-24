package enrich

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/scoring"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func fixture(text string) domain.Case {
	return domain.Case{SchemaVersion: "1", Target: domain.Target{ID: "case", Name: "Ayu Lestari"}, Candidates: []domain.Candidate{{ID: "ayu", Decision: "PENDING"}}, Evidence: []domain.Evidence{{ID: "doc", CandidateID: "ayu", Type: "page_text", Value: text, Status: "OBSERVED", Sources: []domain.Source{{URL: "https://example.org/ayu", ObservedAt: "2026-10-24T00:00:00Z", Collector: "fixture"}}}}}
}

func TestRelationsCannotBeForgedAcrossSourcesOrScored(t *testing.T) {
	c := fixture("Ayu Lestari works at Orbit Labs.")
	c.Candidates[0].Decision = "CONFIRM"
	c.Candidates[0].Confidence = 1
	c.Evidence[0].Confidence = 1
	c.Evidence[0].Discoverability = 1
	data, _ := json.Marshal([]domain.PersonRecord{{Name: "Ayu Lestari", Employers: []string{"Orbit Labs"}}})
	e := c.Evidence[0]
	e.ID = "structured"
	e.Type = "person_jsonld"
	e.Value = string(data)
	c.Evidence = append(c.Evidence, e)
	got, err := Run(context.Background(), c, extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var policy domain.Policy
	if err := storage.Read("../../configs/experimental-v1.json", &policy); err != nil {
		t.Fatal(err)
	}
	report, err := scoring.Calculate(got, policy)
	if err != nil {
		t.Fatal(err)
	}
	if report.PES != nil {
		t.Fatal("unverified relationships or structured page metadata were scored")
	}
	for _, mutate := range []func(*domain.Relation){func(r *domain.Relation) { r.EvidenceID = "missing" }, func(r *domain.Relation) { r.Organization = "Invented Corp" }, func(r *domain.Relation) { r.Status = "OBSERVED" }, func(r *domain.Relation) { r.CandidateID = "another-person" }} {
		bad := got
		bad.Relations = append([]domain.Relation(nil), got.Relations...)
		mutate(&bad.Relations[0])
		if _, err := evidence.Normalize(bad); err == nil {
			t.Fatal("forged relation accepted")
		}
	}
}

func TestHistoryAmbiguityAndReplay(t *testing.T) {
	for _, tc := range []struct {
		text, company, role string
		ambiguous           bool
	}{
		{"Ayu Lestari works at Orbit Labs.", "Orbit Labs", "", false},
		{"Ayu Lestari worked at Contoso Ltd from 2015 to 2018.", "", "", false},
		{"Ayu Lestari works at Orbit Labs. Ayu Lestari works at Meridian Group.", "", "", true},
		{"Ayu Lestari is President of Indonesia from 2014 to 2024.", "", "", false},
		{"Ayu Lestari was born at Hope Hospital.", "", "", false},
		{"Ayu Lestari works at Orbit Labs. Ayu Lestari worked at Orbit Labs from 2015 to 2018.", "", "", false},
	} {
		t.Run(tc.text, func(t *testing.T) {
			c := fixture(tc.text)
			original, _ := json.Marshal(c)
			got, err := Run(context.Background(), c, extract.Config{})
			if err != nil {
				t.Fatal(err)
			}
			p := got.Candidates[0]
			if p.Company != tc.company || p.Role != tc.role || p.CompanyAmbiguous != tc.ambiguous {
				t.Fatalf("incorrect summary %+v", p)
			}
			after, _ := json.Marshal(c)
			if string(original) != string(after) {
				t.Fatal("input mutated")
			}
			if len(got.Relations) == 0 {
				t.Fatal("relations lost")
			}
			again, err := Run(context.Background(), got, extract.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Relations, again.Relations) || !reflect.DeepEqual(got.Candidates, again.Candidates) {
				t.Fatal("replay changed relationships")
			}
		})
	}
}

func TestSummaryRoleUsesCanonicalEnglishAndKeepsSourceQuote(t *testing.T) {
	text := "Ayu Lestari menjabat sebagai Dosen di Universitas Nusantara."
	got, err := Run(context.Background(), fixture(text), extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	p := got.Candidates[0]
	if p.Role != "Lecturer" || !p.RoleInferred || p.Company != "Universitas Nusantara" {
		t.Fatalf("role summary was not canonicalized: %+v", p)
	}
	if len(got.Relations) != 1 || got.Relations[0].Role != "Dosen" || !strings.Contains(got.Relations[0].Quote, "Dosen") {
		t.Fatalf("source-language audit was not preserved: %+v", got.Relations)
	}
}

func TestSummaryRoleAlternativesUseCanonicalEnglish(t *testing.T) {
	text := "Ayu Lestari menjabat sebagai Dosen di Universitas Nusantara. Ayu Lestari menjabat sebagai Peneliti di Lembaga Riset."
	got, err := Run(context.Background(), fixture(text), extract.Config{})
	if err != nil {
		t.Fatal(err)
	}
	p := got.Candidates[0]
	if p.Role != "Lecturer; Researcher" || p.RoleTemporal != "multiple_claims" || len(p.RoleClaims) != 2 || p.Company != "" || !reflect.DeepEqual(p.RoleAlternatives, []string{"Lecturer", "Researcher"}) {
		t.Fatalf("role alternatives were not canonicalized: %+v", p)
	}
}

func TestProfileHeadlineSummary(t *testing.T) {
	cases := []struct {
		name, text, company, role string
	}{
		{"Yohan Muliono", "Yohan Muliono - Head of Cyber Security Program", "", "Head of Cyber Security Program"},
		{"Elon Musk", "Elon Musk - Co-founder and CEO of Tesla", "Tesla", "Co-founder and CEO"},
		{"Yohan Muliono", "Yohan Muliono - Head of Cyber Security Program 2014-2024", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			c := fixture(tc.text)
			c.Target.Name = tc.name
			c.Evidence[0].Type = "search_title"
			got, err := Run(context.Background(), c, extract.Config{})
			if err != nil {
				t.Fatal(err)
			}
			p := got.Candidates[0]
			if p.Company != tc.company || p.Role != tc.role {
				t.Fatalf("incorrect headline summary: %+v", p)
			}
			if tc.role != "" && p.RoleTemporal != "headline" {
				t.Fatalf("headline role lost its temporal marker: %+v", p)
			}
			if tc.company != "" && p.CompanyTemporal != "headline" {
				t.Fatalf("headline company lost its temporal marker: %+v", p)
			}
			if tc.role != "" && !strings.Contains(strings.Join(p.Missing, " "), "undated profile headline") {
				t.Fatalf("headline summary lacks an audit note: %+v", p.Missing)
			}
			if tc.role == "" && tc.company == "" && !strings.Contains(strings.Join(p.Missing, " "), "no unambiguous current or headline relationship") {
				t.Fatalf("historical headline was not withheld: %+v", p)
			}
			if len(got.Relations) != 1 || got.Relations[0].Temporal == "" {
				t.Fatalf("headline provenance lost: %+v", got.Relations)
			}
		})
	}
}

func TestLocalProposalsDoNotChangeSummaryOrIdentity(t *testing.T) {
	quote := "Ayu Lestari interviewed Budi Santoso, Chief Financial Officer at Contoso Ltd."
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"models":[]}`))
			return
		}
		proposal := map[string]any{"subject": "Ayu Lestari", "kind": "employment", "organization": "Contoso Ltd", "role": "Chief Financial Officer", "start_year": 0, "end_year": 0, "temporal": "present_assertion", "evidence_id": "doc", "quote": quote}
		content, _ := json.Marshal(map[string]any{"relations": []any{proposal}})
		json.NewEncoder(w).Encode(map[string]any{"done": true, "message": map[string]string{"content": string(content)}})
	}))
	defer srv.Close()
	c := fixture(quote)
	got, err := Run(context.Background(), c, extract.Config{Mode: "hybrid", URL: srv.URL, Model: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	p := got.Candidates[0]
	if p.Company != "" || p.Role != "" || p.Decision != "PENDING" || p.Confidence != 0 {
		t.Fatalf("unreviewed semantic error reached summary: %+v", p)
	}
	if len(got.Extraction) != 2 {
		t.Fatal("missing model audit")
	}
}

func TestLocalFailurePreservesRulesAndReportsBudget(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"models":[]}`))
			return
		}
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := fixture("Ayu Lestari works at Orbit Labs.")
	other := c.Candidates[0]
	other.ID = "other"
	c.Candidates = append(c.Candidates, other)
	e := c.Evidence[0]
	e.ID = "other-doc"
	e.CandidateID = "other"
	c.Evidence = append(c.Evidence, e)
	got, err := Run(context.Background(), c, extract.Config{Mode: "hybrid", URL: srv.URL, Model: "fixture", MaxRequests: 1})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(got.Extraction) != 4 || got.Extraction[1].Status != "FAILED" || got.Extraction[3].Status != "SKIPPED" {
		t.Fatalf("missing failure/budget diagnostics: %+v", got.Extraction)
	}
	for _, p := range got.Candidates {
		if p.Company != "Orbit Labs" {
			t.Fatal("model failure erased rules")
		}
	}
}
