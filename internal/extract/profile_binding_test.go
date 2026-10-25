package extract

import (
	"strings"
	"testing"
	"time"
)

func TestProfileTitleAndBioBindWithoutInventingACombinedQuote(t *testing.T) {
	asOf := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ url, name, title, body, role, org string }{
		{"https://instagram.com/sari_dev", "Sari", "Sari (@sari_dev) | Instagram", `100 followers · @sari_dev: “Sharing thoughts and stories Cyber Security Lecturer ❣ Mom ...`, "Cyber Security Lecturer", ""},
		{"https://linkedin.com/in/sari-dev", "Sari Sari", "Sari Sari - CHFI | Deputy Head of Cyber Security Program at ...", "CHFI | Deputy Head of Cyber Security Program at Example University · Experience: Example University", "Deputy Head of Cyber Security Program", "Example University"},
	} {
		docs := []Document{{ID: "title", Type: "search_title", Text: tc.title}, {ID: "body", Type: "search_snippet", Text: tc.body}}
		r := BoundProfileRelations(tc.name, tc.url, docs, asOf)
		if len(r) != 1 || r[0].Role != tc.role || r[0].Organization != tc.org || r[0].SubjectEvidenceID != "title" || !strings.Contains(tc.body, r[0].Quote) {
			t.Fatalf("claim missing or fabricated: %+v", r)
		}
		if err := ValidateProfileBinding(r[0], tc.url, docs[1], docs[0]); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProfileBindingAbstainsOnWrongOwnerOtherPeopleAndTruncation(t *testing.T) {
	for _, tc := range []struct{ url, title, body string }{
		{"https://instagram.com/sari_dev", "Sari (@other) | Instagram", `@sari_dev: “Cyber Security Lecturer”`},
		{"https://instagram.com/p/post", "Sari (@sari_dev) | Instagram", `@sari_dev: “Cyber Security Lecturer”`},
		{"https://instagram.com/sari_dev", "Sari (@sari_dev) | Instagram", `@other: “Cyber Security Lecturer”`},
		{"https://instagram.com/sari_dev", "Sari (@sari_dev) | Instagram", `@sari_dev: “My husband is a Cyber Security Lecturer”`},
		{"https://instagram.com/sari_dev", "Sari (@sari_dev) | Instagram", `@sari_dev: “Not a Cyber Security Lecturer”`},
		{"https://linkedin.com/in/sari", "Sari - CHFI", "CHFI | Deputy Head of Cyber Security Program at ..."},
		{"https://linkedin.com/in/sari", "Sari Amelia - CHFI", "CHFI | Lecturer at Example University"},
	} {
		docs := []Document{{ID: "title", Type: "search_title", Text: tc.title}, {ID: "body", Type: "search_snippet", Text: tc.body}}
		if got := BoundProfileRelations("Sari", tc.url, docs, time.Now()); len(got) != 0 {
			t.Fatalf("unsafe attribution from %q: %+v", tc.body, got)
		}
	}
}

func TestLinkedInEducationRequiresCompleteLabeledInstitution(t *testing.T) {
	for _, body := range []string{
		"Undergraduate Student · Pendidikan: Example University ...",
		"Undergraduate Student · Pengalaman: Example University · Lokasi: Jakarta",
		"My friend is an Undergraduate Student · Pendidikan: Example University · Lokasi: Jakarta",
		"Undergraduate Student · Pendidikan: Example · Lokasi: Jakarta",
	} {
		docs := []Document{{ID: "title", Type: "search_title", Text: "Sari Amelia - Student"}, {ID: "body", Type: "search_snippet", Text: body}}
		for _, r := range BoundProfileRelations("Sari Amelia", "https://linkedin.com/in/sari", docs, time.Now()) {
			if r.Kind == "education" {
				t.Fatalf("unsupported education from %q: %+v", body, r)
			}
		}
	}
}
