package extract

import (
	"testing"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

func TestModelRejectsRelativeClaimsAndRolesAsSchools(t *testing.T) {
	for _, tc := range []struct{ text, kind, role, org, code string }{
		{"Mira Kenanga profile. My husband is a Software Engineer.", "occupation", "Software Engineer", "", "RELATIVE_ATTRIBUTION"},
		{"Mira Kenanga. Her mother works at Willow Labs.", "employment", "", "Willow Labs", "RELATIVE_ATTRIBUTION"},
		{"Mira Kenanga. Suami bekerja sebagai Software Engineer.", "occupation", "Software Engineer", "", "RELATIVE_ATTRIBUTION"},
		{"Mira Kenanga - Undergraduate Computer Science Student", "education", "Student", "Undergraduate Computer Science Student", "EDUCATION_IS_ROLE"},
		{"Mira Kenanga works as Researcher at Willow Labs. Her husband is a Teacher.", "employment", "Researcher", "Willow Labs", ""},
		{"Mira Kenanga is a Student at Harbor University.", "education", "Student", "Harbor University", ""},
	} {
		r := domain.Relation{Subject: "Mira Kenanga", EvidenceID: "doc", Quote: tc.text, Kind: tc.kind, Role: tc.role, Organization: tc.org, Temporal: "present_assertion"}
		_, _, code, detail := groundProposal(r, r.Subject, Document{ID: "doc", Type: "page_text", Text: tc.text}, time.Now(), 0)
		if code != tc.code {
			t.Fatalf("%s: code=%s want=%s detail=%s", tc.text, code, tc.code, detail)
		}
	}
}
