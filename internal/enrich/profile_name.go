package enrich

import (
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/profile"
)

func recoverProfileName(p *domain.Candidate, evidence *[]domain.Evidence) {
	if p.DisplayName != "" {
		return
	}
	name := ""
	var title domain.Evidence
	for _, e := range *evidence {
		if e.CandidateID != p.ID || e.Status != "OBSERVED" || (e.Type != "page_title" && e.Type != "search_title") {
			continue
		}
		observed, ok := profile.NameFromTitle(p.ProfileURL, e.Value)
		if !ok {
			continue
		}
		if name != "" && !strings.EqualFold(name, observed) {
			return
		}
		name = observed
		if title.ID == "" || e.Type == "page_title" {
			title = e
		}
	}
	if name == "" {
		return
	}
	for _, e := range *evidence {
		if e.ID == p.ID+"-name" {
			return
		}
	}
	p.DisplayName = name
	*evidence = append(*evidence, domain.Evidence{ID: p.ID + "-name", CandidateID: p.ID, Type: "profile_display_name", Value: name, Status: "OBSERVED", Confidence: title.Confidence, Discoverability: title.Discoverability, Sources: append([]domain.Source(nil), title.Sources...), SupportingIDs: []string{title.ID}})
}
