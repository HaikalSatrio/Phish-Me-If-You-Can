package extract

import (
	"strings"
	"testing"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

func TestInternationalIntroductionsKeepCitationsAndRoleBoundaries(t *testing.T) {
	for _, tc := range []struct{ subject, text, role string }{
		{"Luka Petrović", "Luka Petrović (Croatian pronunciation: [luka]; [4][5] born 9 September 1985) is a Croatian professional footballer who plays for Example FC.", "Croatian professional footballer"},
		{"Yuval Levi", "Yuval Levi[a] (born 1976) [1] is an Israeli historian, writer and public speaker.", "Israeli historian"},
		{"Mira Solis", "Mira Solis [ b ] (born ( 1982-12-01 ) 1 December 1982 ) is a singer.", "singer"},
		{"Ana María Pérez", "Ana María Pérez[12] is a Spanish chef with a distinctive cooking style.", "Spanish chef"},
	} {
		doc := Document{ID: "doc", Type: "page_description", Text: tc.text}
		got := Relations(tc.subject, []Document{doc}, time.Now())
		if len(got) != 1 || got[0].Role != tc.role || got[0].Organization != "" || !strings.Contains(doc.Text, got[0].Quote) {
			t.Fatalf("%s: %+v", tc.subject, got)
		}
	}
	for _, text := range []string{"Mira Solis[not the actor] is a doctor.", "Mira Solis[invented] is a singer.", "Mira Solis Jr. is a singer.", "Mira Solis (friend of Ana is a doctor."} {
		if got := Relations("Mira Solis", []Document{{ID: "doc", Type: "page_text", Text: text}}, time.Now()); len(got) > 0 {
			t.Fatalf("unbound annotation accepted: %+v", got)
		}
	}
}

func TestModelFormerAliasDoesNotOverrideProvedOccupationTense(t *testing.T) {
	doc := Document{ID: "doc", Type: "search_snippet", Text: "Rafael Costa, known as Glass (formerly Crystal), is a singer."}
	r := domain.Relation{Subject: "Rafael Costa", Kind: "occupation", Role: "singer", Temporal: "historical", Quote: doc.Text, EvidenceID: doc.ID}
	got, _, code, detail := groundProposal(r, r.Subject, doc, time.Now(), 0)
	if code != "" || got.Temporal != "present_assertion" {
		t.Fatalf("alias dated occupation: %+v %s %s", got, code, detail)
	}
}

func TestRolesExcludeInterfaceLabelsAndKeepPrimaryTense(t *testing.T) {
	for _, text := range []string{"Mira Solis - Player profile", "Mira Solis - Player profile 26/27", "Mira Solis - Engineer biography"} {
		if got := Relations("Mira Solis", []Document{{ID: "doc", Type: "page_title", Text: text}}, time.Now()); len(got) != 0 {
			t.Fatalf("UI label became a role: %+v", got)
		}
	}
	for _, tc := range []struct{ text, role, org, state string }{
		{"Mira Solis is a professional basketball player for the Example Stars of the National Basketball Association (NBA).", "professional basketball player", "Example Stars", "present_assertion"},
		{"Mira Solis is a 38-year old football player aus France.", "football player", "", "present_assertion"},
		{"Mira Solis is an environmental activist who worked to address climate change.", "environmental activist", "", "present_assertion"},
		{"Mira Solis is a former professional tennis player.", "former professional tennis player", "", "historical"},
		{"From 2018 to 2022, Mira Solis worked as Engineer at Orbit Labs.", "Engineer", "Orbit Labs", "historical"},
	} {
		got := Relations("Mira Solis", []Document{{ID: "doc", Type: "page_text", Text: tc.text}}, time.Now())
		if len(got) != 1 || got[0].Role != tc.role || got[0].Organization != tc.org || got[0].Temporal != tc.state {
			t.Fatalf("%s: %+v", tc.text, got)
		}
	}
	if got := organizationField("Example Institute (EI)"); got != "Example Institute (EI)" {
		t.Fatalf("parenthetical organization damaged: %q", got)
	}
	if got := sportsTeamField("tennis player", "University of the Arts"); got != "University of the Arts" {
		t.Fatalf("organization name truncated: %q", got)
	}
}

func TestAuditedNarrativeBoundaries(t *testing.T) {
	for _, tc := range []struct{ text, role, org, state string }{
		{"Mira Solis is an American actress known for her precise, controlled performances.", "American actress", "", "present_assertion"},
		{"Mira Solis is a chef renowned for innovative cuisine.", "chef", "", "present_assertion"},
		{"Mira Solis is a software engineer, researcher, and former Senior Engineer at Orbit Labs.", "software engineer", "", "present_assertion"},
		{"Mira Solis is Chef and owner of Orbit Restaurant (located in the historic city center), which has three stars.", "Chef and owner", "Orbit Restaurant", "present_assertion"},
		{"Mira Solis is an Engineer for Orbit Labs.", "Engineer", "Orbit Labs", "present_assertion"},
	} {
		got := Relations("Mira Solis", []Document{{ID: "doc", Type: "page_description", Text: tc.text}}, time.Now())
		if len(got) != 1 || got[0].Role != tc.role || got[0].Organization != tc.org || got[0].Temporal != tc.state {
			t.Fatalf("%s: %+v", tc.text, got)
		}
	}
	doc := Document{ID: "doc", Type: "page_description", Text: "Mira Solis was a singer from January 1, 2018 to December 31, 2022."}
	got := Relations("Mira Solis", []Document{doc}, time.Now())
	if len(got) != 1 || got[0].StartYear != 2018 || got[0].EndYear != 2022 || got[0].Temporal != "historical" {
		t.Fatalf("punctuated occupation dates were lost: %+v", got)
	}
}

func TestModelAuditedKindsAndClauseScope(t *testing.T) {
	for _, tc := range []struct{ text, kind, role, org, code string }{
		{"Mira Solis is a footballer who plays as a midfielder for Orbit FC.", "employment", "plays as", "Orbit FC", "ROLE_IS_VERB"},
		{"Mira Solis is a programmer and co-creator of the Orbit framework.", "education", "Orbit framework", "co-creator", "EDUCATION_UNSUPPORTED"},
		{"Mira Solis formerly worked at Orbit Labs. She is a professor at Example University.", "employment", "professor", "Example University", ""},
		{"Mira Solis worked at Example University. She is a professor at Orbit College.", "employment", "professor", "Example University", "RELATION_SCOPE"},
		{"Mira Solis attended Example University and earned a degree in mathematics.", "education", "", "Example University", ""},
		{"Mira Solis accepts an Honorary Doctorate from Example University.", "education", "Honorary Doctorate", "Example University", ""},
		{"Mira Solis melanjutkan pendidikannya di Example University.", "education", "", "Example University", ""},
		{"Mira Solis. Actress: Example Movie. French actress Mira Solis was born in Paris.", "occupation", "Actress", "", ""},
		{"Mira Solis was an executive. She previously was a director at Orbit Inc.", "employment", "director", "Orbit Inc.", ""},
		{"Mira Solis has several roles. She is an Engineer at Orbit Labs. She was formerly an Engineer at Orbit Labs.", "employment", "Engineer", "Orbit Labs", "RELATION_SCOPE"},
	} {
		doc := Document{ID: "doc", Type: "page_text", Text: tc.text}
		r := domain.Relation{Subject: "Mira Solis", Kind: tc.kind, Role: tc.role, Organization: tc.org, Temporal: "present_assertion", Quote: tc.text, EvidenceID: doc.ID}
		got, _, code, detail := groundProposal(r, r.Subject, doc, time.Now(), 0)
		if code != tc.code || code == "" && tc.role == "professor" && got.Temporal != "present_assertion" {
			t.Fatalf("%s: %+v %s %s", tc.text, got, code, detail)
		}
	}
}
