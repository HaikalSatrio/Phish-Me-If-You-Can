package collector

import (
	"net/url"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/profile"
)

func directoryResult(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return true
	}
	path := strings.ToLower(u.Path)
	switch platformFor(u.Hostname()) {
	case "linkedin":
		return strings.HasPrefix(path, "/pub/dir/") || strings.HasPrefix(path, "/directory/") || strings.HasPrefix(path, "/search/")
	case "facebook":
		return strings.HasPrefix(path, "/public/") || strings.HasPrefix(path, "/search/")
	}
	return false
}

// Partial names admit review candidates only when an individual profile title
// contains an ordered subset of the supplied name, with at least two words.
func partialProfileName(q Query, raw, title string) bool {
	if q.Username != "" {
		return false
	}
	name, ok := profile.NameFromTitle(raw, title)
	if !ok {
		return false
	}
	want, got := strings.Fields(searchWords(q.Name)), strings.Fields(searchWords(name))
	if len(got) < 2 || len(got) >= len(want) {
		return false
	}
	at := 0
	for _, word := range want {
		if at < len(got) && word == got[at] {
			at++
		}
	}
	return at == len(got)
}

func relevantSearchResult(q Query, raw, title, snippet string) bool {
	return !directoryResult(raw) && (relevantResult(q, title+" "+snippet+" "+raw) || partialProfileName(q, raw, title))
}

func indexedProfile(q Query, raw, source, title, snippet string) Profile {
	u, _ := url.Parse(raw)
	p := Profile{Key: keyFor("web", raw), Platform: platformFor(u.Hostname()), URL: raw, SourceURL: source, Facts: []Fact{{"search_title", title}, {"search_snippet", snippet}, {"indexed_url", raw}}}
	if partialProfileName(q, raw, title) {
		p.DiscoveryBasis = "partial observed profile name; missing name components require operator identity review"
	}
	return p
}
