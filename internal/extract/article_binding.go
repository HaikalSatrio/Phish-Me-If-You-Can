package extract

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

const ArticleBindingVersion = "article-binding-v1"

var introductoryName = regexp.MustCompile(`^([\p{Lu}][\p{L}\p{M}'\x{2019}.\-]*(?:[ \t]+(?:[\p{Lu}][\p{L}\p{M}'\x{2019}.\-]*|van|von|de|del|da|dos|di|la|der|den|bin|al)){1,7})(?:\s|\[|\()`)
var birthAnnotation = regexp.MustCompile(`(?i)\b(?:born|lahir)\b`)

func nameExpansion(short, full string) bool {
	words := func(s string) []string {
		return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return unicode.IsSpace(r) || r == '-' })
	}
	want, got := words(short), words(full)
	if len(want) < 2 || len(got) <= len(want) || len(got) > 8 || want[0] != got[0] {
		return false
	}
	at := 0
	for _, part := range got {
		if at < len(want) && want[at] == part {
			at++
		}
	}
	return at == len(want)
}

// An article heading and its opening birth introduction can bind an expanded
// personal name within that article only. This never links separate profiles.
func BoundArticleRelations(subject, raw string, docs []Document, asOf time.Time) []domain.Relation {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "en.wikipedia.org" || !strings.HasPrefix(u.Path, "/wiki/") || strings.Contains(strings.TrimPrefix(u.Path, "/wiki/"), ":") {
		return nil
	}
	var title Document
	for _, d := range docs {
		if d.Type == "page_title" && strings.EqualFold(compact(d.Text), compact(subject)+" - Wikipedia") {
			title = d
			break
		}
	}
	if title.ID == "" {
		return nil
	}
	out := []domain.Relation{}
	for _, d := range docs {
		if d.Type != "page_description" {
			continue
		}
		text := strings.TrimSpace(d.Text)
		m := introductoryName.FindStringSubmatch(text)
		if len(m) == 0 || !nameExpansion(subject, m[1]) {
			continue
		}
		clauses := Sentences(text)
		if len(clauses) == 0 || len([]rune(clauses[0])) > 1000 {
			continue
		}
		quote := clauses[0]
		open, close := strings.Index(quote, "("), strings.LastIndex(quote, ")")
		if open < 0 || close <= open || !birthAnnotation.MatchString(quote[open:close]) {
			continue
		}
		lead := Document{ID: d.ID, Type: d.Type, Text: quote}
		for _, r := range Relations(m[1], []Document{lead}, asOf) {
			r.Subject, r.SubjectEvidenceID, r.SubjectQuote = subject, title.ID, title.Text
			r.Method = ArticleBindingVersion
			if ValidateRelation(r, subject, d) == nil {
				out = append(out, r)
			}
		}
	}
	return out
}

func ValidateArticleBinding(r domain.Relation, raw string, body, title Document, asOf time.Time) error {
	if r.Method != ArticleBindingVersion || r.SubjectEvidenceID != title.ID || r.SubjectQuote != title.Text {
		return fmt.Errorf("invalid article subject reference")
	}
	for _, p := range BoundArticleRelations(r.Subject, raw, []Document{title, body}, asOf) {
		if p.Quote == r.Quote && p.Kind == r.Kind && p.Role == r.Role && p.Organization == r.Organization && p.StartYear == r.StartYear && p.EndYear == r.EndYear && p.Temporal == r.Temporal {
			return nil
		}
	}
	return fmt.Errorf("article heading and lead do not support this claim")
}
