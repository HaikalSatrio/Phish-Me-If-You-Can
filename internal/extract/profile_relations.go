package extract

import (
	"regexp"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

var roleConjunction = regexp.MustCompile(`(?i)\s+(?:and|dan|&)\s+`)
var profileSeparator = regexp.MustCompile(`\s*[|\x{00b7}\x{2022}\x{2013}\x{2014}]\s*|\s+-\s+|\.\s+`)
var credentialPrefix = regexp.MustCompile(`^(?:[A-Z](?:\.[A-Za-z]{1,6})+\.?|[A-Z][A-Z0-9/-]{1,15}(?:master)?|[a-z]{1,2}[A-Z]{2,12})(?:[,\.\s]+|$)`)

func validRoleValue(role string) bool {
	if validLiteral(role, 16) {
		return true
	}
	parts := roleConjunction.Split(role, -1)
	if len(parts) < 2 || len(parts) > 3 || len([]rune(role)) > 160 {
		return false
	}
	for _, part := range parts {
		if !validLiteral(part, 8) || !profileRole(part) {
			return false
		}
	}
	return true
}

// A headline role starts with a role word or a bounded professional qualifier.
func profileRole(role string) bool {
	qualifiers := map[string]bool{"general": true, "senior": true, "junior": true, "software": true, "security": true, "information": true, "cyber": true, "cybersecurity": true, "computer": true, "science": true, "awareness": true, "penetration": true, "financial": true, "technical": true, "business": true, "product": true, "full": true, "stack": true}
	if !validLiteral(role, 16) {
		return false
	}
	for _, word := range strings.Fields(strings.ToLower(role)) {
		if roleWords[word] {
			return true
		}
		if !qualifiers[word] {
			return false
		}
	}
	return false
}

func profileRelationFields(body string) (kind, org, role string) {
	body = literalField(strings.TrimSpace(body))
	for _, suffix := range []string{" specializing in ", " specialising in "} {
		if i := strings.Index(strings.ToLower(body), suffix); i >= 0 {
			body = body[:i]
		}
	}
	for _, separator := range []string{" at ", " in ", " di ", " of "} {
		index := strings.Index(strings.ToLower(body), separator)
		if index < 0 {
			continue
		}
		left, right := strings.TrimSpace(body[:index]), strings.TrimSpace(body[index+len(separator):])
		if !validRoleValue(left) || !validLiteral(right, 14) {
			continue
		}
		// "Head of Cybersecurity Program" names a function, not an employer.
		if separator == " of " && !organizationTargetStartsProper(right) {
			continue
		}
		if !profileRole(left) && len(roleConjunction.Split(left, -1)) == 1 {
			continue
		}
		kind = "employment"
		for _, word := range []string{"student", "undergraduate", "postgraduate", "mahasiswa", "mahasiswi"} {
			if containsWords(left, word) {
				kind = "education"
			}
		}
		if separator == " of " {
			for _, prefix := range []string{"himti ", "himpunan ", "association ", "club ", "community "} {
				if strings.HasPrefix(strings.ToLower(right), prefix) {
					kind = "membership"
				}
			}
		}
		return kind, right, left
	}
	if profileRole(body) || len(roleConjunction.Split(body, -1)) > 1 && validRoleValue(body) {
		return "occupation", "", body
	}
	return "", "", ""
}

// Profile headers retain unknown time instead of guessing a current position.
// Parsing stops at narrative or another name, so later people's roles are excluded.
func profileRelations(subject string, doc Document, asOf time.Time) []domain.Relation {
	switch doc.Type {
	case "search_title", "page_title", "search_snippet", "profile_description", "page_description":
	default:
		return nil
	}
	text := strings.TrimSpace(doc.Text)
	if strings.HasPrefix(text, "... ") {
		text = strings.TrimSpace(text[4:])
	}
	if len(text) > 1600 || subject == "" || roleTitleContextRejected(text) || negatedRelation.MatchString(text) {
		return nil
	}
	namePattern := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(subject) + `(?:[\s,.:|\x{00b7}\x{2022}\x{2013}\x{2014}]|$)`)
	if !namePattern.MatchString(text) {
		return nil
	}
	// Find the actual matched name bytes instead of indexing a case-folded string.
	nameEnd := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(subject)).FindStringIndex(text)[1]
	rest := text[nameEnd:]
	for n := 0; n < 24; n++ {
		rest = strings.TrimLeft(rest, " ,.:|\u00b7\u2022\u2013\u2014-")
		first := strings.Fields(rest)
		if len(first) == 0 {
			return nil
		}
		if roleWords[strings.ToLower(first[0])] {
			break
		}
		loc := credentialPrefix.FindStringIndex(rest)
		if loc == nil {
			break
		}
		rest = rest[loc[1]:]
	}
	if rest == "" {
		return nil
	}
	start := len(text) - len(rest)
	out := []domain.Relation{}
	for _, loc := range segmentRanges(rest) {
		part := strings.TrimSpace(rest[loc[0]:loc[1]])
		if part == "" {
			continue
		}
		kind, org, role := profileRelationFields(part)
		if kind == "" {
			break
		}
		quote := strings.TrimSpace(text[:start+loc[1]])
		if len([]rune(quote)) > 1000 {
			break
		}
		s, e, state := temporal(quote, "", asOf)
		r := domain.Relation{Subject: subject, Kind: kind, Organization: org, Role: role, StartYear: s, EndYear: e, Temporal: state, EvidenceID: doc.ID, Quote: quote, Method: RelationVersion, Status: "UNVERIFIED"}
		if ValidateRelation(r, subject, doc) == nil {
			out = append(out, r)
		}
	}
	return out
}

func segmentRanges(text string) [][2]int {
	out := [][2]int{}
	start := 0
	for _, loc := range profileSeparator.FindAllStringIndex(text, -1) {
		out = append(out, [2]int{start, loc[0]})
		start = loc[1]
	}
	return append(out, [2]int{start, len(text)})
}
