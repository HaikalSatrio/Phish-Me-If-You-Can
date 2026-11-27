package collector

import (
	"context"
	"strings"
	"testing"
	"time"
)

const syntheticGitHubHeader = `<meta property="og:type" content="profile"><meta property="og:url" content="https://github.com/mira-test"><meta property="og:title" content="mira-test (Mira Chen)"><span class="p-nickname" itemprop="additionalName">mira-test</span><span class="p-name" itemprop="name">Mira Chen</span>`
const syntheticGitHubDetails = `<ul class="vcard-details"><li itemprop="worksFor">Orbit Labs</li><li itemprop="homeLocation">Bandung</li><li itemprop="url"><a href="https://mira.example/">Website</a></li><li itemprop="email"><a href="mailto:mira@example.org">Email</a></li></ul>`

func TestGitHubNestedOrganizationURLIsNotPersonalWebsite(t *testing.T) {
	body := syntheticGitHubHeader + `<ul class="vcard-details"><li itemprop="worksFor"><a itemprop="url" href="https://orbit.example/">Orbit Labs</a></li></ul>`
	p, status, _ := parseDirect([]byte(body), DirectAdapters()[0], "mira-test", "https://github.com/mira-test")
	if status != "SUCCESS" || p.Company != "Orbit Labs" || p.Blog != "" {
		t.Fatalf("company link became a personal website: %+v", p)
	}
}

func TestGitHubIdentityTextExcludesHiddenChildren(t *testing.T) {
	header := strings.Replace(syntheticGitHubHeader, ">mira-test</span>", `>mira-test<span hidden>other-user</span></span>`, 1)
	header = strings.Replace(header, ">Mira Chen</span>", `>Mira Chen<span aria-hidden="true">Other Name</span></span>`, 1)
	p, status, _ := parseDirect([]byte(header+syntheticGitHubDetails), DirectAdapters()[0], "mira-test", "https://github.com/mira-test")
	if status != "SUCCESS" || p.Login != "mira-test" || p.Name != "Mira Chen" {
		t.Fatalf("hidden child polluted observed identity: status=%s profile=%+v", status, p)
	}
}

func TestMergedNameKeepsExactValueProvenance(t *testing.T) {
	source := FactSource{URL: "https://profile.example/", RequestURL: "https://profile.example/request", ObservedAt: "2026-11-28T00:00:00Z", ResponseSHA256: strings.Repeat("b", 64), Rule: "exact-name"}
	src := Profile{Name: "Mira Chen", SourceURL: "https://index.example/", FactSources: map[string]FactSource{"name\x00Mira Chen": source}}
	dst := Profile{}
	mergeProfileFacts(&dst, src)
	if got := SourceForFact(dst, Fact{"name", "Mira Chen"}); got != source {
		t.Fatalf("name provenance changed: got %+v want %+v", got, source)
	}
}

func TestGitHubHTMLMapsExplicitDetailsWithoutExtraRequests(t *testing.T) {
	body := syntheticGitHubHeader + syntheticGitHubDetails + `<main><div itemprop="worksFor">README Corp</div></main>`
	page := NewPage(time.Second)
	page.client = mockClient(body, "text/html", 200)
	r := page.Collect(context.Background(), Query{URL: "https://github.com/mira-test", Budget: 1})
	if r.Requests != 1 || len(r.Profiles) != 1 {
		t.Fatalf("unexpected collection: %+v", r)
	}
	p := r.Profiles[0]
	if p.Login != "mira-test" || p.Name != "Mira Chen" || p.Company != "Orbit Labs" || p.Location != "Bandung" || p.Email != "mira@example.org" || p.Blog != "https://mira.example/" {
		t.Fatalf("lost or misattributed fields: %+v", p)
	}
	for _, fact := range []Fact{{"workplace", p.Company}, {"location", p.Location}, {"email", p.Email}, {"website", p.Blog}} {
		source := SourceForFact(p, fact)
		if source.Rule != "github-html-v4" || source.URL != p.URL || source.ResponseSHA256 != p.ResponseSHA256 || len(source.ResponseSHA256) != 64 {
			t.Fatalf("wrong provenance for %+v: %+v", fact, source)
		}
	}
}

