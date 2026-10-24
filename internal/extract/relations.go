package extract

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

const RelationVersion = "relations-v4"

type Document struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text"`
}

func RelationTextType(typ string) bool {
	if typ == "person_jsonld" {
		return true
	}
	for _, t := range textFactTypes {
		if typ == t {
			return true
		}
	}
	return false
}

var relationVerb = regexp.MustCompile(`(?i)^(works at|worked at|bekerja di|pernah bekerja di|works as|worked as|serves as|served as|menjabat sebagai|pernah menjabat sebagai|is|was|adalah|merupakan)\s+(.+)$`)
var relationSeparator = regexp.MustCompile(`(?i)\s+(?:at|for|di|pada)\s+`)
var publicOffice = regexp.MustCompile(`(?i)^(?:(?:the|a|an|seorang|mantan|former|first|second|third|fourth|fifth|sixth|seventh|eighth|ninth|tenth|ke-?\d+|\d+(?:st|nd|rd|th))\s+)*(?:presiden(?:t)?|wakil presiden|vice president|governor|gubernur|mayor|wali kota|walikota|menteri|minister|prime minister|perdana menteri)\b`)
var negatedRelation = regexp.MustCompile(`(?i)\b(?:not|never|denied|denies|allegedly|rumored|rumoured|reportedly|might|could|would|bukan|tidak|dikabarkan|diduga|akan|jika|if)\b|\b(?:may|will)\s+(?:be|work|serve|join|become)\b`)
var aliasClause = regexp.MustCompile(`(?i)^,\s*(?:also known as|known as|dikenal sebagai)\s+[^,;.!?]{1,80},\s*`)

func compact(s string) string { return strings.Join(strings.Fields(s), " ") }

// Sentences retains table and paragraph boundaries and does not split PT names.
func Sentences(text string) []string {
	var out []string
	start := 0
	for i, r := range text {
		boundary := r == '\n' || r == ';' || r == '!' || r == '?'
		if r == '.' && (i+1 == len(text) || text[i+1] == ' ' || text[i+1] == '\n') {
			words := strings.Fields(text[start:i])
			last := ""
			if len(words) > 0 {
				last = strings.ToLower(words[len(words)-1])
			}
			boundary = last != "pt" && last != "dr" && last != "mr" && last != "mrs" && last != "prof"
		}
		if boundary {
			if v := strings.TrimSpace(text[start:i]); v != "" {
				out = append(out, v)
			}
			start = i + len(string(r))
		}
	}
	if v := strings.TrimSpace(text[start:]); v != "" {
		out = append(out, v)
	}
	return out
}

func literalField(s string) string {
	if loc := dateTail.FindStringIndex(s); loc != nil {
		s = s[:loc[0]]
	}
	if loc := strings.Index(s, ","); loc >= 0 {
		s = s[:loc]
	}
	s = strings.Trim(s, " .,:;()")
	return s
}

func validLiteral(s string, maxWords int) bool {
	if s == "" || len([]rune(s)) > 160 || len(strings.Fields(s)) > maxWords || strings.ContainsAny(s, "|\n:[]") {
		return false
	}
	lower := " " + strings.ToLower(s) + " "
	for _, bad := range []string{" who ", " yang ", " interviewed ", " said ", " mengatakan ", " and ", " dan ", " also ", " mulai menjabat ", " selesai menjabat ", " he ", " she "} {
		if strings.Contains(lower, bad) {
			return false
		}
	}
	return true
}

func literalRole(s string) bool {
	if !validRoleValue(s) {
		return false
	}
	if publicOffice.MatchString(s) {
		return true
	}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		if roleWords[w] {
			return true
		}
	}
	return false
}

func subjectPrefix(quote, subject string) (string, bool) {
	if loc := leadingPeriod.FindStringIndex(quote); loc != nil {
		quote = quote[loc[1]:]
	}
	// Match full names at the beginning of a clause, never a substring of a name.
	parts := strings.Fields(subject)
	if len(parts) == 0 {
		return "", false
	}
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	re := regexp.MustCompile(`(?i)^` + strings.Join(parts, `\s+`) + `(?:\s+|,\s*)`)
	idx := re.FindStringIndex(quote)
	if idx == nil {
		return "", false
	}
	// Retain the comma for the explicitly scoped alias form.
	rest := strings.TrimSpace(quote[idx[1]:])
	if strings.HasPrefix(rest, "(") {
		if end := strings.Index(rest, ")"); end > 0 && end < 220 {
			rest = strings.TrimSpace(rest[end+1:])
		}
	}
	if strings.HasSuffix(strings.TrimSpace(quote[:idx[1]]), ",") {
		rest = ", " + rest
	}
	if loc := aliasClause.FindStringIndex(rest); loc != nil {
		rest = strings.TrimSpace(rest[loc[1]:])
	}
	return rest, true
}

