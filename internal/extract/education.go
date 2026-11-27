package extract

import (
	"regexp"
	"strings"
)

var studyStatement = regexp.MustCompile(`(?i)^(studies|is studying|kuliah|berkuliah)(?:\s+(.{1,80}?))?\s+(?:at|di)\s+(.+)$`)
var educationalInstitution = regexp.MustCompile(`(?i)\b(?:university|universitas|college|school|sekolah|polytechnic|politeknik|akademi|academy|institute|institut)\b`)
var studyDiscipline = regexp.MustCompile(`(?i)^(?:computer science|cyber\s?security|information technology|mathematics|physics|chemistry|biology|medicine|law|economics|psychology|engineering|teknik informatika|ilmu komputer|sistem informasi|kedokteran|hukum|ekonomi|psikologi)$`)

func studyFields(rest string) (organization, verb string) {
	m := studyStatement.FindStringSubmatch(rest)
	if len(m) == 0 {
		return "", ""
	}
	if course := compact(m[2]); course != "" && !studyDiscipline.MatchString(course) {
		return "", ""
	}
	organization = literalField(m[3])
	if !validLiteral(organization, 14) || !educationalInstitution.MatchString(organization) || strings.ContainsAny(organization, "?!") {
		return "", ""
	}
	return organization, m[1]
}
