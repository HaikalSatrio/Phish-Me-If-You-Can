package collector

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/personname"
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
	if conflictingLeadingName(q, raw, title, snippet) {
		return false
	}
	// A platform-scoped handle query seeks that account, not another user's mention.
	// Unscoped research queries still retain public third-party references.
	if q.Username != "" && q.Platform != "" && q.Platform != "all" && q.Platform != "web" {
		expected, err := accountURL(q.Platform, q.Username)
		actual, parseErr := cleanURL(raw)
		if err == nil && parseErr == nil {
			account, _ := url.Parse(expected)
			path, prefix := strings.ToLower(strings.TrimSuffix(actual.Path, "/")), strings.ToLower(strings.TrimSuffix(account.Path, "/"))
			if platformFor(actual.Hostname()) != q.Platform || path != prefix && !strings.HasPrefix(path, prefix+"/") {
				return false
			}
		}
	}
	return !directoryResult(raw) && (relevantResult(q, title+" "+snippet+" "+raw) || partialProfileName(q, raw, title))
}

func conflictingLeadingName(q Query, raw, title, snippet string) bool {
	if q.Username != "" || len(strings.Fields(q.Name)) < 2 {
		return false
	}
	name, ok := profile.NameFromTitle(raw, title)
	if !ok {
		u, err := url.Parse(raw)
		if err == nil && strings.HasSuffix(strings.ToLower(u.Hostname()), ".wikipedia.org") && strings.HasPrefix(u.Path, "/wiki/") && strings.HasSuffix(title, " - Wikipedia") {
			name = strings.TrimSuffix(title, " - Wikipedia")
			ok = plausiblePersonName(name)
		}
	}
	if !ok {
		return false
	}
	want, got := searchWords(q.Name), searchWords(name)
	if before, _, found := strings.Cut(got, want); found {
		honorificsOnly := strings.TrimSpace(before) != ""
		for _, word := range strings.Fields(before) {
			switch word {
			case "dr", "prof", "mr", "mrs", "ms", "bapak", "ibu":
			default:
				honorificsOnly = false
			}
		}
		if honorificsOnly {
			return false
		}
	}
	for _, claim := range personname.Find(q.Name, snippet) {
		if personname.Equal(claim.Name, name) || personname.Equal(claim.Alias, name) {
			return false
		}
	}
	alias := `(?im)(?:^|[.!?\n][ \t]*)` + regexp.QuoteMeta(strings.TrimSpace(name)) + `,?[ \t]+(?:is[ \t]+)?(?:also known as|known as|dikenal sebagai)[ \t]+` + regexp.QuoteMeta(strings.TrimSpace(q.Name)) + `(?:[.!,;]|$)`
	if regexp.MustCompile(alias).MatchString(snippet) {
		return false
	}
	return strings.Contains(" "+got+" ", " "+want+" ") && !strings.HasPrefix(got, want+" ") && got != want
}

// AcceptIndexedPerson exposes the same candidate filter used by public search
// collectors for replaying labeled index results without network requests.
func AcceptIndexedPerson(q Query, raw, title, snippet string) bool {
	return relevantSearchResult(q, raw, title, snippet)
}

func indexedProfile(q Query, raw, source, title, snippet string) Profile {
	u, _ := url.Parse(raw)
	p := Profile{Key: keyFor("web", raw), Platform: platformFor(u.Hostname()), URL: raw, SourceURL: source, Facts: []Fact{{"search_title", title}, {"search_snippet", snippet}, {"indexed_url", raw}}}
	if partialProfileName(q, raw, title) {
		p.DiscoveryBasis = "partial observed profile name; missing name components require operator identity review"
	}
	return p
}
