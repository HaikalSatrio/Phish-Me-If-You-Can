package collector

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestBiographyGetsReservedSlotAndBoundedBudget(t *testing.T) {
	index := &anchorPage{response: func(q Query) Result {
		if q.Purpose == "biography" {
			if q.Name != "Awan" || q.Budget != 1 || !strings.Contains(discoveryPhrase(q), `"Awan" biography real name`) {
				t.Fatalf("bad biography query %+v", q)
			}
			return Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{URL: "https://biography.example/awan", Facts: []Fact{{"search_snippet", "Sari Utami, known as Awan, is a singer."}}}}}
		}
		r := Result{Status: "SUCCESS", Requests: 1}
		for i := 0; i < q.Limit; i++ {
			r.Profiles = append(r.Profiles, Profile{URL: fmt.Sprintf("https://video.example/awan/%d", i)})
		}
		return r
	}}
	page := &anchorPage{response: func(q Query) Result { return Result{Status: "SUCCESS", Requests: 1} }}
	d := Discovery{providers: []Collector{index}, page: page}
	r := d.Collect(context.Background(), Query{Name: "Awan", Limit: 5, Budget: 8})
	if len(r.Profiles) != 5 || len(index.queries) != 2 || len(r.Research) != 1 || r.Research[0].Action != "search_biography" || r.Requests > 8 {
		t.Fatalf("lost biography or exceeded cap: %+v", r)
	}
	if r.Profiles[0].URL != "https://biography.example/awan" {
		t.Fatal("explicit biography did not outrank video results")
	}
}

func TestBiographySkippedWhenEvidenceExistsOrBudgetSmall(t *testing.T) {
	for _, budget := range []int{1, 7, 8} {
		index := &anchorPage{response: func(q Query) Result {
			if q.Purpose != "" {
				t.Fatal("unnecessary biography query")
			}
			return Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{URL: "https://bio.example/awan", Facts: []Fact{{"search_snippet", "Sari Utami, known as Awan, is a singer."}}}}}
		}}
		page := &anchorPage{response: func(q Query) Result { return Result{Status: "SUCCESS", Requests: 1} }}
		d := Discovery{providers: []Collector{index}, page: page}
		r := d.Collect(context.Background(), Query{Name: "Awan", Limit: 5, Budget: budget})
		if r.Requests > budget {
			t.Fatal("budget exceeded")
		}
	}
}

func TestBiographyQueriesKeepRelevanceClueSeparate(t *testing.T) {
	q := Query{Name: "Awan", Purpose: "biography"}
	if !relevantSearchResult(q, "https://bio.example/awan", "Awan", "Sari Utami, known as Awan, is a singer.") {
		t.Fatal("retrieval suffix became identity tokens")
	}
	if relevantSearchResult(q, "https://bio.example/other", "Other", "biography real name") {
		t.Fatal("generic query words accepted as identity")
	}
	if phrases := searchPhrases(q); len(phrases) != 1 || phrases[0] != `"Awan" biography real name` {
		t.Fatalf("bad query expansion: %q", phrases)
	}
}

func TestFanpageDoesNotSuppressBiographyResearch(t *testing.T) {
	fan := Profile{URL: "https://fan.example/awan", Facts: []Fact{{"search_title", "Awan fan page"}, {"search_snippet", "Sari Utami, known as Awan, is a singer."}}}
	index := &anchorPage{response: func(q Query) Result {
		if q.Purpose == "biography" {
			return Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{{URL: "https://bio.example/awan", Facts: []Fact{{"search_snippet", "Sari Utami, known as Awan, is a singer."}}}}}
		}
		return Result{Status: "SUCCESS", Requests: 1, Profiles: []Profile{fan}}
	}}
	page := &anchorPage{response: func(q Query) Result { return Result{Status: "SUCCESS", Requests: 1} }}
	d := Discovery{providers: []Collector{index}, page: page}
	r := d.Collect(context.Background(), Query{Name: "Awan", Limit: 5, Budget: 8})
	if len(r.Research) != 1 || len(index.queries) != 2 || r.Profiles[0].URL != "https://bio.example/awan" {
		t.Fatalf("copied biography suppressed research: %+v", r)
	}
}
