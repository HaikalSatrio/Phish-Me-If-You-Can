package profile

import (
	"net/url"
	"strings"
	"unicode"
)

// Account identifies profile routes, excluding posts and platform landing pages.
func Account(raw string) (platform, handle string, ok bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return
	}
	host := strings.ToLower(u.Hostname())
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if (host == "instagram.com" || host == "www.instagram.com") && len(parts) == 1 && parts[0] != "" {
		switch strings.ToLower(parts[0]) {
		case "accounts", "explore", "reels", "p", "stories":
			return
		}
		return "instagram", parts[0], true
	}
	if (host == "linkedin.com" || strings.HasSuffix(host, ".linkedin.com")) && len(parts) == 2 && parts[0] == "in" && parts[1] != "" {
		return "linkedin", parts[1], true
	}
	return
}

func DisplayName(s string) bool {
	if len(s) > 150 || len(strings.Fields(s)) < 1 || len(strings.Fields(s)) > 6 {
		return false
	}
	letters := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
			continue
		}
		if !unicode.IsSpace(r) && !strings.ContainsRune("-'.", r) {
			return false
		}
	}
	return letters > 0
}

func NameFromTitle(raw, title string) (string, bool) {
	platform, handle, ok := Account(raw)
	if !ok {
		return "", false
	}
	var name string
	if platform == "instagram" {
		prefix, rest, found := strings.Cut(title, " (@")
		observed, _, end := strings.Cut(rest, ")")
		if !found || !end || !strings.EqualFold(handle, observed) {
			return "", false
		}
		name = prefix
	} else {
		prefix, _, found := strings.Cut(title, " - ")
		if !found {
			return "", false
		}
		name = prefix
	}
	name = strings.TrimSpace(name)
	return name, DisplayName(name)
}
