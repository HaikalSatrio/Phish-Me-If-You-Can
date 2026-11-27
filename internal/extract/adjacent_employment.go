package extract

import (
	"regexp"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

var ambiguousAdjacentClause = regexp.MustCompile(`(?i)\b(?:with|alongside|together|before|after|where|while|who|which|but|dengan|bersama|sebelum|setelah|tetapi|namun|yang)\b`)

func adjacentEmploymentRelations(subject string, doc Document, asOf time.Time) []domain.Relation {
	if !RelationTextType(doc.Type) || doc.Type == "person_jsonld" || strings.TrimSpace(subject) == "" {
		return nil
	}
	parts := strings.Fields(subject)
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	pattern := `(?im)^[ \t]*` + strings.Join(parts, `[ \t]+`) + `[ \t]+(?:joined|bergabung di)[ \t]+([^.!?;\n]{1,120})\.[ \t]+(?:she works there as|he works there as|they work there as|ia bekerja di sana sebagai|dia bekerja di sana sebagai)[ \t]+([^.!?;\n]{1,100})(?:[.!]|$)`
	var out []domain.Relation
	for _, m := range regexp.MustCompile(pattern).FindAllStringSubmatch(doc.Text, 8) {
		if negatedRelation.MatchString(m[0]) || ambiguousAdjacentClause.MatchString(m[0]) {
			continue
		}
		organization, role := literalField(m[1]), literalField(m[2])
		if !validLiteral(organization, 12) || !literalRole(role) || !validLiteral(role, 10) || relationSeparator.MatchString(role) {
			continue
		}
		start, end, state := temporal(m[2], "works", asOf)
		r := domain.Relation{Subject: subject, Kind: "employment", Role: role, Organization: organization, Temporal: state, StartYear: start, EndYear: end, EvidenceID: doc.ID, Quote: m[0], Method: RelationVersion, Status: "UNVERIFIED"}
		if ValidateRelation(r, subject, doc) == nil {
			out = append(out, r)
		}
	}
	return out
}
