package extract

import (
	"strings"
	"testing"
	"time"
)

func TestPublisherAndTaskDescriptionAreNotSummaryFields(t *testing.T) {
	for _, tc := range []struct{ text, role, org string }{
		{"Ayu Lestari - Executive Woman Media", "", ""},
		{"Ayu Lestari | Executive Magazine", "", ""},
		{"Ayu Lestari - Chief Executive Officer at Orbit Labs", "Chief Executive Officer", "Orbit Labs"},
		{"Ayu Lestari is a Principal Engineer at Orbit Labs working on its cloud platform.", "Principal Engineer", "Orbit Labs"},
		{"Ayu Lestari works at Orbit Labs where she leads research.", "", "Orbit Labs"},
		{"Ayu Lestari - Executive Director", "Executive Director", ""},
	} {
		got := Relations("Ayu Lestari", []Document{{ID: "doc", Type: "search_title", Text: tc.text}}, time.Now())
		if tc.role == "" && tc.org == "" {
			if len(got) != 0 {
				t.Fatalf("publisher treated as role: %+v", got)
			}
			continue
		}
		if len(got) == 0 {
			t.Fatalf("lost explicit claim: %s", tc.text)
		}
		for _, r := range got {
			if r.Role != tc.role || r.Organization != tc.org {
				t.Fatalf("wrong boundary: %s %+v", tc.text, got)
			}
		}
	}
}

func TestPersonalBioPassageDoesNotBorrowDatesFromOtherJobs(t *testing.T) {
	intro := "Ayu Lestari\nI'm currently the co-founder of Orbit Labs"
	text := intro + ". I previously worked at Other Corp and left the company in 2023."
	docs := []Document{{ID: "title", Type: "page_title", Text: "Ayu Lestari"}, {ID: "body", Type: "page_text", Text: text}}
	passages := SelectPassages("Ayu Lestari", docs)
	if len(passages) == 0 || passages[0].Text != intro || passages[0].EvidenceID != "body" || strings.Contains(passages[0].Text, "2023") {
		t.Fatalf("personal introduction not prioritized: %+v", passages)
	}
}

func TestFirstPersonBiographyRequiresPersonalPageContext(t *testing.T) {
	for _, tc := range []struct {
		title, body string
		want        bool
	}{
		{"Ayu Lestari", "Ayu Lestari\nI'm a developer living in Jakarta. I'm currently the co-founder\nof Orbit Labs .", true},
		{"Ayu Lestari", "Ayu Lestari\nI am CEO of Orbit Labs.", true},
		{"Ayu Lestari", "Ayu Lestari\nI'm a developer. I worked at Other Corp from 2010 to 2015. I'm currently the co-founder of Orbit Labs.", true},
		{"Interview with Ayu Lestari", "Ayu Lestari\nI am CEO of Orbit Labs.", false},
		{"Ayu Lestari", "Budi Santoso\nI am CEO of Orbit Labs.", false},
		{"Ayu Lestari", "Ayu Lestari\nI am writing a fictional story. I am CEO of Orbit Labs.", false},
		{"Ayu Lestari", "Ayu Lestari\nMy colleague said: I am CEO of Orbit Labs.", false},
		{"Ayu Lestari", "Ayu Lestari\nI'm not CEO of Orbit Labs.", false},
		{"Ayu Lestari", "Ayu Lestari\nI'm Head of engineering.", false},
	} {
		docs := []Document{{ID: "title", Type: "page_title", Text: tc.title}, {ID: "body", Type: "page_text", Text: tc.body}}
		got := personalBiographyRelations("Ayu Lestari", docs, time.Now())
		if (len(got) == 1) != tc.want {
			t.Fatalf("wrong first-person attribution: %q %+v", tc.body, got)
		}
		if tc.want && (got[0].Organization != "Orbit Labs" || got[0].Temporal != "present_assertion") {
			t.Fatalf("wrong employer or period: %+v", got)
		}
	}
}
