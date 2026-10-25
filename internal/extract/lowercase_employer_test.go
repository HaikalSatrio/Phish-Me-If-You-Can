package extract

import (
	"testing"
	"time"
)

func TestLowercaseEmployerAndDescriptiveComma(t *testing.T) {
	for _, tc := range []struct{ text, org, role string }{
		{"Daphne Koller. CEO and Founder, insitro.", "insitro", "CEO and Founder"},
		{"Daphne Koller is CEO and Founder of insitro, a physical AI therapeutics company.", "insitro", "CEO and Founder"},
		{"Daphne Koller is CEO of Orbit Labs, a research company.", "Orbit Labs", "CEO"},
		{"Daphne Koller - Engineer, acme", "acme", "Engineer"},
		{"Daphne Koller - Engineer, remote", "", "Engineer"},
		{"Daphne Koller - Head of engineering", "", "Head of engineering"},
		{"Daphne Koller - Director of research", "", "Director of research"},
		{"Daphne Koller - Founder of community", "", "Founder of community"},
	} {
		got := Relations("Daphne Koller", []Document{{ID: "source", Type: "search_snippet", Text: tc.text}}, time.Now())
		if len(got) == 0 {
			t.Fatalf("no relation: %s", tc.text)
		}
		for _, r := range got {
			if r.Organization != tc.org || r.Role != tc.role {
				t.Fatalf("wrong employer boundary: %s %+v", tc.text, got)
			}
		}
	}
}
