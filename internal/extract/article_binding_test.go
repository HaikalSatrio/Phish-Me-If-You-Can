package extract

import (
	"testing"
	"time"
)

func TestArticleLeadExpansionNeedsMatchingHeadingAndBirthIntroduction(t *testing.T) {
	for _, tc := range []struct{ subject, full string }{
		{"Lina Torres", "Lina Maria Torres"}, {"Ana Pérez", "Ana María Pérez"}, {"Mira Solis", "Mira Solis-Hart"},
	} {
		title := Document{ID: "title", Type: "page_title", Text: tc.subject + " - Wikipedia"}
		body := Document{ID: "lead", Type: "page_description", Text: tc.full + " (born 1 January 1985) is a software engineer."}
		raw := "https://en.wikipedia.org/wiki/Example"
		got := BoundArticleRelations(tc.subject, raw, []Document{title, body}, time.Now())
		if len(got) != 1 || got[0].Subject != tc.subject || got[0].SubjectEvidenceID != title.ID || got[0].Role != "software engineer" {
			t.Fatalf("missing grounded expansion: %+v", got)
		}
		if err := ValidateArticleBinding(got[0], raw, body, title, time.Now()); err != nil {
			t.Fatal(err)
		}
		got[0].Role = "doctor"
		if ValidateArticleBinding(got[0], raw, body, title, time.Now()) == nil {
			t.Fatal("forged article role accepted")
		}
	}
}

func TestArticleLeadDoesNotLinkOtherPeopleOrSources(t *testing.T) {
	for _, tc := range []struct{ title, body, raw, typ string }{
		{"Mira Solis - Wikipedia", "Mira Solis's husband, Raul Solis (born 1980) is a doctor.", "https://en.wikipedia.org/wiki/Mira_Solis", "page_description"},
		{"Mira Solis - Wikipedia", "Another Mira Solis (born 1980) is a doctor.", "https://en.wikipedia.org/wiki/Mira_Solis", "page_description"},
		{"Mira Solis - Wikipedia", "Maria Solis (born 1980) is a doctor.", "https://en.wikipedia.org/wiki/Mira_Solis", "page_description"},
		{"Mira Solis - Wikipedia", "Mira Rosa Solis is a doctor.", "https://en.wikipedia.org/wiki/Mira_Solis", "page_description"},
		{"Other Person - Wikipedia", "Mira Rosa Solis (born 1980) is a doctor.", "https://en.wikipedia.org/wiki/Mira_Solis", "page_description"},
		{"Mira Solis - Wikipedia", "Mira Rosa Solis (born 1980) is a doctor.", "https://fan.example/wiki/Mira_Solis", "page_description"},
		{"Mira Solis - Wikipedia", "Mira Rosa Solis (born 1980) is a doctor.", "https://en.wikipedia.org/wiki/Talk:Mira_Solis", "page_description"},
		{"Mira Solis - Wikipedia", "Mira Rosa Solis (born 1980) is a doctor.", "https://en.wikipedia.org/wiki/Mira_Solis", "search_snippet"},
	} {
		docs := []Document{{ID: "title", Type: "page_title", Text: tc.title}, {ID: "lead", Type: tc.typ, Text: tc.body}}
		if got := BoundArticleRelations("Mira Solis", tc.raw, docs, time.Now()); len(got) > 0 {
			t.Fatalf("unbound name promoted: %+v", got)
		}
	}
}
