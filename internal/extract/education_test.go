package extract

import (
	"testing"
	"time"
)

func TestExplicitStudyStatements(t *testing.T) {
	for _, tc := range []struct{ name, text, organization, temporal string }{
		{"course", "Mira Chen studies Computer Science at Harbor University.", "Harbor University", "present_assertion"},
		{"ongoing", "Mira Chen is studying Medicine at Harbor University.", "Harbor University", "present_assertion"},
		{"institution only", "Mira Chen studies at Harbor University.", "Harbor University", "present_assertion"},
		{"indonesian", "Mira Chen kuliah Teknik Informatika di Universitas Cendana.", "Universitas Cendana", "present_assertion"},
		{"since", "Mira Chen berkuliah di Universitas Cendana sejak 2024.", "Universitas Cendana", "present_assertion"},
		{"historical range", "Mira Chen studies Computer Science at Harbor University from 2018 to 2022.", "Harbor University", "historical"},
		{"other subject", "Amelia Mira Chen studies Computer Science at Harbor University.", "", ""},
		{"negated", "Mira Chen never studies Computer Science at Harbor University.", "", ""},
		{"uncertain", "Mira Chen might study Computer Science at Harbor University.", "", ""},
		{"conditional", "If Mira Chen studies Computer Science at Harbor University, she will move.", "", ""},
		{"research", "Mira Chen studies the effects of pollution at Harbor University.", "", ""},
		{"no institution", "Mira Chen studies Computer Science at Pine Labs.", "", ""},
		{"second person", "Mira Chen says Amelia studies Computer Science at Harbor University.", "", ""},
		{"multiple people", "Mira Chen and Amelia Solis study Computer Science at Harbor University.", "", ""},
		{"question", "Mira Chen studies Computer Science at Harbor University?", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := Document{ID: "study", Type: "page_text", Text: tc.text}
			relations := Relations("Mira Chen", []Document{doc}, time.Date(2026, 11, 28, 0, 0, 0, 0, time.UTC))
			if tc.organization == "" {
				if len(relations) != 0 {
					t.Fatalf("unsupported claim emitted: %+v", relations)
				}
				return
			}
			if len(relations) != 1 || relations[0].Kind != "education" || relations[0].Organization != tc.organization || relations[0].Role != "" || relations[0].Temporal != tc.temporal || relations[0].Status != "UNVERIFIED" || relations[0].EvidenceID != doc.ID {
				t.Fatalf("wrong education relation: %+v", relations)
			}
			if err := ValidateRelation(relations[0], "Mira Chen", doc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStatementAfterQuestionRemainsEligible(t *testing.T) {
	doc := Document{ID: "mixed", Type: "page_text", Text: "Mira Chen studies Computer Science at Harbor University? Mira Chen studies Computer Science at Cedar University."}
	got := Relations("Mira Chen", []Document{doc}, time.Now())
	if len(got) != 1 || got[0].Organization != "Cedar University" {
		t.Fatalf("question became an assertion or hid a later statement: %+v", got)
	}
}
