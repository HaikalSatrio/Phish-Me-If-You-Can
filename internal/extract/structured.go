package extract

import (
	"encoding/json"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

func structuredRelations(subject string, doc Document) []domain.Relation {
	var people []domain.PersonRecord
	if len(doc.Text) > 64<<10 || json.Unmarshal([]byte(doc.Text), &people) != nil || len(people) > 20 {
		return nil
	}
	out := []domain.Relation{}
	for _, p := range people {
		if !strings.EqualFold(compact(p.Name), compact(subject)) {
			continue
		}
		quote, _ := json.Marshal(p)
		add := func(kind, org, role string) {
			r := domain.Relation{Subject: p.Name, Kind: kind, Organization: org, Role: role, Temporal: "unknown", EvidenceID: doc.ID, Quote: string(quote), Method: "jsonld/" + RelationVersion, Status: "UNVERIFIED"}
			if ValidateRelation(r, subject, doc) == nil {
				out = append(out, r)
			}
		}
		for _, employer := range p.Employers {
			role := ""
			if len(p.Employers) == 1 && len(p.JobTitles) == 1 {
				role = p.JobTitles[0]
			}
			add("employment", employer, role)
		}
		if len(p.Employers) != 1 || len(p.JobTitles) != 1 {
			for _, role := range p.JobTitles {
				add("occupation", "", role)
			}
		}
		for _, school := range p.Schools {
			add("education", school, "")
		}
		for _, member := range p.Memberships {
			add("membership", member, "")
		}
		for _, place := range p.BirthPlaces {
			add("birthplace", place, "")
		}
	}
	return out
}
