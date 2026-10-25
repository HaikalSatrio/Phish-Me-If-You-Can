package benchmark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func sample() Corpus {
	return Corpus{Version: "1", Description: "Synthetic scorer fixture", ObservedAt: "2026-10-25T00:00:00Z", Cases: []Case{{ID: "one", Category: "employment", Name: "Mira Kenanga", Results: []Result{{ID: "a", URL: "https://profiles.example.org/mira", Title: "Mira Kenanga", Snippet: "Mira Kenanga works as Engineer at Willow Labs.", Keep: true, Expected: []Claim{{Subject: "Mira Kenanga", Kind: "employment", Role: "Engineer", Organization: "Willow Labs", Temporal: "present_assertion"}}}}}}}
}

func TestMetricsDeduplicateAndKeepCandidateAndPeriodBoundaries(t *testing.T) {
	a := Claim{CandidateID: "a", Subject: "Mira Kenanga", Kind: "employment", Role: "Engineer", Organization: "Willow Labs", Temporal: "headline"}
	b := a
	b.CandidateID = "b"
	past := a
	past.Temporal = "historical"
	var m Counts
	m.add(values([]Claim{a}, "claims"), values([]Claim{a, a, b, past}, "claims"))
	if m.TP != 1 || m.FP != 2 || m.FN != 0 || m.Precision == nil || *m.Precision != 1.0/3.0 {
		t.Fatalf("incorrect claim counts: %+v", m)
	}
	var empty Counts
	empty.add(map[string]bool{}, map[string]bool{})
	if empty.Precision != nil || empty.Recall != nil {
		t.Fatal("undefined rates must be null, not perfect or zero scores")
	}
}

func TestReplayRetainsErrorsAndDoesNotChangeLabels(t *testing.T) {
	c := sample()
	c.Cases[0].Results[0].Expected[0].Role = "Researcher"
	before, _ := json.Marshal(c)
	r, err := Execute(context.Background(), c, extract.Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := r.Metrics["rules"]
	if m.Claims.TP != 0 || m.Claims.FP != 1 || m.Claims.FN != 1 || len(r.Runs[0].Unexpected) != 1 || len(r.Runs[0].Missing) != 1 {
		t.Fatalf("wrong or missing claims were hidden: %+v", m)
	}
	after, _ := json.Marshal(c)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("benchmark mutated its labels")
	}
	if r.Runs[0].Output.Candidates[0].Decision != "PENDING" {
		t.Fatal("evaluation confirmed an identity")
	}
}

func TestUnavailableModelIsReportedAndInputsAreIdentical(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer s.Close()
	r, err := Execute(context.Background(), sample(), extract.Config{Model: "fixture", URL: s.URL, TimeoutSeconds: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := r.Metrics["hybrid"]
	if m.ModelFailed != 1 || m.ModelSuccess != 0 || r.Runs[1].Status != "DEGRADED" {
		t.Fatalf("model failure hidden: %+v", m)
	}
	if r.Runs[0].InputSHA256 != r.Runs[1].InputSHA256 {
		t.Fatal("rules and hybrid used different snapshots")
	}
	if m.Claims.TP != r.Metrics["rules"].Claims.TP {
		t.Fatal("fallback rules disappeared after model failure")
	}
}

func TestCancelledCasesAreNotSuccessfulAbstentions(t *testing.T) {
	c := sample()
	c.Cases[0].Results[0].Expected = []Claim{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := Execute(ctx, c, extract.Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Metrics["rules"].CorrectAbstentions != 0 || r.Metrics["rules"].Completed != 0 || r.Runs[0].Status != "ERROR" {
		t.Fatal("cancellation counted as a correct empty result")
	}
}

func TestBundledCorpusAndInvalidLabels(t *testing.T) {
	var c Corpus
	if err := storage.Read("../../fixtures/benchmark-exploratory-v1.json", &c); err != nil {
		t.Fatal(err)
	}
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
	if len(c.Cases) != 24 {
		t.Fatal("unexpected corpus size")
	}
	c.Cases[0].Results[0].Expected = nil
	if err := Validate(c); err == nil {
		t.Fatal("unlabeled result accepted as a negative")
	}
}
