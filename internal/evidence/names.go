package evidence

import (
	"fmt"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/personname"
)

func normalizeNames(c *domain.Case, aliases map[string]string, byID map[string]domain.Evidence) error {
	seen := map[string]bool{}
	for i := range c.NameClaims {
		n := &c.NameClaims[i]
		if id, ok := aliases[n.EvidenceID]; ok {
			n.EvidenceID = id
		}
		e, ok := byID[n.EvidenceID]
		if !ok || e.Status != "OBSERVED" || e.CandidateID != n.CandidateID || !personname.TextType(e.Type) || n.ID == "" || seen[n.ID] || n.Status != "UNVERIFIED" || n.Method != personname.Version || !strings.Contains(e.Value, n.Quote) {
			return fmt.Errorf("invalid public name provenance %s", n.ID)
		}
		seen[n.ID] = true
		bound := false
		for _, p := range c.Candidates {
			if p.ID != n.CandidateID {
				continue
			}
			subject := c.Target.Name
			if p.DisplayName != "" {
				subject = p.DisplayName
			} else if c.Search != nil && c.Search.Name != "" {
				subject = c.Search.Name
			}
			bound = personname.Equal(subject, n.Alias) || personname.Equal(subject, n.Name)
		}
		matched := false
		for _, found := range personname.Find(n.Alias, e.Value) {
			if found.Alias == n.Alias && found.Name == personname.WithoutPostnominals(n.Name) && strings.Contains(n.Quote, n.Name) && found.Kind == n.Kind && found.Quote == n.Quote {
				matched = true
			}
		}
		for _, other := range c.Evidence {
			if other.CandidateID == n.CandidateID && other.Status == "OBSERVED" && (other.Type == "search_title" || other.Type == "page_title" || other.Type == "profile_description") && personname.ExcludedContext(other.Value) {
				bound = false
			}
		}
		if !bound || !matched {
			return fmt.Errorf("unbound public name claim %s", n.ID)
		}
	}
	return nil
}
