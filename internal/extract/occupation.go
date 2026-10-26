package extract

import (
	"regexp"
	"strings"
)

var occupationIntro = regexp.MustCompile(`(?i)^(?:known as|dikenal sebagai)\s+(?:an?\s+|seorang\s+)?`)
var occupationDetail = regexp.MustCompile(`(?i)\s+(?:with a|with an|dengan gaya|asal|aus|who|whose|best known|renowned|recognized|yang)\s+`)
var occupationAge = regexp.MustCompile(`(?i)^\d{1,3}-year(?:-|\s+)old\s+`)
var narrativeOccupation = regexp.MustCompile(`(?i)^(?:known|famous|famed|fan|friend|relative|supporter|admirer|penggemar|teman)\b`)
var geographicQualifier = regexp.MustCompile(`(?i)^(?:American|Indonesian|Swedish|Canadian|English|British|Irish|Australian|New Zealand|South Korean|French|German|Dutch|Japanese|Chinese|Indian|Brazilian|Moroccan|Nigerian|Filipino|Mexican|online)\s+`)

func occupationField(body string) string {
	value := occupationIntro.ReplaceAllString(literalField(body), "")
	value = occupationAge.ReplaceAllString(value, "")
	if narrativeOccupation.MatchString(value) {
		return ""
	}
	if at := occupationDetail.FindStringIndex(value); at != nil {
		value = value[:at[0]]
	}
	return strings.TrimSpace(value)
}

func sportsTeamField(role, org string) string {
	if !regexp.MustCompile(`(?i)\b(?:player|footballer)\b`).MatchString(role) {
		return org
	}
	org = regexp.MustCompile(`(?i)^the\s+`).ReplaceAllString(org, "")
	if at := strings.Index(strings.ToLower(org), " of the "); at >= 0 {
		if regexp.MustCompile(`(?i)\b(?:association|league)\b`).MatchString(org[at+8:]) {
			org = org[:at]
		}
	}
	return org
}

func occupationParts(value string) []string {
	parts := roleConjunction.Split(value, -1)
	if len(parts) < 2 || len(parts) > 3 {
		return nil
	}
	for _, part := range parts {
		// Every conjunct must independently describe a profession.
		if !profileRole(geographicQualifier.ReplaceAllString(part, "")) || !literalRole(part) {
			return nil
		}
	}
	return parts
}
