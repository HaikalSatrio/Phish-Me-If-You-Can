package extract

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

func TestJSONLDSubjectAndRelationSeparation(t *testing.T) {
	people := []domain.PersonRecord{{Name: "Ayu Lestari", JobTitles: []string{"Engineer"}, Employers: []string{"Orbit Labs"}, Schools: []string{"Nusantara University"}, BirthPlaces: []string{"Hope Hospital"}}, {Name: "Budi Santoso", JobTitles: []string{"Director"}, Employers: []string{"Contoso Ltd"}}}
	data, _ := json.Marshal(people)
	got := Relations("Ayu Lestari", []Document{{ID: "doc", Type: "person_jsonld", Text: string(data)}}, time.Now())
	if len(got) != 3 {
		t.Fatalf("missing structured claims: %+v", got)
	}
	for _, r := range got {
		if r.Organization == "Contoso Ltd" || r.Temporal != "unknown" || r.Method != "jsonld/"+RelationVersion {
			t.Fatalf("misattributed structured claim %+v", r)
		}
	}
	if got[0].Role != "Engineer" || got[0].Organization != "Orbit Labs" {
		t.Fatal("explicit relationship lost")
	}
}
