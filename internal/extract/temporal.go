package extract

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

const yearAtom = `(?:18|19|20|21)\d{2}`
const monthAtom = `(?:jan(?:uary|uari)?|feb(?:ruary|ruari)?|mar(?:ch|et)?|apr(?:il)?|may|mei|jun(?:e|i)?|jul(?:y|i)?|aug(?:ust)?|agu(?:stus)?|sep(?:tember)?|oct(?:ober)?|okt(?:ober)?|nov(?:ember)?|dec(?:ember)?|des(?:ember)?)\.?`
const dayAtom = `(?:0?[1-9]|[12]\d|3[01])`
const dateAtom = `(?:` + yearAtom + `-\d{2}-\d{2}|` + dayAtom + `\s+` + monthAtom + `\s+` + yearAtom + `|` + monthAtom + `\s+` + dayAtom + `,?\s+` + yearAtom + `|` + monthAtom + `\s+` + yearAtom + `|` + yearAtom + `)`

var periodRange = regexp.MustCompile(`(?i)\b(` + dateAtom + `)\s*(?:-|\x{2013}|\x{2014}|to|until|through|sampai|hingga|s/d)\s*(` + dateAtom + `)\b`)
var betweenRange = regexp.MustCompile(`(?i)\bbetween\s+(` + dateAtom + `)\s+and\s+(` + dateAtom + `)\b`)
var leadingPeriod = regexp.MustCompile(`(?i)^(?:between|from)\s+` + dateAtom + `\s+(?:and|to|until)\s+` + dateAtom + `,\s*`)
var periodStart = regexp.MustCompile(`(?i)\b(?:since|sejak|from|dari|mulai)\s+(?:tahun\s+)?(` + dateAtom + `)\b`)
var periodEnd = regexp.MustCompile(`(?i)\b(?:until|hingga|sampai)\s+(` + dateAtom + `)\b`)
var dateTail = regexp.MustCompile(`(?i)\s+(?:from|since|during|between|in|pada|sejak|dari|selama|tahun|mulai)\s+(?:tahun\s+)?` + dateAtom + `|\s*\(?\b` + dateAtom + `\s*[-\x{2013}\x{2014}]`)
var yearPattern = regexp.MustCompile(`\b` + yearAtom + `\b`)
var pastTense = regexp.MustCompile(`(?i)\b(?:worked|served|former|formerly|previously|mantan|pernah)\b|\bwas\s+(?:an?\s+)?(?:[A-Z][a-z]+\s+)?(?:engineer|president|director|manager|ceo|cfo|lecturer|researcher)\b`)

func dateYear(value string) int { n, _ := strconv.Atoi(yearPattern.FindString(value)); return n }

func temporal(quote, verb string, asOf time.Time) (int, int, string) {
	start, end := 0, 0
	if m := periodRange.FindStringSubmatch(quote); len(m) > 0 {
		start, end = dateYear(m[1]), dateYear(m[2])
	} else if m := betweenRange.FindStringSubmatch(quote); len(m) > 0 {
		start, end = dateYear(m[1]), dateYear(m[2])
	} else {
		if m := periodStart.FindStringSubmatch(quote); len(m) > 0 {
			start = dateYear(m[1])
		}
		if m := periodEnd.FindStringSubmatch(quote); len(m) > 0 {
			end = dateYear(m[1])
		}
	}
	if start > asOf.Year() {
		return start, end, "future"
	}
	if end != 0 && end <= asOf.Year() || pastTense.MatchString(quote) || pastTense.MatchString(verb) || strings.EqualFold(strings.TrimSpace(verb), "was") {
		return start, end, "historical"
	}
	if strings.TrimSpace(verb) != "" {
		return start, end, "present_assertion"
	}
	return start, end, "unknown"
}
