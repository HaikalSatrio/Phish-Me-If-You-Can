package collector

import "strings"

const maxSearchPhrases = 4

func sanitizeTerm(s string) string {
	return strings.TrimSpace(strings.NewReplacer(`"`, " ", "\r", " ", "\n", " ", "\\", " ").Replace(s))
}

func quoteTerm(s string) string { return `"` + s + `"` }

// searchPhrases keeps the primary identity clue first. Optional hints only
// narrow later variants so a hint can never suppress the discovery query.
func searchPhrases(q Query) []string {
	name := sanitizeTerm(q.Name)
	username := strings.TrimPrefix(sanitizeTerm(q.Username), "@")
	primary := name
	if username != "" {
		primary = username
	}
	if primary == "" {
		return nil
	}
	base := quoteTerm(primary)
	if q.Purpose == "biography" {
		return []string{base + " biography real name"}
	}
	company := sanitizeTerm(q.Company)
	location := sanitizeTerm(q.Location)
	phrases := []string{base}
	if username == "" && len(strings.Fields(name)) >= 3 && (q.Platform == "" || q.Platform == "linkedin") {
		suffix := ""
		if q.Platform == "" {
			suffix = " (site:linkedin.com)"
			phrases = append(phrases, base+suffix)
		}
		phrases = append(phrases, quoteTerm(strings.Join(strings.Fields(name)[:2], " "))+suffix)
	}
	if name != "" && username != "" && !strings.EqualFold(name, username) {
		phrases = append(phrases, base+" "+quoteTerm(name))
	}
	if company != "" {
		phrases = append(phrases, base+" "+quoteTerm(company))
	}
	if location != "" {
		phrases = append(phrases, base+" "+quoteTerm(location))
	}
	if company != "" && location != "" {
		phrases = append(phrases, base+" "+quoteTerm(company)+" "+quoteTerm(location))
	}
	seen := map[string]bool{}
	out := make([]string, 0, maxSearchPhrases)
	for _, phrase := range phrases {
		if seen[phrase] {
			continue
		}
		seen[phrase] = true
		out = append(out, phrase)
		if len(out) == maxSearchPhrases {
			break
		}
	}
	return out
}
