package extract

import (
	"strings"
	"testing"
	"time"
)

func TestRelationshipGrammarAcrossUnseenNames(t *testing.T) {
	asOf := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ text, typ, role, org, kind string }{
		{"Rina Prasetyo is a computer security researcher, entrepreneur, and pioneer in vulnerability disclosure.", "page_text", "computer security researcher", "", "occupation"},
		{"Rina Prasetyo is a researcher in vulnerability disclosure.", "page_text", "researcher in vulnerability disclosure", "", "occupation"},
		{"Rina Prasetyo is the founder and CEO of Aurora Security, a consulting company.", "page_description", "founder and CEO", "Aurora Security", "employment"},
		{"Rina Prasetyo is the Chief Policy Officer for Example Labs, a platform provider.", "search_snippet", "Chief Policy Officer", "Example Labs", "employment"},
		{"Rina Prasetyo is the president of the Example Foundation and serves on its board of directors.", "page_text", "president", "Example Foundation", "employment"},
		{"Rina Prasetyo is Example’s President.", "page_text", "President", "Example", "employment"},
		{"Rina Prasetyo is Example's President.", "page_text", "President", "Example", "employment"},
		{"Rina Prasetyo - Founder & CEO, Example Labs", "search_title", "Founder & CEO", "Example Labs", "employment"},
		{"Aurora Security founder and CEO Rina Prasetyo has extensive experience.", "page_description", "founder and CEO", "Aurora Security", "employment"},
		{"Rina Prasetyo (lahir 29 Juli 1975) adalah seorang penulis dan oleh masyarakat dikenal sebagai aktivis.", "page_text", "penulis", "", "occupation"},
		{"Rina Prasetyo - Digital Activist | Co-Founder, Example.ID & Other App | Startup Mentor", "search_title", "Digital Activist", "", "occupation"},
		{"Rina Prasetyo worked as Engineer at Orbit Labs from January 1, 2018 to December 31, 2022.", "page_text", "Engineer", "Orbit Labs", "employment"},
		{"Between 2015 and 2017, Rina Prasetyo served as a Cybersecurity Fellow at New America, a think tank.", "page_text", "Cybersecurity Fellow", "New America", "employment"},
		{"Rina Prasetyo is Head of Cybersecurity Program in Bina Nusantara University, Indonesia.", "page_text", "Head of Cybersecurity Program", "Bina Nusantara University", "employment"},
	} {
		t.Run(tc.text, func(t *testing.T) {
			doc := Document{ID: "source", Type: tc.typ, Text: tc.text}
			got := Relations("Rina Prasetyo", []Document{doc}, asOf)
			found := false
			for _, r := range got {
				if r.Role == tc.role && r.Organization == tc.org && r.Kind == tc.kind {
					found = true
				}
				if err := ValidateRelation(r, "Rina Prasetyo", doc); err != nil {
					t.Fatal(err)
				}
			}
			if !found {
				t.Fatalf("missing %s/%s/%s in %+v", tc.kind, tc.role, tc.org, got)
			}
		})
	}
	for _, text := range []string{
		"Sorie Rina Prasetyo is Engineer at Orbit Labs.",
		"Rina Prasetyo interviewed Budi Santoso, CEO of Orbit Labs.",
		"According to Aurora Security CEO Rina Prasetyo, Budi is a researcher.",
		"Former Aurora Security CEO Rina Prasetyo attended an event.",
	} {
		if got := Relations("Rina Prasetyo", []Document{{ID: "source", Type: "search_snippet", Text: text}}, asOf); len(got) > 0 {
			t.Fatalf("incorrect attribution: %+v", got)
		}
	}
}

func TestPassagesAreExactBoundedAndReachLaterMentions(t *testing.T) {
	intro := "Rina Prasetyo attends a conference."
	useful := "Rina Prasetyo is CEO of Orbit Labs. She is also a researcher."
	doc := Document{ID: "page", Type: "page_text", Text: intro + "\n" + strings.Repeat("Unrelated navigation.\n", 900) + useful}
	passages := SelectPassages("Rina Prasetyo", []Document{doc, {ID: "copy", Type: "search_snippet", Text: intro}})
	total := 0
	found := false
	seen := map[string]bool{}
	for _, p := range passages {
		if seen[p.Text] {
			t.Fatal("duplicate passage")
		}
		seen[p.Text] = true
		if !strings.Contains(doc.Text, p.Text) || len([]rune(p.Text)) > 1000 {
			t.Fatalf("nonliteral or oversized passage: %+v", p)
		}
		if strings.Contains(p.Text, "CEO of Orbit Labs") && strings.Contains(p.Text, "She is also") {
			found = true
		}
		total += len([]rune(p.Text))
	}
	if !found || total > 12000 || len(passages) > 24 {
		t.Fatalf("missing later context or exceeded bounds: %+v", passages)
	}
}