func Relations(subject string, docs []Document, asOf time.Time) []domain.Relation {
	out := []domain.Relation{}
	for _, doc := range docs {
		if doc.Type == "person_jsonld" {
			out = append(out, structuredRelations(subject, doc)...)
			continue
		}
		out = append(out, biographyRelations(subject, doc, asOf)...)
		out = append(out, profileRelations(subject, doc, asOf)...)
		out = append(out, attributedTitleRelations(subject, doc, asOf)...)
		for _, quote := range Sentences(doc.Text) {
			if len([]rune(quote)) > 600 || negatedRelation.MatchString(quote) {
				continue
			}
			rest, ok := subjectPrefix(quote, subject)
			if !ok {
				continue
			}
			r := domain.Relation{Subject: subject, EvidenceID: doc.ID, Quote: quote, Method: RelationVersion, Status: "UNVERIFIED", Temporal: "unknown"}
			// Non-employment relations must never populate the company summary.
			lower := strings.ToLower(rest)
			for _, marker := range []struct{ prefix, kind string }{{"was born at ", "birthplace"}, {"was born in ", "birthplace"}, {"lahir di ", "birthplace"}, {"studied at ", "education"}, {"belajar di ", "education"}, {"is a member of ", "membership"}, {"adalah anggota ", "membership"}} {
				if strings.HasPrefix(lower, marker.prefix) {
					r.Kind = marker.kind
					r.Organization = literalField(rest[len(marker.prefix):])
					r.StartYear, r.EndYear, r.Temporal = temporal(quote, "", asOf)
				}
			}
			if r.Kind == "" {
				m := relationVerb.FindStringSubmatch(rest)
				if len(m) == 0 {
					continue
				}
				verb, body := strings.ToLower(m[1]), primaryRoleClause(m[2])
				r.StartYear, r.EndYear, r.Temporal = temporal(quote, verb, asOf)
				body = regexp.MustCompile(`(?i)^(?:a|an|the|seorang|former|mantan)\s+`).ReplaceAllString(body, "")
				if publicOffice.MatchString(body) && (strings.Contains(strings.ToLower(body), "indonesia") || strings.Contains(strings.ToLower(body), "republik") || strings.Contains(strings.ToLower(body), "republic") || strings.Contains(strings.ToLower(body), "governor") || strings.Contains(strings.ToLower(body), "gubernur") || strings.Contains(strings.ToLower(body), "wali kota") || strings.Contains(strings.ToLower(body), "mayor")) {
					r.Kind = "public_office"
					r.Role = literalField(body)
				} else if strings.HasSuffix(verb, " at") || strings.HasSuffix(verb, " di") {
					r.Kind = "employment"
					r.Organization = literalField(body)
				} else if kind, org, role := profileRelationFields(body); kind != "" {
					r.Kind, r.Organization, r.Role = kind, org, role
				} else if loc := relationSeparator.FindStringIndex(body); loc != nil {
					r.Kind = "employment"
					r.Role = literalField(body[:loc[0]])
					r.Organization = literalField(body[loc[1]:])
					for _, word := range []string{"student", "undergraduate", "postgraduate", "mahasiswa", "mahasiswi", "pelajar"} {
						if containsWords(r.Role, word) {
							r.Kind = "education"
						}
					}
				} else {
					r.Kind = "occupation"
					r.Role = literalField(body)
				}
			}
			if r.Role != "" && !literalRole(r.Role) || r.Organization != "" && !validLiteral(r.Organization, 14) {
				continue
			}
			if r.Role == "" && r.Organization == "" {
				continue
			}
			if err := ValidateRelation(r, subject, doc); err == nil {
				out = append(out, r)
			}
		}
	}
	return out
}

