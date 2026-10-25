package identity

import (
	"sort"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

type NameAmbiguity struct {
	Alias string
	Names []string
}

// Different source names may be namesakes or spelling variants; neither is resolved here.
func NameAmbiguities(c domain.Case) []NameAmbiguity {
	rejected := map[string]bool{}
	for _, p := range c.Candidates {
		rejected[p.ID] = p.Decision == "REJECT"
	}
	names := map[string]map[string]string{}
	aliases := map[string]string{}
	for _, n := range c.NameClaims {
		if rejected[n.CandidateID] {
			continue
		}
		alias := strings.ToLower(strings.Join(strings.Fields(n.Alias), " "))
		key := strings.ToLower(strings.Join(strings.Fields(n.Name), " "))
		if names[alias] == nil {
			names[alias] = map[string]string{}
		}
		names[alias][key], aliases[alias] = n.Name, n.Alias
	}
	out := []NameAmbiguity{}
	for alias, values := range names {
		if len(values) < 2 {
			continue
		}
		item := NameAmbiguity{Alias: aliases[alias]}
		for _, name := range values {
			item.Names = append(item.Names, name)
		}
		sort.Strings(item.Names)
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Alias < out[j].Alias })
	return out
}