func TestGitHubHTMLDetailsRequireProfileMarkers(t *testing.T) {
	for _, tc := range []struct{ label, header, path string }{
		{"missing markers", "", "https://github.com/mira-test"},
		{"different account", strings.ReplaceAll(syntheticGitHubHeader, "mira-test", "other-user"), "https://github.com/mira-test"},
		{"repository", syntheticGitHubHeader, "https://github.com/mira-test/project"},
		{"hidden identity", `<div hidden>` + syntheticGitHubHeader + `</div>`, "https://github.com/mira-test"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			page := NewPage(time.Second)
			page.client = mockClient(tc.header+syntheticGitHubDetails, "text/html", 200)
			r := page.Collect(context.Background(), Query{URL: tc.path, Budget: 1})
			if len(r.Profiles) != 1 {
				t.Fatalf("expected metadata-only candidate: %+v", r)
			}
			p := r.Profiles[0]
			if p.Company != "" || p.Location != "" || p.Email != "" || p.Blog != "" {
				t.Fatalf("unbound fields promoted: %+v", p)
			}
		})
	}
}

func TestGitHubHTMLRejectsHiddenAndUnsafeAttributes(t *testing.T) {
	body := syntheticGitHubHeader + `<ul class="vcard-details"><li hidden itemprop="worksFor">Hidden Corp</li><li aria-hidden="true" itemprop="homeLocation">Hidden City</li><li style="display: none" itemprop="worksFor">Styled Corp</li><li itemprop="url"><a href="http://127.0.0.1/">Private</a><a href="javascript:alert(1)">Script</a></li><li itemprop="email"><a href="mailto:invalid">Bad email</a></li><li itemprop="worksFor">Orbit Labs<span hidden>Secret Corp</span></li></ul><template>` + syntheticGitHubDetails + `</template>`
	p, status, _ := parseDirect([]byte(body), DirectAdapters()[0], "mira-test", "https://github.com/mira-test")
	if status != "SUCCESS" || p.Company != "Orbit Labs" || p.Location != "" || p.Email != "" || p.Blog != "" {
		t.Fatalf("unsafe attributes accepted: %+v status=%s", p, status)
	}
}

func TestGitHubHTMLConflictingFieldsRemainSeparateClaims(t *testing.T) {
	body := syntheticGitHubHeader + syntheticGitHubDetails + `<ul class="vcard-details"><li itemprop="worksFor">Pine Labs</li></ul>`
	p, status, _ := parseDirect([]byte(body), DirectAdapters()[0], "mira-test", "https://github.com/mira-test")
	if status != "SUCCESS" || p.Company != "" {
		t.Fatalf("conflicting employers projected to one company: %+v", p)
	}
	workplaces := map[string]bool{}
	for _, f := range p.Facts {
		if f.Type == "workplace" {
			workplaces[f.Value] = true
		}
	}
	if len(workplaces) != 2 || !workplaces["Orbit Labs"] || !workplaces["Pine Labs"] {
		t.Fatalf("conflict evidence lost: %v", workplaces)
	}
}

func TestMergeProfilePreservesScalarFieldsAndConflictProvenance(t *testing.T) {
	dst := Profile{Company: "Orbit Labs", SourceURL: "https://index.example/", Rule: "index"}
	src := Profile{Company: "Pine Labs", Location: "Bandung", Email: "mira@example.org", Blog: "https://mira.example/", SourceURL: "https://github.com/mira-test", Rule: "github-html-v4", ResponseSHA256: strings.Repeat("a", 64)}
	mergeProfileFacts(&dst, src)
	mergeProfileFacts(&dst, src)
	if dst.Company != "" || dst.Location != src.Location || dst.Email != src.Email || dst.Blog != src.Blog {
		t.Fatalf("scalar fields lost or overwritten: %+v", dst)
	}
	count := 0
	originalRetained := false
	for _, f := range dst.Facts {
		if f.Type == "workplace" && f.Value == "Orbit Labs" {
			originalRetained = SourceForFact(dst, f).URL == "https://index.example/"
		}
		if f.Type == "workplace" && f.Value == "Pine Labs" {
			count++
			if source := SourceForFact(dst, f); source.URL != src.SourceURL || source.Rule != src.Rule || source.ResponseSHA256 != src.ResponseSHA256 {
				t.Fatalf("conflict source changed: %+v", source)
			}
		}
	}
	if count != 1 || !originalRetained || SourceForFact(dst, Fact{"location", "Bandung"}).URL != src.SourceURL {
		t.Fatalf("duplicate conflict or wrong scalar provenance: count=%d original=%v profile=%+v", count, originalRetained, dst)
	}
}
