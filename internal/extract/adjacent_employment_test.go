package extract

import (
	"strings"
	"testing"
	"time"
)

func TestAdjacentEmploymentUsesOnlyUnambiguousLocalContext(t *testing.T) {
	for _, tc := range []struct {
		label, text string
		want        bool
	}{
		{"adjacent", "Mira Chen joined Pine Labs. She works there as Researcher.", true},
		{"plural pronoun", "Mira Chen joined Pine Labs. They work there as Researcher.", true},
		{"indonesian", "Mira Chen bergabung di Pine Labs. Ia bekerja di sana sebagai Researcher.", true},
		{"other full name", "Amelia Mira Chen joined Pine Labs. She works there as Researcher.", false},
		{"changed subject", "Mira Chen joined Pine Labs. Amelia Solis works there as Researcher.", false},
		{"intervening person", "Mira Chen joined Pine Labs. Amelia Solis joined too. She works there as Researcher.", false},
		{"companion", "Mira Chen joined Pine Labs with Amelia Solis. She works there as Researcher.", false},
		{"negated", "Mira Chen never joined Pine Labs. She works there as Researcher.", false},
		{"negated role", "Mira Chen joined Pine Labs. She does not work there as Researcher.", false},
		{"uncertain", "Mira Chen allegedly joined Pine Labs. She works there as Researcher.", false},
		{"paragraph break", "Mira Chen joined Pine Labs.\n\nShe works there as Researcher.", false},
		{"another organization", "Mira Chen joined Pine Labs. She works there as Researcher at Orbit Labs.", false},
		{"conditional", "If Mira Chen joined Pine Labs. She works there as Researcher.", false},
		{"question", "Mira Chen joined Pine Labs. She works there as Researcher?", false},
		{"unrecognized role", "Mira Chen joined Pine Labs. She works there as a possible future applicant.", false},
	} {
		t.Run(tc.label, func(t *testing.T) {
			doc := Document{ID: "paragraph", Type: "page_text", Text: tc.text}
			relations := Relations("Mira Chen", []Document{doc}, time.Date(2026, 11, 28, 0, 0, 0, 0, time.UTC))
			if !tc.want {
				if len(relations) != 0 {
					t.Fatalf("ambiguous pronoun produced a claim: %+v", relations)
				}
				return
			}
			if len(relations) != 1 || relations[0].Kind != "employment" || relations[0].Subject != "Mira Chen" || relations[0].Role != "Researcher" || relations[0].Organization != "Pine Labs" || relations[0].Temporal != "present_assertion" || relations[0].Status != "UNVERIFIED" || !strings.Contains(doc.Text, relations[0].Quote) {
				t.Fatalf("wrong adjacent relation: %+v", relations)
			}
		})
	}
}
