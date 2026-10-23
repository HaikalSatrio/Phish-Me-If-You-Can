package extract

import (
	"strings"
	"testing"
)

func TestExtractRole(t *testing.T) {
	tests := []struct {
		name        string
		person      string
		factType    string
		text        string
		want        string
		wantRule    string
		wantSupport string
	}{
		{
			name:        "explicit role label",
			person:      "Yohan Muliono",
			factType:    "search_snippet",
			text:        "Yohan Muliono, S.Kom., M.TI., CND, CEHmaster, eMAPT. Head of Cyber Security Program. D5543. Cyber Security | Computer Science.",
			want:        "Head of Cyber Security Program",
			wantRule:    "role-head-v1",
			wantSupport: "search_snippet",
		},
		{
			name:        "role after is",
			person:      "Yohan Muliono",
			factType:    "search_snippet",
			text:        "Yohan Muliono is Head of Cybersecurity Program in Bina Nusantara University, Indonesia.",
			want:        "Head of Cybersecurity Program",
			wantRule:    "role-is-v1",
			wantSupport: "search_snippet",
		},
		{
			name:        "occupation field",
			person:      "Linus Torvalds",
			factType:    "page_text",
			text:        "Linus Torvalds. Occupation Software engineer Employer Linux Foundation",
			want:        "Software Engineer",
			wantRule:    "role-label-v1",
			wantSupport: "page_text",
		},
		{
			name:        "indonesian occupation field",
			person:      "Linus Torvalds",
			factType:    "page_text",
			text:        "Linus Torvalds. Pekerjaan Rekayasawan perangkat lunak Pemberi kerja Linux Foundation",
			want:        "Software Engineer",
			wantRule:    "role-label-v1",
			wantSupport: "page_text",
		},
		{
			name:        "works as",
			person:      "Nadia Putri",
			factType:    "search_snippet",
			text:        "Nadia Putri works as a software engineer at Acme.",
			want:        "Software Engineer",
			wantRule:    "role-as-v1",
			wantSupport: "search_snippet",
		},
		{
			name:        "bekerja sebagai",
			person:      "Nadia Putri",
			factType:    "search_snippet",
			text:        "Nadia Putri bekerja sebagai dosen di Universitas Indonesia.",
			want:        "Lecturer",
			wantRule:    "role-id-as-v1",
			wantSupport: "search_snippet",
		},
		{
			name:        "title segment",
			person:      "Yohan Muliono",
			factType:    "search_title",
			text:        "Yohan Muliono - CND | CEH | eMAPT | CAP | DFE | Lecturer - LinkedIn",
			want:        "Lecturer",
			wantRule:    "role-title-v1",
			wantSupport: "search_title",
		},
		{
			name:        "page title head",
			person:      "Yohan Muliono",
			factType:    "page_title",
			text:        "Yohan Muliono | Head of Cyber Security Program | Binus",
			want:        "Head of Cyber Security Program",
			wantRule:    "role-title-v1",
			wantSupport: "page_title",
		},
		{
			name:        "international article role",
			person:      "Linus Torvalds",
			factType:    "search_snippet",
			text:        "Linus Torvalds is a Finnish-American computer programmer.",
			want:        "Finnish-American Computer Programmer",
			wantRule:    "role-is-v1",
			wantSupport: "search_snippet",
		},
		{
			name:        "role label with colon",
			person:      "Alex Kim",
			factType:    "page_text",
			text:        "Alex Kim. Role: Security Consultant. Location: Jakarta.",
			want:        "Security Consultant",
			wantRule:    "role-label-v1",
			wantSupport: "page_text",
		},
		{
			name:        "indonesian role label",
			person:      "Budi Santoso",
			factType:    "page_text",
			text:        "Budi Santoso. Jabatan: Kepala Divisi Keamanan Informasi. Lokasi: Jakarta.",
			want:        "Head of Information Security Division",
			wantRule:    "role-label-v1",
			wantSupport: "page_text",
		},
		{
			name:        "indonesian director role",
			person:      "Budi Santoso",
			factType:    "search_snippet",
			text:        "Budi Santoso bekerja sebagai Direktur Teknologi di Acme.",
			want:        "Technology Director",
			wantRule:    "role-id-as-v1",
			wantSupport: "search_snippet",
		},
		{
			name:        "indonesian leadership connectors",
			person:      "Budi Santoso",
			factType:    "search_snippet",
			text:        "Budi Santoso bekerja sebagai CEO dan CTO di Acme.",
			want:        "CEO and CTO",
			wantRule:    "role-id-as-v1",
			wantSupport: "search_snippet",
		},
		{
			name:        "indonesian general treasurer",
			person:      "Brian Filbert Chandra",
			factType:    "search_snippet",
			text:        "Brian Filbert Chandra bekerja sebagai Bendahara Umum di HIMTI BINUS University.",
			want:        "General Treasurer",
			wantRule:    "role-id-as-v1",
			wantSupport: "search_snippet",
		},
		{
			name:        "indonesian student role",
			person:      "Nadia Putri",
			factType:    "search_snippet",
			text:        "Nadia Putri adalah seorang mahasiswa di Universitas Indonesia.",
			want:        "Student",
			wantRule:    "role-is-v1",
			wantSupport: "search_snippet",
		},
		{
			name:        "indonesian infobox role with organization",
			person:      "Elon Musk",
			factType:    "page_text",
			text:        "Elon Musk. Pekerjaan CEO dan Arsitek Produk Tesla Pendiri.",
			want:        "CEO and Product Architect",
			wantRule:    "role-label-v1",
			wantSupport: "page_text",
		},
		{
			name:        "linkedin leadership headline",
			person:      "Brian Filbert Chandra",
			factType:    "search_snippet",
			text:        "Brian Filbert Chandra. Cybersecurity Undergraduate at BINUS University | General Treasurer of HIMTI BINUS University | Malang, Indonesia",
			want:        "General Treasurer",
			wantRule:    "role-headline-of-v1",
			wantSupport: "search_snippet",
		},
		{
			name:        "headline at organization",
			person:      "Brian Filbert Chandra",
			factType:    "search_snippet",
			text:        "Brian Filbert Chandra. Cybersecurity Undergraduate at BINUS University | Malang, Indonesia",
			want:        "Cybersecurity Undergraduate",
			wantRule:    "role-headline-at-v1",
			wantSupport: "search_snippet",
		},
		{
			name:        "ceo of organization",
			person:      "Elon Musk",
			factType:    "search_snippet",
			text:        "Elon Musk. Co-founder and CEO of Tesla | entrepreneur and investor",
			want:        "Co-founder and CEO",
			wantRule:    "role-headline-of-v1",
			wantSupport: "search_snippet",
		},
		{
			name:        "role label splits organization",
			person:      "Brian Filbert Chandra",
			factType:    "page_text",
			text:        "Brian Filbert Chandra. Role: General Treasurer of HIMTI BINUS University. Location: Malang.",
			want:        "General Treasurer",
			wantRule:    "role-label-v1",
			wantSupport: "page_text",
		},
		{
			name:        "possessive role predicate",
			person:      "Brian Filbert Chandra",
			factType:    "page_text",
			text:        "Brian Filbert Chandra serves as HIMTI's General Treasurer managing budgets for 60+ events.",
			want:        "General Treasurer",
			wantRule:    "role-as-v1",
			wantSupport: "page_text",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ExtractRole(Input{Name: test.person, Facts: map[string]string{test.factType: test.text}})
			if !ok {
				t.Fatalf("expected role, got none")
			}
			if got.Value != test.want {
				t.Fatalf("role = %q, want %q", got.Value, test.want)
			}
			if got.Rule != test.wantRule {
				t.Fatalf("rule = %q, want %q", got.Rule, test.wantRule)
			}
			if got.Quote == "" || got.QuoteType != test.wantSupport {
				t.Fatalf("audit = %#v", got)
			}
		})
	}
}

