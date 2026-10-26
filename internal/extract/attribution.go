package extract

import (
	"regexp"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

var possessiveRole = regexp.MustCompile(`^(.{2,100}?)['\x{2019}]s\s+(.+)$`)
var organizationalLeadership = regexp.MustCompile(`(?i)\b(?:ceo|cfo|cto|coo|founder|co-founder|cofounder|owner|president|pendiri|pemilik)\b`)
var clauseContinuation = regexp.MustCompile(`(?i)\s+(?:and|dan)\s+(?:serves|sits|works|is|was|has|oleh|dikenal|menjabat)\b`)

func primaryClause(body string) string {
	if i := regexp.MustCompile(`(?i)\s+(?:(?:best\s+)?known|renowned|famous|recognized)\s+for\s+`).FindStringIndex(body); i != nil {
		body = body[:i[0]]
	}
	if i := regexp.MustCompile(`(?i)\s+(?:who|whose|yang)\s+`).FindStringIndex(body); i != nil {
		body = body[:i[0]]
	}
	if i := clauseContinuation.FindStringIndex(body); i != nil {
		body = body[:i[0]]
	}
	return body
}

func primaryRoleClause(body string) string {
	// Strip dates before descriptive commas so month-first dates remain intact.
	return literalField(primaryClause(body))
}

// Attributive titles keep the organization, role and full name in one source span.
func attributedTitleRelations(subject string, doc Document, asOf time.Time) []domain.Relation {
	if subject == "" || doc.Type == "person_jsonld" {
		return nil
	}
	pattern := regexp.MustCompile(`(?i)^([\p{L}\p{N}][\p{L}\p{N}.&' -]{1,100}?)\s+((?:co-founder|founder|ceo|cfo|cto|president)(?:\s+(?:and|&)\s+(?:co-founder|founder|ceo|cfo|cto|president)){0,2})\s+` + regexp.QuoteMeta(subject) + `(?:\s|[,.]|$)`)
	out := []domain.Relation{}
	for _, line := range strings.Split(doc.Text, "\n") {
		m := pattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(m) == 0 || !validLiteral(m[1], 12) || !organizationTargetStartsProper(m[1]) || negatedRelation.MatchString(m[0]) {
			continue
		}
		// Reject introductory prose that happens to precede a title and a name.
		if regexp.MustCompile(`(?i)\b(?:said|says|with|met|interviewed|according|former|formerly|ex|mantan)\b`).MatchString(m[1]) {
			continue
		}
		r := domain.Relation{Subject: subject, Kind: "employment", Organization: strings.TrimSpace(m[1]), Role: m[2], Temporal: "headline", EvidenceID: doc.ID, Quote: strings.TrimSpace(m[0]), Method: RelationVersion, Status: "UNVERIFIED"}
		if ValidateRelation(r, subject, doc) == nil {
			out = append(out, r)
		}
	}
	return out
}
