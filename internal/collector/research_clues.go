package collector

import (
	"encoding/json"
	"net/url"
	"strings"
	"unicode"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/profile"
	"golang.org/x/net/html"
)

type researchLink struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

// Only explicit profile fields supply observed names. Handle splitting is separate.
func pageIdentity(doc *html.Node, base *url.URL, facts []Fact) (string, []researchLink) {
	name, heading := "", ""
	links := []researchLink{}
	seen := map[string]bool{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, a := range n.Attr {
				if a.Key == "hidden" || a.Key == "aria-hidden" && a.Val == "true" {
					return
				}
			}
			switch n.Data {
			case "script", "style", "template", "footer", "form":
				return
			}
			if n.Data == "h1" && heading == "" {
				heading = nodeText(n)
			}
			if platformFor(base.Hostname()) == "github" && len(strings.Split(strings.Trim(base.Path, "/"), "/")) == 1 && hasClass(n, "p-name") && hasClass(n, "vcard-fullname") {
				name = nodeText(n)
			}
			if n.Data == "a" && len(links) < 8 {
				label := strings.ToLower(strings.TrimSpace(nodeText(n)))
				reason := ""
				for _, rel := range strings.Fields(attr(n, "rel")) {
					if rel == "me" {
						reason = "profile self-link (ownership not verified)"
					}
				}
				switch label {
				case "about", "bio", "biography":
					reason = "same-site biography navigation"
				case "about me", "my website", "personal website", "my portfolio", "portfolio", "resume", "curriculum vitae", "tentang saya":
					reason = "explicit biography or portfolio link"
				}
				if ref, err := url.Parse(attr(n, "href")); err == nil && reason != "" && attr(n, "href") != "" && !strings.HasPrefix(attr(n, "href"), "#") {
					if u, err := cleanURL(base.ResolveReference(ref).String()); err == nil && u.RawQuery == "" && !seen[u.String()] && (reason != "same-site biography navigation" || u.Hostname() == base.Hostname()) {
						seen[u.String()] = true
						links = append(links, researchLink{u.String(), reason})
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	for _, f := range facts {
		if f.Type == "person_jsonld" && name == "" {
			var people []domain.PersonRecord
			if json.Unmarshal([]byte(f.Value), &people) == nil && len(people) == 1 && strings.EqualFold(strings.TrimSpace(people[0].Name), heading) {
				name = people[0].Name
			}
		}
		if f.Type == "page_title" {
			if observed, ok := profile.NameFromTitle(base.String(), f.Value); ok {
				name = observed
			}
		}
	}
	return strings.TrimSpace(name), links
}

func plausiblePersonName(s string) bool {
	parts := strings.Fields(s)
	if len(parts) < 2 || len(parts) > 6 || len(s) > 150 {
		return false
	}
	for _, part := range parts {
		for _, r := range part {
			if !unicode.IsLetter(r) && !strings.ContainsRune("-'.", r) {
				return false
			}
		}
	}
	return true
}

// A split handle is a search hypothesis, never a name observation.
func handleHypothesis(handle, suppliedName string) string {
	if suppliedName == "" || len(strings.Fields(suppliedName)) != 1 {
		return ""
	}
	var b strings.Builder
	var previous rune
	for _, r := range handle {
		if r == '_' || r == '-' || r == '.' {
			b.WriteByte(' ')
			previous = r
			continue
		}
		if !unicode.IsLetter(r) {
			return ""
		}
		if unicode.IsUpper(r) && unicode.IsLower(previous) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		previous = r
	}
	name := strings.Join(strings.Fields(b.String()), " ")
	if !plausiblePersonName(name) || !strings.EqualFold(strings.Fields(name)[0], suppliedName) {
		return ""
	}
	return name
}

func researchLinks(p Profile) []researchLink {
	out := []researchLink{}
	for _, f := range p.Facts {
		if f.Type == "profile_links" {
			var links []researchLink
			if json.Unmarshal([]byte(f.Value), &links) == nil {
				out = append(out, links...)
			}
		}
	}
	u, err := cleanURL(p.URL)
	if err != nil {
		return out
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for _, f := range p.Facts {
		if f.Type != "page_links" {
			continue
		}
		for _, raw := range strings.Fields(f.Value) {
			link, err := cleanURL(raw)
			if err != nil || link.RawQuery != "" || link.Hostname() != u.Hostname() {
				continue
			}
			path := strings.Trim(link.Path, "/")
			if platformFor(u.Hostname()) == "web" {
				switch strings.ToLower(path) {
				case "about", "about-me", "bio", "biography", "cv", "resume":
					out = append(out, researchLink{raw, "same-site biography path; identity unverified"})
				}
			}
			if platformFor(u.Hostname()) == "github" && len(parts) == 1 && strings.EqualFold(path, parts[0]+"/"+parts[0]) {
				out = append(out, researchLink{raw, "profile README repository linked by account"})
			}
		}
	}
	return out
}
