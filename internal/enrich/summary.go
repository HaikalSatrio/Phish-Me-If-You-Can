package enrich

import (
	"sort"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
)

type fieldSummary struct {
	value        string
	temporal     string
	ids          []string
	alternatives []string
}

func relationFieldValue(field string, r domain.Relation) string {
	if field == "company" {
		if r.Kind == "employment" {
			return r.Organization
		}
		return ""
	}
	switch r.Kind {
	case "employment", "public_office", "occupation", "membership", "education":
		return extract.CanonicalRole(r.Role)
	}
	return ""
}

func summaryRelation(r domain.Relation) bool {
	return r.Status == "UNVERIFIED" && (r.Method == extract.RelationVersion || r.Method == "jsonld/"+extract.RelationVersion)
}

func summaryKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func summaryField(candidateID, field string, relations []domain.Relation) fieldSummary {
	present := map[string]string{}
	presentIDs := map[string][]string{}
	otherIDs := map[string][]string{}
	for _, r := range relations {
		if r.CandidateID != candidateID || !summaryRelation(r) {
			continue
		}
		value := relationFieldValue(field, r)
		if value == "" {
			continue
		}
		key := summaryKey(value)
		if r.Temporal == "present_assertion" {
			if _, ok := present[key]; !ok {
				present[key] = value
			}
			presentIDs[key] = append(presentIDs[key], r.ID)
			continue
		}
		otherIDs[key] = append(otherIDs[key], r.ID)
	}
	if len(present) == 0 {
		return fieldSummary{}
	}
	keys := make([]string, 0, len(present))
	for key := range present {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 1 {
		out := fieldSummary{}
		for _, key := range keys {
			out.alternatives = append(out.alternatives, present[key])
		}
		return out
	}
	key := keys[0]
	ids := append([]string(nil), presentIDs[key]...)
	if len(otherIDs[key]) > 0 {
		ids = append(ids, otherIDs[key]...)
		sort.Strings(ids)
		return fieldSummary{temporal: "conflicting", ids: ids}
	}
	sort.Strings(ids)
	return fieldSummary{value: present[key], temporal: "present_assertion", ids: ids}
}

func projectSummary(p *domain.Candidate, relations []domain.Relation) {
	company := summaryField(p.ID, "company", relations)
	role := summaryField(p.ID, "role", relations)
	p.CompanyAlternatives = company.alternatives
	p.RoleAlternatives = role.alternatives
	p.CompanyAmbiguous = len(company.alternatives) > 1
	if p.Company == "" && company.value != "" {
		p.Company = company.value
		p.CompanyInferred = true
		p.CompanyTemporal = company.temporal
		p.CompanyRelationIDs = company.ids
	}
	if p.Role == "" && role.value != "" {
		p.Role = role.value
		p.RoleInferred = true
		p.RoleTemporal = role.temporal
		p.RoleRelationIDs = role.ids
	}
	notes := map[string]bool{}
	if p.CompanyInferred || p.RoleInferred {
		notes["Extraction: summary fields are unverified source claims; inspect each field's period and relationship references"] = true
	}
	if len(company.alternatives) > 1 || len(role.alternatives) > 1 {
		notes["Extraction: competing present-tense values remain ambiguous; only the affected summary field is withheld"] = true
	}
	if company.temporal == "conflicting" || role.temporal == "conflicting" {
		notes["Extraction: the value is supported but source claims disagree on its period"] = true
	}
	if p.Company == "" && p.Role == "" && len(company.alternatives) == 0 && len(role.alternatives) == 0 && company.temporal != "conflicting" && role.temporal != "conflicting" {
		notes["Extraction: no unambiguous present-tense relationship; inspect history and proposals"] = true
	}
	for note := range notes {
		p.Missing = append(p.Missing, note)
	}
	sort.Strings(p.Missing)
}
