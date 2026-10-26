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
	if r.Status != "UNVERIFIED" || domain.RelationDecision(r) == "REJECT" {
		return false
	}
	return domain.RelationDecision(r) == "ACCEPT" || r.Method == extract.RelationVersion || r.Method == "jsonld/"+extract.RelationVersion || r.Method == extract.ProfileBindingVersion || r.Method == extract.ArticleBindingVersion
}

func summaryKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func summaryField(candidateID, field string, relations []domain.Relation) fieldSummary {
	current := map[string]string{}
	currentTemporal := map[string]string{}
	currentIDs := map[string][]string{}
	otherIDs := map[string][]string{}
	unknown := map[string][]domain.Relation{}
	for _, r := range relations {
		if r.CandidateID != candidateID || !summaryRelation(r) {
			continue
		}
		value := relationFieldValue(field, r)
		if value == "" {
			continue
		}
		key := summaryKey(value)
		if r.Temporal == "present_assertion" || r.Temporal == "headline" {
			if _, ok := current[key]; !ok {
				current[key] = value
			}
			currentIDs[key] = append(currentIDs[key], r.ID)
			if currentTemporal[key] != "present_assertion" {
				currentTemporal[key] = r.Temporal
			}
			continue
		}
		if r.Temporal == "unknown" {
			unknown[key] = append(unknown[key], r)
		}
		if r.Temporal == "historical" {
			otherIDs[key] = append(otherIDs[key], r.ID)
		}
	}
	if len(current) == 0 {
		for key, relations := range unknown {
			current[key] = relationFieldValue(field, relations[0])
			currentTemporal[key] = "unknown"
		}
	}
	for key, relations := range unknown {
		if _, ok := current[key]; ok {
			for _, r := range relations {
				currentIDs[key] = append(currentIDs[key], r.ID)
			}
		}
	}
	if len(current) == 0 {
		return fieldSummary{}
	}
	keys := make([]string, 0, len(current))
	for key := range current {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 1 {
		out := fieldSummary{}
		for _, key := range keys {
			out.alternatives = append(out.alternatives, current[key])
			out.ids = append(out.ids, currentIDs[key]...)
		}
		if field == "role" {
			conflict := false
			for _, key := range keys {
				if len(otherIDs[key]) > 0 {
					conflict = true
				}
			}
			if !conflict {
				out.value = strings.Join(out.alternatives, "; ")
				out.temporal = "multiple_claims"
			}
		}
		sort.Strings(out.ids)
		return out
	}
	key := keys[0]
	ids := append([]string(nil), currentIDs[key]...)
	if len(otherIDs[key]) > 0 {
		ids = append(ids, otherIDs[key]...)
		sort.Strings(ids)
		return fieldSummary{temporal: "conflicting", ids: ids}
	}
	sort.Strings(ids)
	return fieldSummary{value: current[key], temporal: currentTemporal[key], ids: ids}
}

func projectSummary(p *domain.Candidate, relations []domain.Relation) {
	p.RoleClaims = nil
	p.CompanyClaims = nil
	p.EducationClaims = nil
	for _, r := range relations {
		if r.CandidateID != p.ID || !summaryRelation(r) {
			continue
		}
		if role := relationFieldValue("role", r); role != "" {
			p.RoleClaims = appendClaim(p.RoleClaims, role, r)
		}
		if company := relationFieldValue("company", r); company != "" {
			p.CompanyClaims = appendClaim(p.CompanyClaims, company, r)
		}
		if r.Kind == "education" && r.Organization != "" {
			p.EducationClaims = appendClaim(p.EducationClaims, r.Organization, r)
		}
	}
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
	p.CompanyStatus = companyStatus(*p, company, relations)
	notes := map[string]bool{}
	if p.CompanyInferred || p.RoleInferred {
		notes["Extraction: summary fields are unverified source claims; inspect each field's period and relationship references"] = true
	}
	if company.temporal == "headline" || role.temporal == "headline" {
		notes["Extraction: summary includes an undated profile headline; verify it against the current source"] = true
	}
	if company.temporal == "present_assertion" || role.temporal == "present_assertion" {
		notes["Extraction: present tense is wording from the source, not verification of current employment; the source may be outdated"] = true
	}
	if len(company.alternatives) > 1 || len(role.alternatives) > 1 && role.value == "" {
		notes["Extraction: competing current or headline values remain ambiguous; only the affected summary field is withheld"] = true
	}
	if role.temporal == "multiple_claims" {
		notes["Extraction: several role claims are displayed separately; this does not establish concurrent employment"] = true
	}
	if company.temporal == "unknown" || role.temporal == "unknown" {
		notes["Extraction: source provides a relationship without a known period"] = true
	}
	if company.temporal == "conflicting" || role.temporal == "conflicting" {
		notes["Extraction: the value is supported but source claims disagree on its period"] = true
	}
	if p.Company == "" && p.Role == "" && len(company.alternatives) == 0 && len(role.alternatives) == 0 && company.temporal != "conflicting" && role.temporal != "conflicting" {
		notes["Extraction: no unambiguous current or headline relationship; inspect history and proposals"] = true
	}
	for note := range notes {
		p.Missing = append(p.Missing, note)
	}
	sort.Strings(p.Missing)
}

func companyStatus(p domain.Candidate, summary fieldSummary, relations []domain.Relation) string {
	if p.Company != "" {
		return "available"
	}
	if len(summary.alternatives) > 1 {
		return "ambiguous"
	}
	if summary.temporal == "conflicting" {
		return "period_conflict"
	}
	for _, r := range relations {
		if r.CandidateID == p.ID && r.Kind == "employment" && r.Organization != "" && strings.HasPrefix(r.Method, "ollama/") && domain.RelationDecision(r) == "PENDING" {
			return "pending_review"
		}
	}
	if len(p.CompanyClaims) > 0 {
		for _, claim := range p.CompanyClaims {
			if claim.Temporal != "historical" {
				return "non_current_claims"
			}
		}
		return "historical_only"
	}
	return "no_employment_claim"
}

func appendClaim(claims []domain.SummaryClaim, value string, r domain.Relation) []domain.SummaryClaim {
	for i := range claims {
		if summaryKey(claims[i].Value) == summaryKey(value) && claims[i].Kind == r.Kind && claims[i].Temporal == r.Temporal {
			claims[i].RelationIDs = append(claims[i].RelationIDs, r.ID)
			return claims
		}
	}
	return append(claims, domain.SummaryClaim{Value: value, Kind: r.Kind, Temporal: r.Temporal, RelationIDs: []string{r.ID}})
}

// Reproject changes only derived summaries, never identity or observed fields.
func Reproject(c *domain.Case) {
	for i := range c.Candidates {
		p := &c.Candidates[i]
		if p.CompanyInferred {
			p.Company = ""
		}
		if p.RoleInferred {
			p.Role = ""
		}
		p.CompanyInferred = false
		p.RoleInferred = false
		p.CompanyTemporal = ""
		p.RoleTemporal = ""
		p.CompanyRelationIDs = nil
		p.RoleRelationIDs = nil
		notes := []string{}
		for _, n := range p.Missing {
			if !strings.HasPrefix(n, "Extraction:") {
				notes = append(notes, n)
			}
		}
		p.Missing = notes
		projectSummary(p, c.Relations)
	}
}
