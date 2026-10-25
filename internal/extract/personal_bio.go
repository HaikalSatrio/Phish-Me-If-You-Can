package extract

import (
	"regexp"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

var firstPersonEmployment = regexp.MustCompile(`(?i)(?:^|[.!?]\s+)(?:I am|I'm|I\x{2019}m)\s+(?:currently\s+)?(?:(?:the|a|an)\s+)?([^.;\n]{2,80}?)\s+(at|of|for)\s+([^.;\n]{2,100})`)
var narrativeContext = regexp.MustCompile(`(?i)\b(?:interview|said|says|quoted|quote|example|fiction|fictional|novel|character|met|visited)\b`)

// First-person employment requires an exact personal-page title and opening name heading.
func personalBiographyRelations(subject string, docs []Document, asOf time.Time) []domain.Relation {
	if len(strings.Fields(subject)) < 2 {
		return nil
	}
	personalTitle := false
	for _, doc := range docs {
		if doc.Type == "page_title" && strings.EqualFold(compact(doc.Text), compact(subject)) {
			personalTitle = true
		}
	}
	if !personalTitle {
		return nil
	}
	out := []domain.Relation{}
	for _, doc := range docs {
		if doc.Type != "page_text" {
			continue
		}
		text := strings.TrimSpace(doc.Text)
		line, body, ok := strings.Cut(text, "\n")
		if !ok || !strings.EqualFold(compact(line), compact(subject)) || !regexp.MustCompile(`(?i)^(?:I'm|I am|I\x{2019}m)\s`).MatchString(strings.TrimSpace(body)) {
			continue
		}
		if len([]rune(body)) > 900 {
			body = string([]rune(body)[:900])
		}
		for _, loc := range firstPersonEmployment.FindAllStringSubmatchIndex(body, 4) {
			quote := strings.TrimSpace(text[:len(line)+1+loc[1]])
			if len([]rune(quote)) > 1000 || negatedRelation.MatchString(quote) || narrativeContext.MatchString(quote) {
				continue
			}
			role := strings.TrimSpace(body[loc[2]:loc[3]])
			separator := body[loc[4]:loc[5]]
			org := organizationField(body[loc[6]:loc[7]])
			kind, parsedOrg, parsedRole := profileRelationFields(role + " " + separator + " " + org)
			if kind != "employment" || parsedOrg != org || parsedRole != role {
				continue
			}
			s, e, state := temporal(body[loc[0]:loc[1]], "is", asOf)
			r := domain.Relation{Subject: subject, Kind: kind, Organization: org, Role: role, Temporal: state, StartYear: s, EndYear: e, EvidenceID: doc.ID, Quote: quote, Method: RelationVersion, Status: "UNVERIFIED"}
			if ValidateRelation(r, subject, doc) == nil {
				out = append(out, r)
			}
		}
	}
	return out
}
