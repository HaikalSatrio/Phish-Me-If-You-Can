package extract

import (
	"strings"
	"testing"
	"time"
)

func TestBiographyOccupationsWithAliases(t *testing.T) {
	for _, tc := range []struct {
		subject, text string
		roles         []string
	}{
		{"Lina Hart", "Lina Hart, known as Ember, is an American influencer and online streamer.", []string{"American influencer", "online streamer"}},
		{"Damar Wicaksono", "Damar Wicaksono, dikenal dengan nama panggung Biru, adalah seorang pelawak tunggal.", []string{"pelawak tunggal"}},
		{"Liam O'Connor", "Liam O'Connor, known as North, is an Irish singer-songwriter.", []string{"Irish singer-songwriter"}},
		{"Rafael Costa Jr.", "Rafael Costa Jr. (born March 3, 1990), known as Glass, is a musician.", []string{"musician"}},
		{"Ember", "Ember is known as a comedian with a distinctive humorous style.", []string{"comedian"}},
		{"Ember", "Ember is a fan of a singer.", nil},
		{"Ember", "Ember is known for interviewing a rapper.", nil},
		{"Ember", "Ember is an American singer and her husband is a doctor.", nil},
		{"Liam O'Connor", "Liam O'Connor (pronunciation /li.am/; born 1990), known as North, is an Irish singer-songwriter and activist.", []string{"Irish singer-songwriter", "activist"}},
	} {
		t.Run(tc.text, func(t *testing.T) {
			doc := Document{ID: "doc", Type: "search_snippet", Text: tc.text}
			got := Relations(tc.subject, []Document{doc}, time.Now())
			if len(got) != len(tc.roles) {
				t.Fatalf("want %q, got %+v", tc.roles, got)
			}
			for _, want := range tc.roles {
				found := false
				for _, r := range got {
					if r.Role == want && strings.Contains(doc.Text, r.Quote) {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing grounded role %q: %+v", want, got)
				}
			}
		})
	}
}

func TestFormerAliasDoesNotMakeOccupationHistorical(t *testing.T) {
	doc := Document{ID: "doc", Type: "search_snippet", Text: "Rafael Costa, known as Glass (formerly Crystal), is a singer and actor."}
	got := Relations("Rafael Costa", []Document{doc}, time.Now())
	if len(got) == 0 {
		t.Fatalf("missing occupations: %+v", got)
	}
	for _, r := range got {
		if r.Temporal != "present_assertion" || r.StartYear != 0 || r.EndYear != 0 {
			t.Fatalf("alias history became employment dates: %+v", r)
		}
	}
}