// Biography introductions allow pronunciation, birth details and explicit aliases.
// The bounded relative clause remains attached to the named subject.
func biographyRelations(subject string, doc Document, asOf time.Time) []domain.Relation {
	if strings.TrimSpace(subject) == "" {
		return nil
	}
	pattern := `(?i)\b` + regexp.QuoteMeta(subject) + `(?:\s+\([^)]{0,220}\))?(?:,\s*(?:also known as|known as|often known mononymously as|lebih dikenal sebagai|dikenal sebagai)\s+[\p{L} -]{1,80}(?:\([^)]{0,120}\))?,?)?\s*(?:is|was|adalah|merupakan)\s+(?:an?\s+)?(?:[\p{L} ]{1,100}?\s+(?:who served as|who serves as|yang menjabat sebagai)\s+)?(?:the\s+)?([^.;\n|]{3,100}?)\s+(?:from|dari(?: tahun)?|pada)\s+((?:18|19|20|21)\d{2})\s*(?:to|until|sampai|hingga|-|–|—)\s*((?:18|19|20|21)\d{2})\b`
	out := []domain.Relation{}
	for _, m := range regexp.MustCompile(pattern).FindAllStringSubmatch(doc.Text, 12) {
		if negatedRelation.MatchString(m[0]) || !publicOffice.MatchString(m[1]) || !validLiteral(m[1], 16) {
			continue
		}
		start, _ := strconv.Atoi(m[2])
		end, _ := strconv.Atoi(m[3])
		_, _, state := temporal(m[0], "", asOf)
		r := domain.Relation{Subject: subject, Kind: "public_office", Role: strings.TrimSpace(m[1]), StartYear: start, EndYear: end, Temporal: state, EvidenceID: doc.ID, Quote: m[0], Method: RelationVersion, Status: "UNVERIFIED"}
		if ValidateRelation(r, subject, doc) == nil {
			out = append(out, r)
		}
	}
	return out
}

// ValidateRelation checks grounding, not the truth of the source or identity.
func ValidateRelation(r domain.Relation, subject string, doc Document) error {
	if !strings.EqualFold(compact(r.Subject), compact(subject)) || r.EvidenceID != doc.ID || len(r.Quote) < 3 || len([]rune(r.Quote)) > 1000 || !strings.Contains(doc.Text, r.Quote) {
		return fmt.Errorf("subject or citation mismatch")
	}
	if !containsWords(r.Quote, subject) {
		return fmt.Errorf("subject absent from quote")
	}
	switch r.Kind {
	case "employment", "public_office", "occupation", "education", "membership", "birthplace":
	default:
		return fmt.Errorf("unsupported relation kind")
	}
	if r.Role == "" && r.Organization == "" {
		return fmt.Errorf("empty relation")
	}
	for i, f := range []string{r.Role, r.Organization} {
		valid := validLiteral(f, 16)
		if i == 0 {
			valid = validRoleValue(f)
		}
		if f != "" && (!valid || !containsWords(r.Quote, f)) {
			return fmt.Errorf("field absent from quote or not a short literal")
		}
	}
	if r.StartYear != 0 && (r.StartYear < 1800 || r.StartYear > 2199 || !strings.Contains(r.Quote, strconv.Itoa(r.StartYear))) || r.EndYear != 0 && (r.EndYear < 1800 || r.EndYear > 2199 || !strings.Contains(r.Quote, strconv.Itoa(r.EndYear))) || r.StartYear != 0 && r.EndYear != 0 && r.StartYear > r.EndYear {
		return fmt.Errorf("unsupported period")
	}
	switch r.Temporal {
	case "unknown", "historical", "present_assertion", "headline", "future":
	default:
		return fmt.Errorf("unsupported temporal status")
	}
	return nil
}

func containsWords(text, value string) bool {
	t := []rune(strings.ToLower(compact(text)))
	v := []rune(strings.ToLower(compact(value)))
	for i := 0; i+len(v) <= len(t); i++ {
		if string(t[i:i+len(v)]) != string(v) {
			continue
		}
		if i > 0 && unicode.IsLetter(t[i-1]) || i+len(v) < len(t) && unicode.IsLetter(t[i+len(v)]) {
			continue
		}
		return true
	}
	return false
}
