package extract

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

// SelectPassages retains exact source spans and bounds the complete model context.
func SelectPassages(subject string, docs []Document) []domain.ExtractionPassage {
	ordered := append([]Document(nil), docs...)
	priority := map[string]int{"page_description": 6, "search_snippet": 5, "person_jsonld": 4, "page_text": 3, "profile_description": 3, "page_title": 2, "search_title": 1}
	sort.SliceStable(ordered, func(i, j int) bool { return priority[ordered[i].Type] > priority[ordered[j].Type] })
	out := []domain.ExtractionPassage{}
	seen := map[string]bool{}
	remaining := 12000
	add := func(doc Document, text string) {
		text = strings.TrimSpace(text)
		key := strings.ToLower(compact(text))
		size := len([]rune(text))
		if seen[key] || size > 1000 || size > remaining || len(out) >= 24 || !containsWords(text, subject) {
			return
		}
		seen[key] = true
		remaining -= size
		out = append(out, domain.ExtractionPassage{ID: fmt.Sprintf("s%d", len(out)+1), EvidenceID: doc.ID, Type: doc.Type, Text: text})
	}
	continuation := regexp.MustCompile(`(?i)^(?:she|he|ia|dia)\s`)
	for _, doc := range ordered {
		if subject == "" || !containsWords(doc.Text, subject) {
			continue
		}
		if len([]rune(doc.Text)) <= 1000 {
			add(doc, doc.Text)
			continue
		}
		sentences := Sentences(doc.Text)
		offset := 0
		for i, sentence := range sentences {
			at := strings.Index(doc.Text[offset:], sentence)
			if at < 0 {
				continue
			}
			start := offset + at
			end := start + len(sentence)
			offset = end
			if !containsWords(sentence, subject) {
				continue
			}
			if i+1 < len(sentences) && continuation.MatchString(sentences[i+1]) {
				next := strings.Index(doc.Text[end:], sentences[i+1])
				if next >= 0 && len([]rune(doc.Text[start:end+next+len(sentences[i+1])])) <= 1000 {
					end += next + len(sentences[i+1])
				}
			}
			add(doc, doc.Text[start:end])
		}
	}
	return out
}
