package extract

import (
	"testing"
	"time"
)

func TestProfileHeadlineRelations(t *testing.T) {
	cases := []struct {
		id, name, text, kind, org, role, state string
		start, end                             int
	}{
		{"headline-function", "Yohan Muliono", "Yohan Muliono - Head of Cyber Security Program", "occupation", "", "Head of Cyber Security Program", "headline", 0, 0},
		{"headline-function-short", "Yohan Muliono", "Yohan Muliono - Head of Cybersecurity", "occupation", "", "Head of Cybersecurity", "headline", 0, 0},
		{"headline-explicit-employer", "Yohan Muliono", "Yohan Muliono - Head of Cyber Security Program at Bina Nusantara University", "employment", "Bina Nusantara University", "Head of Cyber Security Program", "headline", 0, 0},
		{"headline-executive", "Elon Musk", "Elon Musk - Co-founder and CEO of Tesla", "employment", "Tesla", "Co-founder and CEO", "headline", 0, 0},
		{"headline-membership", "Brian Filbert Chandra", "Brian Filbert Chandra - Treasurer of HIMTI BINUS University", "membership", "HIMTI BINUS University", "Treasurer", "headline", 0, 0},
		{"headline-history", "Yohan Muliono", "Yohan Muliono - Head of Cyber Security Program 2014-2024", "occupation", "", "Head of Cyber Security Program", "historical", 2014, 2024},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			doc := Document{ID: "title", Type: "search_title", Text: tc.text}
			got := Relations(tc.name, []Document{doc}, time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC))
			if len(got) != 1 {
				t.Fatalf("expected one relationship, got %+v", got)
			}
			r := got[0]
			if r.Kind != tc.kind || r.Organization != tc.org || r.Role != tc.role || r.Temporal != tc.state || r.StartYear != tc.start || r.EndYear != tc.end {
				t.Fatalf("incorrect headline relationship: %+v", r)
			}
			if err := ValidateRelation(r, tc.name, doc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProfileHeadlineRejectsSkillList(t *testing.T) {
	doc := Document{ID: "title", Type: "search_title", Text: "Brian Filbert Chandra - Cybersecurity & Web Development"}
	if got := Relations("Brian Filbert Chandra", []Document{doc}, time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)); len(got) != 0 {
		t.Fatalf("skill list became a headline role: %+v", got)
	}
}

func TestRelationshipChallenge(t *testing.T) {
	cases := []struct {
		id, name, text, kind, org, role, state string
		start, end                             int
	}{
		{"employment", "Ayu Lestari", "Ayu Lestari works at Orbit Labs.", "employment", "Orbit Labs", "", "present_assertion", 0, 0},
		{"role-employer", "Ayu Lestari", "Ayu Lestari works as Chief Financial Officer at Orbit Labs.", "employment", "Orbit Labs", "Chief Financial Officer", "present_assertion", 0, 0},
		{"indonesian", "Ayu Lestari", "Ayu Lestari bekerja di PT Nusantara.", "employment", "PT Nusantara", "", "present_assertion", 0, 0},
		{"indonesian-role", "Ayu Lestari", "Ayu Lestari menjabat sebagai Direktur di PT Nusantara.", "employment", "PT Nusantara", "Direktur", "present_assertion", 0, 0},
		{"history", "Ayu Lestari", "Ayu Lestari worked at Contoso Ltd from 2015 to 2018.", "employment", "Contoso Ltd", "", "historical", 2015, 2018},
		{"former", "Ayu Lestari", "Ayu Lestari was Chief Financial Officer at Contoso Ltd.", "employment", "Contoso Ltd", "Chief Financial Officer", "historical", 0, 0},
		{"former-indonesian", "Ayu Lestari", "Ayu Lestari pernah bekerja di PT Nusantara.", "employment", "PT Nusantara", "", "historical", 0, 0},
		{"education", "Ayu Lestari", "Ayu Lestari studied at Nusantara University.", "education", "Nusantara University", "", "unknown", 0, 0},
		{"birth", "Ayu Lestari", "Ayu Lestari lahir di Rumah Sakit Harapan.", "birthplace", "Rumah Sakit Harapan", "", "unknown", 0, 0},
		{"membership", "Ayu Lestari", "Ayu Lestari is a member of Writers Guild.", "membership", "Writers Guild", "", "unknown", 0, 0},
		{"office-id", "Joko Widodo", "Joko Widodo adalah Presiden Republik Indonesia ke-7 pada 2014 sampai 2024.", "public_office", "", "Presiden Republik Indonesia ke-7", "historical", 2014, 2024},
		{"office-en", "Joko Widodo", "Joko Widodo served as President of Indonesia from 2014 to 2024.", "public_office", "", "President of Indonesia", "historical", 2014, 2024},
		{"alias-in-source", "Joko Widodo", "Joko Widodo, known as Jokowi, served as President of Indonesia from 2014 to 2024.", "public_office", "", "President of Indonesia", "historical", 2014, 2024},
		{"bio-id", "Joko Widodo", "Joko Widodo ( Indonesia: [ dʒɔkɔ widɔdɔ ] ; lahir 21 Juni 1961 ), lebih dikenal sebagai Jokowi adalah politikus dan pengusaha Indonesia yang menjabat sebagai Presiden Indonesia ketujuh dari tahun 2014 sampai 2024.", "public_office", "", "Presiden Indonesia ketujuh", "historical", 2014, 2024},
		{"bio-en", "Joko Widodo", "Joko Widodo ( Indonesian: [ ˌd͡ʒoko wiˈdodo ] ; born Mulyono , Indonesian: [ muˈljono ] ; 21 June 1961), often known mononymously as Jokowi ( Indonesian: [ d͡ʒoˈkowi ] ), is an Indonesian politician and businessman who served as the seventh president of Indonesia from 2014 to 2024.", "public_office", "", "seventh president of Indonesia", "historical", 2014, 2024},
		{"other-person", "Ayu Lestari", "Ayu Lestari interviewed Budi Santoso. Budi Santoso works at Contoso Ltd.", "", "", "", "", 0, 0},
		{"other-role", "Ayu Lestari", "Ayu Lestari interviewed Budi Santoso. Budi Santoso works as Chief Financial Officer at Contoso Ltd.", "", "", "", "", 0, 0},
		{"other-clause", "Ayu Lestari", "Ayu Lestari interviewed Budi Santoso, who works at Contoso Ltd.", "", "", "", "", 0, 0},
		{"negation", "Ayu Lestari", "Ayu Lestari does not work at Contoso Ltd.", "", "", "", "", 0, 0},
		{"uncertain", "Ayu Lestari", "Ayu Lestari reportedly works at Contoso Ltd.", "", "", "", "", 0, 0},
		{"hypothetical", "Ayu Lestari", "If Ayu Lestari works at Contoso Ltd, she could be promoted.", "", "", "", "", 0, 0},
		{"name-collision", "Ben", "Benjamin Harle works at Contoso Ltd.", "", "", "", "", 0, 0},
		{"unproven-alias", "Jokowi dodo", "Joko Widodo, known as Jokowi, served as President of Indonesia from 2014 to 2024.", "", "", "", "", 0, 0},
		{"table-neighbor", "Joko Widodo", "Joko Widodo | Presiden\nBudi Santoso | Chief Financial Officer | Contoso Ltd", "", "", "", "", 0, 0},
		{"birth-snippet", "Joko Widodo", "Sejak lahir pada 21 Juni 1961 di Rumah Sakit Brayat Minulyo, Joko Widodo tinggal bersama keluarganya di sebuah rumah kontrakan.", "", "", "", "", 0, 0},
		{"incidental-organization", "Joko Widodo", "Joko Widodo menghadiri rapat Kementerian Luar Negeri.", "", "", "", "", 0, 0},
	}
	legacyFP, newFP, matched := 0, 0, 0
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			doc := Document{ID: "snippet", Type: "search_snippet", Text: tc.text}
			got := Relations(tc.name, []Document{doc}, time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC))
			legacy, _ := ExtractCompany(Input{Name: tc.name, Facts: map[string]string{"search_snippet": tc.text}})
			wantCurrent := ""
			if tc.kind == "employment" && tc.state == "present_assertion" {
				wantCurrent = tc.org
			}
			if legacy.Value != "" && !sameValue(legacy.Value, wantCurrent) {
				legacyFP++
			}
			if tc.kind == "" {
				if len(got) != 0 {
					t.Fatalf("incorrect attribution: %+v", got)
				}
				return
			}
			found := false
			for _, r := range got {
				if r.Kind == "employment" && r.Temporal == "present_assertion" && !sameValue(r.Organization, wantCurrent) {
					newFP++
				}
				if r.Kind == tc.kind && r.Organization == tc.org && r.Role == tc.role && r.Temporal == tc.state && r.StartYear == tc.start && r.EndYear == tc.end {
					found = true
				}
				if err := ValidateRelation(r, tc.name, doc); err != nil {
					t.Fatal(err)
				}
			}
			if !found {
				t.Fatalf("missing expected relationship: %+v", got)
			}
			matched++
		})
	}
	t.Logf("Challenge regression, not held-out accuracy: cases=%d positive_cases_matched=%d legacy_incorrect_current_company=%d new_incorrect_current_company=%d", len(cases), matched, legacyFP, newFP)
	if legacyFP == 0 || newFP != 0 {
		t.Fatal("challenge did not discriminate attribution errors")
	}
}
