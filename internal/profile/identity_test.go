package profile

import "testing"

func TestMononymProfileTitlesAndAccountScope(t *testing.T) {
	for _, tc := range []struct{ url, title, name string }{
		{"https://instagram.com/sari_dev", "Sari (@sari_dev) | Instagram", "Sari"},
		{"https://instagram.com/sari_dev/", "- (@sari_dev) | Instagram", ""},
		{"https://instagram.com/sari_dev", "Sari (@someone_else) | Instagram", ""},
		{"https://instagram.com/p/post", "Sari (@sari_dev) | Instagram", ""},
		{"https://id.linkedin.com/in/sari-123", "Sari Sari - CHFI | Lecturer", "Sari Sari"},
		{"https://linkedin.com.evil.example/in/sari", "Sari Sari - Lecturer", ""},
	} {
		name, ok := NameFromTitle(tc.url, tc.title)
		if ok != (tc.name != "") || ok && name != tc.name {
			t.Fatalf("%s: %q %v", tc.title, name, ok)
		}
	}
}