func TestCanonicalEnglishRole(t *testing.T) {
	tests := map[string]string{
		"Bendahara Umum":                       "General Treasurer",
		"CEO dan CTO":                          "CEO and CTO",
		"CEO dan Arsitek Produk Tesla Pendiri": "CEO and Product Architect",
		"Dosen":                                "Lecturer",
		"Dosen Teknik Informatika":             "Lecturer Informatics Engineering",
		"Direktur Teknologi":                   "Technology Director",
		"Kepala Divisi Keamanan Informasi":     "Head of Information Security Division",
		"Kepala Program Studi":                 "Head of Study Program",
		"Mahasiswa":                            "Student",
		"Mahasiswa Teknik Informatika":         "Student Informatics Engineering",
		"Pelatih":                              "Trainer",
		"Rekayasawan Perangkat Lunak":          "Software Engineer",
		"Rekayasawan Perangkat Lunak Senior":   "Software Engineer Senior",
		"Software Engineer":                    "Software Engineer",
		"Wakil Ketua":                          "Vice Chair",
	}
	for input, want := range tests {
		if got := canonicalEnglishRole(input); got != want {
			t.Errorf("canonicalEnglishRole(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestExtractRoleKeepsOriginalQuote(t *testing.T) {
	got, ok := ExtractRole(Input{Name: "Budi Santoso", Facts: map[string]string{
		"page_text": "Budi Santoso. Jabatan: Kepala Divisi Keamanan Informasi. Lokasi: Jakarta.",
	}})
	if !ok {
		t.Fatal("expected role")
	}
	if got.Value != "Head of Information Security Division" {
		t.Fatalf("role = %q", got.Value)
	}
	if !strings.Contains(got.Quote, "Kepala Divisi Keamanan Informasi") {
		t.Fatalf("quote = %q, want original Indonesian evidence", got.Quote)
	}
}

func TestExtractRoleRetainsAlternatives(t *testing.T) {
	got, ok := ExtractRole(Input{Name: "Yohan Muliono", Facts: map[string]string{
		"search_snippet": "Yohan Muliono. Role: Head of Cyber Security Program.",
		"page_text":      "Yohan Muliono. Role: Lecturer.",
	}})
	if !ok {
		t.Fatal("expected role")
	}
	if got.Value != "Head of Cyber Security Program" {
		t.Fatalf("role = %q", got.Value)
	}
	if len(got.Alternatives) != 1 || got.Alternatives[0].Value != "Lecturer" || got.Alternatives[0].Rule == "" {
		t.Fatalf("alternatives = %#v", got.Alternatives)
	}
	if len(got.SupportingTypes) != 1 || got.SupportingTypes[0] != "search_snippet" {
		t.Fatalf("supporting types = %#v", got.SupportingTypes)
	}
}

func TestExtractRoleRejectsUnrelatedText(t *testing.T) {
	tests := []struct {
		name   string
		person string
		fact   string
		text   string
	}{
		{
			name:   "company only",
			person: "Yohan Muliono",
			fact:   "search_snippet",
			text:   "Yohan Muliono is associated with Bina Nusantara University in Jakarta.",
		},
		{
			name:   "unrelated role",
			person: "Yohan Muliono",
			fact:   "search_snippet",
			text:   "Professor Andrew Tanenbaum wrote the operating system.",
		},
		{
			name:   "name only label",
			person: "Yohan Muliono",
			fact:   "page_title",
			text:   "Title: Yohan Muliono",
		},
		{
			name:   "no role context",
			person: "Yohan Muliono",
			fact:   "search_snippet",
			text:   "Yohan Muliono is based in Jakarta and studied computer science.",
		},
		{
			name:   "missing subject",
			person: "Yohan Muliono",
			fact:   "search_snippet",
			text:   "The company is hiring a software engineer.",
		},
		{
			name:   "unrelated title",
			person: "Yohan Muliono",
			fact:   "page_title",
			text:   "Professor Andrew Tanenbaum",
		},
		{
			name:   "unrelated person phrase",
			person: "Yohan Muliono",
			fact:   "search_snippet",
			text:   "Nadia Putri works as a software engineer at Acme.",
		},
		{
			name:   "negated role",
			person: "Yohan Muliono",
			fact:   "search_snippet",
			text:   "Yohan Muliono is not a manager.",
		},
		{
			name:   "former role",
			person: "Yohan Muliono",
			fact:   "search_snippet",
			text:   "Yohan Muliono is a former lecturer.",
		},
		{
			name:   "indonesian former role",
			person: "Yohan Muliono",
			fact:   "search_snippet",
			text:   "Yohan Muliono adalah mantan dosen.",
		},
		{
			name:   "organization page title",
			person: "Brian Filbert Chandra",
			fact:   "page_title",
			text:   "Himpunan Mahasiswa Teknik Informatika \u2013 Brian Filbert Chandra",
		},
		{
			name:   "book title role",
			person: "Elon Musk",
			fact:   "page_title",
			text:   "Elon Musk: How the Billionaire CEO of Spacex and Tesla is Shaping Our Future",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, ok := ExtractRole(Input{Name: test.person, Facts: map[string]string{test.fact: test.text}}); ok {
				t.Fatalf("unexpected role inference: %#v", got)
			}
		})
	}
}
