package extract

import (
	"strings"
	"testing"
	"time"
)

func TestProfileBlocksKeepEmploymentAttachedToSubject(t *testing.T) {
	for _, tc := range []struct{ name, text, org, role string }{
		{"labels", "Ayu Lestari\nRole: Senior Engineer\nCompany: Orbit Labs", "Orbit Labs", "Senior Engineer"},
		{"indonesian", "Ayu Lestari\nPerusahaan: PT Cakrawala\nJabatan: Director", "PT Cakrawala", "Director"},
		{"credentials", "Home\nAyu Lestari, S.Kom., M.TI., CND, CEHmaster, eMAPT.\nHead of Cyber Security Program\nD5543", "", "Head of Cyber Security Program"},
		{"explicit employer", "Ayu Lestari\nEngineer at Orbit Labs\nOther people\nBudi Santoso\nDirector at Other Corp", "Orbit Labs", "Engineer"},
		{"other person", "Ayu Lestari\nBudi Santoso\nRole: Director\nCompany: Other Corp", "", ""},
		{"longer name", "Ayu Lestari Putri\nRole: Director\nCompany: Other Corp", "", ""},
		{"narrative", "Ayu Lestari\nVisited our office\nCompany: Other Corp", "", ""},
		{"sponsor", "Ayu Lestari\nSponsors\nCompany: Other Corp", "", ""},
		{"uncertain", "Ayu Lestari\nCompany: Not Orbit Labs", "", ""},
		{"bare org", "Ayu Lestari\nOrbit Labs", "", ""},
		{"placeholder", "Ayu Lestari\nCompany: Unknown", "", ""},
		{"conflicting labels", "Ayu Lestari\nCompany: Orbit Labs\nCompany: Other Corp", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := Document{ID: "source", Type: "page_text", Text: tc.text}
			got := Relations("Ayu Lestari", []Document{doc}, time.Now())
			if tc.org == "" && tc.role == "" {
				if len(got) != 0 {
					t.Fatalf("unrelated relationship extracted: %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Organization != tc.org || got[0].Role != tc.role || got[0].Temporal != "headline" || !strings.Contains(tc.text, got[0].Quote) {
				t.Fatalf("profile relationship lost or changed: %+v", got)
			}
		})
	}
}

func TestLongPagePassagesKeepAdjacentProfileFields(t *testing.T) {
	block := "Ayu Lestari, S.Kom., M.TI.\nRole: Engineer\nCompany: Orbit Labs"
	doc := Document{ID: "long-page", Type: "page_text", Text: strings.Repeat("Navigation content.\n", 100) + block + "\nBudi Santoso\nRole: Director\nCompany: Other Corp"}
	passages := SelectPassages("Ayu Lestari", []Document{doc})
	for _, p := range passages {
		if p.Text == block && p.EvidenceID == doc.ID {
			return
		}
	}
	t.Fatalf("profile fields were separated from name: %+v", passages)
}
