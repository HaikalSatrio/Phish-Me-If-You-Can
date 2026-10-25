package collector

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

var accountHandle = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,99}$`)

// Explicit backlinks are supporting clues, not proof of shared ownership.
func LinkedAccounts(p Profile, accounts []domain.AccountClue) []domain.AccountClue {
	links := map[string]bool{}
	for _, f := range p.Facts {
		if f.Type == "page_links" {
			for _, raw := range strings.Fields(f.Value) {
				links[discoveryURLKey(raw)] = true
			}
		}
	}
	out := []domain.AccountClue{}
	for _, account := range accounts {
		raw, err := accountURL(account.Platform, account.Username)
		if err == nil && discoveryURLKey(p.URL) != discoveryURLKey(raw) && links[discoveryURLKey(raw)] {
			out = append(out, account)
		}
	}
	return out
}

// Account URLs are discovery hints. Neither a route nor a matching handle proves ownership.
func accountURL(platform, username string) (string, error) {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if !accountHandle.MatchString(username) || strings.Contains(username, "..") {
		return "", fmt.Errorf("invalid account handle; supply a public URL for other handle formats")
	}
	prefixes := map[string]string{
		"github": "https://github.com/", "devto": "https://dev.to/",
		"instagram": "https://www.instagram.com/", "linkedin": "https://www.linkedin.com/in/",
		"facebook": "https://www.facebook.com/", "x": "https://x.com/",
		"tiktok": "https://www.tiktok.com/@", "youtube": "https://www.youtube.com/@",
		"threads": "https://www.threads.com/@", "reddit": "https://www.reddit.com/user/",
		"pinterest": "https://www.pinterest.com/", "bluesky": "https://bsky.app/profile/",
	}
	prefix, ok := prefixes[platform]
	if !ok {
		return "", fmt.Errorf("unsupported account platform %q; supply -url for other public profiles", platform)
	}
	if platform == "github" && !ValidLogin(username) {
		return "", fmt.Errorf("invalid GitHub username")
	}
	if platform == "bluesky" && !validBlueskyHandle(username) {
		return "", fmt.Errorf("Bluesky account clues require the full handle")
	}
	return prefix + url.PathEscape(username), nil
}

func NormalizeAccounts(accounts []domain.AccountClue) ([]domain.AccountClue, error) {
	if len(accounts) > 16 {
		return nil, fmt.Errorf("at most eight distinct account clues are supported")
	}
	out := []domain.AccountClue{}
	seen := map[string]bool{}
	for _, clue := range accounts {
		clue.Platform = strings.ToLower(strings.TrimSpace(clue.Platform))
		clue.Username = strings.TrimPrefix(strings.TrimSpace(clue.Username), "@")
		if _, err := accountURL(clue.Platform, clue.Username); err != nil {
			return nil, err
		}
		key := clue.Platform + ":" + strings.ToLower(clue.Username)
		if !seen[key] {
			seen[key] = true
			out = append(out, clue)
		}
	}
	if len(out) > 8 {
		return nil, fmt.Errorf("at most eight distinct account clues are supported")
	}
	return out, nil
}
