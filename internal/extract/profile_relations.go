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
	for _, q := range []string{"digital", "strategic", "communication", "communications", "startup", "public"} {
		qualifiers[q] = true
	}
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

func headlineRole(value string) bool {
	if profileRole(value) {
		return true
	}
	parts := roleConjunction.Split(value, -1)
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	for _, part := range parts {
		if !profileRole(part) {
			return false
		}
	}
	return true
}

func functionTarget(value string) bool {
	if organizationMarker.MatchString(value) {
		return false
	}
	for _, term := range []string{
		"program", "programs", "division", "department", "faculty", "studies",
		"engineering", "cybersecurity", "security", "information", "technology",
		"computer", "software", "product", "business", "finance", "marketing",
		"operations", "research", "development", "design", "data", "analytics",
		"audit", "compliance", "risk", "governance", "strategy", "communications",
		"public relations", "human resources", "legal", "medicine", "health",
		"education", "academic", "student", "affairs", "services", "laboratory",
		"lab", "sales", "growth", "talent", "people", "infrastructure",
		"platform", "cloud", "network", "systems", "support", "quality",
		"safety", "procurement", "manufacturing", "production", "logistics",
		"treasury", "accounting", "tax", "policy", "partnerships", "innovation",
		"digital", "architecture", "it", "hr",
	} {
		if containsWords(value, term) {
			return true
		}
	}
	return false
}

var organizationMarker = regexp.MustCompile(`(?i)\b(?:ltd|limited|inc|incorporated|llc|plc|corp|corporation|company|gmbh|university|institute|association|foundation|bank)\.?$|^(?:PT\.?|CV\.?|universitas|institut|yayasan)\s|^(?:university|institute|association)\s+of\s`)

func profileRelationFields(body string) (kind, org, role string) {
	if parts := strings.SplitN(body, ",", 2); len(parts) == 2 && headlineRole(strings.TrimSpace(parts[0])) {
		left, right := strings.TrimSpace(parts[0]), literalField(strings.TrimSpace(parts[1]))
		if validLiteral(right, 14) && organizationTargetStartsProper(right) && !strings.Contains(right, " & ") {
			return "employment", right, left
		}
		return "occupation", "", left
	}
	body = literalField(strings.TrimSpace(body))
	if m := possessiveRole.FindStringSubmatch(body); len(m) > 0 && validLiteral(m[1], 14) && organizationTargetStartsProper(m[1]) && headlineRole(m[2]) {
		return "employment", m[1], m[2]
	}
	for _, suffix := range []string{" specializing in ", " specialising in "} {
		if i := strings.Index(strings.ToLower(body), suffix); i >= 0 {
			body = body[:i]
		}
	}
	for _, separator := range []string{" at ", " for ", " in ", " di ", " of "} {
		index := strings.Index(strings.ToLower(body), separator)
		if index < 0 {
			continue
		}
		left, right := strings.TrimSpace(body[:index]), strings.TrimSpace(body[index+len(separator):])
		right = strings.TrimSpace(regexp.MustCompile(`(?i)^the\s+`).ReplaceAllString(right, ""))
		if separator == " in " && (!organizationMarker.MatchString(right) || !organizationTargetStartsProper(right)) {
			continue
		}
		if !validRoleValue(left) || !validLiteral(right, 14) {
			continue
		}
		// "Head of Cybersecurity Program" names a function, not an employer.
		if separator == " of " && functionTarget(right) && !organizationalLeadership.MatchString(left) {
			if headlineRole(left) {
				return "occupation", "", body
			}
			continue
		}
		if separator == " of " && !organizationTargetStartsProper(right) {
			continue
		}
		if !headlineRole(left) {
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
	if headlineRole(body) {
		return "occupation", "", body
	}
	return "", "", ""
}

// Undated profile headers become headline claims, not verified current employment.
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
		if state == "unknown" {
			state = "headline"
		}
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
