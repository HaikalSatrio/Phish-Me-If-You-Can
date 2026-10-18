package collector

import "strings"

type Platform struct {
	Name    string
	Domains []string
	Mode    string
}

func Platforms() []Platform {
	return []Platform{
		{"instagram", []string{"instagram.com"}, "indexed discovery and public page metadata"},
		{"facebook", []string{"facebook.com"}, "indexed discovery and public page metadata"},
		{"linkedin", []string{"linkedin.com"}, "indexed discovery and public page metadata"},
		{"x", []string{"x.com", "twitter.com"}, "indexed discovery and public page metadata"},
		{"tiktok", []string{"tiktok.com"}, "indexed discovery and public page metadata"},
		{"youtube", []string{"youtube.com"}, "indexed discovery and public page metadata"},
		{"threads", []string{"threads.com", "threads.net"}, "indexed discovery and public page metadata"},
		{"reddit", []string{"reddit.com"}, "indexed discovery and public page metadata"},
		{"pinterest", []string{"pinterest.com"}, "indexed discovery and public page metadata"},
		{"bluesky", []string{"bsky.app"}, "public profile API, indexed discovery"},
		{"mastodon", []string{"mastodon.social"}, "indexed discovery for this instance; other instances via URL"},
		{"github", []string{"github.com"}, "public profile API, indexed discovery"},
	}
}

func platformFor(host string) string {
	host = strings.ToLower(host)
	for _, p := range Platforms() {
		for _, d := range p.Domains {
			if host == d || strings.HasSuffix(host, "."+d) {
				return p.Name
			}
		}
	}
	return "web"
}
