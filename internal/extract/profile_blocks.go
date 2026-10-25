package extract

import (
	"regexp"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

var profileLabel = regexp.MustCompile(`(?i)^(company|employer|works for|perusahaan|tempat kerja|role|job title|position|jabatan)\s*:\s*(.+)$`)

func profileSubjectLine(line, subject string) bool {
	name := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(subject) + `(?:$|,)`)
	if subject == "" || !name.MatchString(line) {
		return false
	}
	end := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(subject)).FindStringIndex(line)[1]
	rest := strings.Trim(line[end:], " ,")
	for n := 0; n < 24 && rest != ""; n++ {
		loc := credentialPrefix.FindStringIndex(rest)
		if loc == nil {
			return false
		}
		rest = strings.TrimSpace(rest[loc[1]:])
	}
	return rest == ""
}

// Adjacent profile fields retain one contiguous quote and stop at unknown text.
func profileBlockRelations(subject string, doc Document, asOf time.Time) []domain.Relation {
	if doc.Type != "page_text" || subject == "" {
		return nil
	}
	out := []domain.Relation{}
	lines := strings.Split(doc.Text, "\n")
	offset := 0
	for i, line := range lines {
		start := offset
		offset += len(line) + 1
		if !profileSubjectLine(strings.TrimSpace(line), subject) {
			continue
		}
		end := min(i+7, len(lines))
		block := strings.Join(lines[i:end], "\n")
		// Reuse headline parsing for a name followed by explicit role clauses.
		header := Document{ID: doc.ID, Type: "profile_description", Text: block}
		out = append(out, profileRelations(subject, header, asOf)...)
		org, role := "", ""
		quoteEnd := start + len(line)
		valid := true
		for _, next := range lines[i+1 : end] {
			m := profileLabel.FindStringSubmatch(strings.TrimSpace(next))
			if m == nil {
				break
			}
			value := strings.TrimSpace(m[2])
			switch strings.ToLower(value) {
			case "n/a", "none", "unknown", "not specified", "tidak diketahui", "-":
				valid = false
			}
			switch strings.ToLower(m[1]) {
			case "company", "employer", "works for", "perusahaan", "tempat kerja":
				if org != "" || !validLiteral(value, 14) || !profileEmployerTarget(value) || headlineRole(value) {
					valid = false
				}
				org = value
			default:
				if role != "" || !headlineRole(value) {
					valid = false
				}
				role = value
			}
			quoteEnd += 1 + len(next)
		}
		if !valid || org == "" && role == "" {
			continue
		}
		quote := strings.TrimSpace(doc.Text[start:quoteEnd])
		if negatedRelation.MatchString(quote) {
			continue
		}
		kind := "occupation"
		if org != "" {
			kind = "employment"
			for _, student := range []string{"student", "undergraduate", "postgraduate", "mahasiswa", "mahasiswi", "pelajar"} {
				if containsWords(role, student) {
					kind = "education"
				}
			}
		}
		s, e, state := temporal(quote, "", asOf)
		if state == "unknown" {
			state = "headline"
		}
		r := domain.Relation{Subject: subject, Kind: kind, Organization: org, Role: role, Temporal: state, StartYear: s, EndYear: e, EvidenceID: doc.ID, Quote: quote, Method: RelationVersion, Status: "UNVERIFIED"}
		if ValidateRelation(r, subject, doc) == nil {
			out = append(out, r)
		}
	}
	return out
}
