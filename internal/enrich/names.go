package enrich

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/extract"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/personname"
)

func publicNames(subject string, p domain.Candidate, docs []extract.Document) []domain.NameClaim {
	for _, doc := range docs {
		if (doc.Type == "search_title" || doc.Type == "page_title" || doc.Type == "profile_description") && personname.ExcludedContext(doc.Text) {
			return nil
		}
	}
	out := []domain.NameClaim{}
	for _, doc := range docs {
		if !personname.TextType(doc.Type) {
			continue
		}
		for _, claim := range personname.Find(subject, doc.Text) {
			claim.CandidateID, claim.EvidenceID = p.ID, doc.ID
			key := strings.Join([]string{p.ID, doc.ID, claim.Alias, claim.Name, claim.Quote}, "\x00")
			hash := sha256.Sum256([]byte(key))
			claim.ID = fmt.Sprintf("%s-name-%x", p.ID, hash[:8])
			out = append(out, claim)
		}
	}
	return out
}
