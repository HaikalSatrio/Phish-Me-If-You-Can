package extract

import "testing"

func TestExtractCompanyPatterns(t *testing.T) {
	tests := []struct {
		name    string
		input   Input
		want    string
		rule    string
		support string
		quote   string
	}{
		{
			name:    "explicit label",
			input:   Input{Name: "Yohan Muliono", Facts: map[string]string{"page_title": "Yohan Muliono", "page_text": "Company: Bina Nusantara University Location: Jakarta"}},
			want:    "Bina Nusantara University",
			rule:    "explicit-company-label-v2",
			support: "page_text",
			quote:   "Bina Nusantara University",
		},
		{
			name:    "employment phrase",
			input:   Input{Name: "Yohan Muliono", Facts: map[string]string{"search_snippet": "Yohan Muliono works at Bina Nusantara University."}},
			want:    "Bina Nusantara University",
			rule:    "employment-phrase-v1",
			support: "search_snippet",
		},
		{
			name:    "role and organization",
			input:   Input{Name: "Yohan Muliono", Facts: map[string]string{"search_snippet": "Yohan Muliono is Head of Cybersecurity Program in Bina Nusantara University, Indonesia."}},
			want:    "Bina Nusantara University",
			rule:    "role-organization-phrase-v2",
			support: "search_snippet",
		},
		{
			name:    "suffix after social counts",
			input:   Input{Name: "Linus Torvalds", Facts: map[string]string{"page_text": "Linus Torvalds torvalds Follow 329k followers · 0 following Linux Foundation Portland, OR"}},
			want:    "Linux Foundation",
			rule:    "organization-suffix-v2",
			support: "page_text",
		},
		{
			name:    "organization suffix in title",
			input:   Input{Name: "Yohan Muliono", Facts: map[string]string{"search_title": "Yohan Muliono | Head of Cyber Security Program Binus University"}},
			want:    "Binus University",
			rule:    "organization-suffix-v2",
			support: "search_title",
		},
		{
			name:    "indonesian employment",
			input:   Input{Name: "Budi Santoso", Facts: map[string]string{"page_text": "Budi Santoso bekerja di PT Maju Jaya."}},
			want:    "PT Maju Jaya",
			rule:    "indonesian-employment-phrase-v1",
			support: "page_text",
		},
		{
			name:    "uppercase organization",
			input:   Input{Name: "Yohan Muliono", Facts: map[string]string{"page_text": "Yohan Muliono works at BINUS UNIVERSITY."}},
			want:    "Binus University",
			rule:    "employment-phrase-v1",
			support: "page_text",
		},
		{
			name:    "sentence boundary before organization",
			input:   Input{Name: "Yohan Muliono", Facts: map[string]string{"search_snippet": "Yohan Muliono. CND | CEH | eMAPT | CAP | DFE | Lecturer | Penetration Tester | Information Security Awareness Trainer. BINUS University BINUS ..."}},
			want:    "Binus University",
			rule:    "organization-suffix-v2",
			support: "search_snippet",
		},
		{
			name:    "role words before organization",
			input:   Input{Name: "Yohan Muliono", Facts: map[string]string{"search_snippet": "Yohan Muliono – Certified Ethical Hacker BINUS CENTER baru saja berhasil mendapatkan penghargaan."}},
			want:    "Binus Center",
			rule:    "organization-suffix-v2",
			support: "search_snippet",
		},
		{
			name:    "email domain",
			input:   Input{Name: "Yohan Muliono", Email: "ymuliono@binus.ac.id", Facts: map[string]string{"email": "ymuliono@binus.ac.id"}},
			want:    "Binus",
			rule:    "email-domain-v1",
			support: "email",
		},
		{
			name:    "profile domain",
			input:   Input{Name: "Yohan Muliono", URL: "https://socs.binus.ac.id/people/yohan-muliono/", Facts: map[string]string{"search_title": "Yohan Muliono", "indexed_url": "https://socs.binus.ac.id/people/yohan-muliono/"}},
			want:    "Binus",
			rule:    "profile-domain-v1",
			support: "indexed_url",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ExtractCompany(test.input)
			if !ok {
				t.Fatal("no inference")
			}
			if got.Value != test.want || got.Rule != test.rule {
				t.Fatalf("got %+v, want value %q rule %q", got, test.want, test.rule)
			}
			found := false
			for _, support := range got.SupportingTypes {
				if support == test.support {
					found = true
				}
			}
			if !found || got.Confidence <= 0 || got.Confidence > 1 {
				t.Fatalf("invalid inference metadata: %+v", got)
			}
			if got.Quote == "" || got.QuoteType != test.support {
				t.Fatalf("missing inference audit trace: %+v", got)
			}
			if test.quote != "" && got.Quote != test.quote {
				t.Fatalf("got quote %q, want %q", got.Quote, test.quote)
			}
		})
	}
}

func TestInternationalCompanyPatterns(t *testing.T) {
	tests := []struct {
		name  string
		input Input
		want  string
		rule  string
	}{
		{
			name:  "german explicit label",
			input: Input{Name: "Lena Hoffmann", Facts: map[string]string{"page_text": "Lena Hoffmann. Unternehmen: Rheinwerk Solutions GmbH."}},
			want:  "Rheinwerk Solutions GmbH",
			rule:  "explicit-company-label-v2",
		},
		{
			name:  "spanish employment",
			input: Input{Name: "Lucía Fernández", Facts: map[string]string{"page_text": "Lucía Fernández trabaja en Meridian Solutions S.A."}},
			want:  "Meridian Solutions S.A.",
			rule:  "international-employment-phrase-v1",
		},
		{
			name:  "french employment",
			input: Input{Name: "Camille Dubois", Facts: map[string]string{"page_text": "Camille Dubois travaille chez Lumière Conseil SARL."}},
			want:  "Lumière Conseil SARL",
			rule:  "international-employment-phrase-v1",
		},
		{
			name:  "portuguese employment",
			input: Input{Name: "Mariana Silva", Facts: map[string]string{"page_text": "Mariana Silva trabalha na Aurora Tecnologia Ltda."}},
			want:  "Aurora Tecnologia Ltda",
			rule:  "international-employment-phrase-v1",
		},
		{
			name:  "italian employment",
			input: Input{Name: "Marco Rossi", Facts: map[string]string{"page_text": "Marco Rossi lavora presso Studio Bianchi S.r.l."}},
			want:  "Studio Bianchi S.r.l.",
			rule:  "international-employment-phrase-v1",
		},
		{
			name:  "dutch employment",
			input: Input{Name: "Sanne de Vries", Facts: map[string]string{"page_text": "Sanne de Vries werkt bij Delft Robotics B.V."}},
			want:  "Delft Robotics B.V.",
			rule:  "international-employment-phrase-v1",
		},
		{
			name:  "japanese role",
			input: Input{Name: "Kenji Sato", Facts: map[string]string{"search_snippet": "Kenji Sato is a Security Engineer at Sakura Systems K.K."}},
			want:  "Sakura Systems K.K.",
			rule:  "role-organization-phrase-v2",
		},
		{
			name:  "uk role",
			input: Input{Name: "Amara Okafor", Facts: map[string]string{"search_snippet": "Amara Okafor is a Product Manager at Northwind Traders PLC."}},
			want:  "Northwind Traders PLC",
			rule:  "role-organization-phrase-v2",
		},
		{
			name:  "india employment",
			input: Input{Name: "Priya Raman", Facts: map[string]string{"page_text": "Priya Raman works at Helios Analytics Pvt Ltd."}},
			want:  "Helios Analytics Pvt Ltd",
			rule:  "employment-phrase-v1",
		},
		{
			name:  "university infix",
			input: Input{Name: "Amara Okafor", Facts: map[string]string{"search_title": "Amara Okafor | University of Oxford"}},
			want:  "University of Oxford",
			rule:  "organization-infix-v1",
		},
		{
			name:  "german profile domain",
			input: Input{Name: "Jonas Weber", URL: "https://www.rheinwerk-solutions.de/team/jonas-weber", Facts: map[string]string{"search_title": "Jonas Weber", "indexed_url": "https://www.rheinwerk-solutions.de/team/jonas-weber"}},
			want:  "Rheinwerk Solutions",
			rule:  "profile-domain-v1",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ExtractCompany(test.input)
			if !ok {
				t.Fatal("no inference")
			}
			if got.Value != test.want || got.Rule != test.rule {
				t.Fatalf("got %+v, want value %q rule %q", got, test.want, test.rule)
			}
		})
	}
}

func TestEmployerLabel(t *testing.T) {
	tests := []struct {
		name  string
		input Input
		want  string
	}{
		{
			name:  "english employer label",
			input: Input{Name: "Alex Kim", Facts: map[string]string{"page_text": "Alex Kim. Employer Linux Foundation Known for Linux."}},
			want:  "Linux Foundation",
		},
		{
			name:  "indonesian employer label",
			input: Input{Name: "Alex Kim", Facts: map[string]string{"page_text": "Alex Kim. Pemberi kerja Linux Foundation Dikenal karena Linux."}},
			want:  "Linux Foundation",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ExtractCompany(test.input)
			if !ok {
				t.Fatal("no inference")
			}
			if got.Value != test.want || got.Rule != "employer-label-v1" {
				t.Fatalf("got %+v, want value %q rule employer-label-v1", got, test.want)
			}
		})
	}
}

func TestExtractCompanyRejectsUnsupportedInferences(t *testing.T) {
	tests := []struct {
		name  string
		input Input
	}{
		{"name mismatch", Input{Name: "Yohan Muliono", Facts: map[string]string{"page_text": "Someone Else works at Example Corp."}}},
		{"free email", Input{Name: "Yohan Muliono", Email: "yohan@gmail.com", Facts: map[string]string{"email": "yohan@gmail.com"}}},
		{"international free email", Input{Name: "Lucía Fernández", Email: "lucia@web.de", Facts: map[string]string{"email": "lucia@web.de"}}},
		{"international education", Input{Name: "Amara Okafor", Facts: map[string]string{"page_text": "Amara Okafor studied at University of Oxford."}}},
		{"international event", Input{Name: "Priya Raman", Facts: map[string]string{"page_text": "Priya Raman spoke at Google Cloud Summit."}}},
		{"generic legal form", Input{Name: "Jonas Weber", Facts: map[string]string{"page_text": "Jonas Weber Company: GmbH."}}},
		{"international location only", Input{Name: "Lena Hoffmann", Facts: map[string]string{"page_text": "Lena Hoffmann is based in Berlin."}}},
		{"role prose", Input{Name: "Alex Kim", Facts: map[string]string{"page_text": "Alex Kim is a software engineer. He bought professor Andrew Tanenbaum's book, in which Tanenbaum describes MINIX."}}},
		{"role article prose", Input{Name: "Alex Kim", Facts: map[string]string{"page_text": "Alex Kim is the developer of the Linux kernel, which is the kernel for a number of operating systems."}}},
		{"suffix prose", Input{Name: "Alex Kim", Facts: map[string]string{"page_text": "Alex Kim. His parents were campus radicals at the University."}}},
		{"award prose", Input{Name: "Alex Kim", Facts: map[string]string{"page_text": "Alex Kim received the 2012 Millennium Technology Prize."}}},
		{"alma mater field", Input{Name: "Alex Kim", Facts: map[string]string{"page_text": "Alex Kim. Almamater Universitas Helsinki Pekerjaan Rekayasawan perangkat lunak."}}},
		{"reference wiki domain", Input{Name: "Alex Kim", URL: "https://en.wikiquote.org/wiki/Alex_Kim", Facts: map[string]string{"page_text": "Alex Kim is a software engineer and the creator of a free kernel."}}},
		{"book reference domain", Input{Name: "Alex Kim", URL: "https://www.goodreads.com/author/quotes/1.Alex_Kim", Facts: map[string]string{"page_text": "Alex Kim is a software engineer and the author of a technical book."}}},
		{"article site title", Input{Name: "Linus Torvalds", URL: "https://lab.abilian.com/Tech/FOSS/People/Linus%20Torvalds/", Facts: map[string]string{"page_title": "Linus Torvalds - Abilian Innovation Lab", "page_text": "Linus Torvalds is best known for creating the Linux operating system."}}},
		{"award page text", Input{Name: "Alex Kim", Facts: map[string]string{"page_text": "Alex Kim received the Millennium Technology Prize from the Free Software Foundation."}}},
		{"social profile domain", Input{Name: "Yohan Muliono", URL: "https://www.linkedin.com/in/yohan-muliono", Facts: map[string]string{"search_title": "Yohan Muliono"}}},
		{"generic organization", Input{Name: "Yohan Muliono", Facts: map[string]string{"page_text": "Yohan Muliono visited the university today."}}},
		{"generic role phrase", Input{Name: "Yohan Muliono", Facts: map[string]string{"page_text": "Yohan Muliono is a manager in training."}}},
		{"generic company label", Input{Name: "Yohan Muliono", Facts: map[string]string{"page_text": "Yohan Muliono Company: Confidential"}}},
		{"self employment", Input{Name: "Yohan Muliono", Facts: map[string]string{"page_text": "Yohan Muliono works at Self-Employed."}}},
		{"lowercase phrase", Input{Name: "Yohan Muliono", Facts: map[string]string{"page_text": "Yohan Muliono works at the intersection of design and engineering."}}},
		{"missing name", Input{Facts: map[string]string{"page_text": "Head of Cybersecurity Program in Bina Nusantara University."}}},
		{"no supporting fact", Input{Name: "Yohan Muliono", URL: "https://socs.binus.ac.id/people/yohan-muliono/", Facts: map[string]string{"search_title": "Yohan Muliono"}}},
		{"personal domain", Input{Name: "Yohan Muliono", URL: "https://yohanmuliono.com/about", Facts: map[string]string{"search_title": "Yohan Muliono", "indexed_url": "https://yohanmuliono.com/about"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, ok := ExtractCompany(test.input); ok {
				t.Fatalf("unexpected inference: %+v", got)
			}
		})
	}
}

func TestInferenceQuoteIsFocused(t *testing.T) {
	got, ok := ExtractCompany(Input{
		Name: "Yohan Muliono",
		Facts: map[string]string{
			"search_snippet": "Yohan Muliono is a Head of Cybersecurity Program in Bina Nusantara University, Indonesia. Yohan received his Bachelor's degree in Computer Science.",
		},
	})
	if !ok {
		t.Fatal("no inference")
	}
	if got.Quote != "Bina Nusantara University" {
		t.Fatalf("got quote %q, want focused organization", got.Quote)
	}
}

func TestAmbiguousCompanyInferenceExposesAlternatives(t *testing.T) {
	got, ok := ExtractCompany(Input{
		Name: "Nadia Pratama",
		Facts: map[string]string{
			"page_text":      "Nadia Pratama works at Orbit Labs.",
			"search_snippet": "Nadia Pratama works at Meridian Group.",
		},
	})
	if !ok {
		t.Fatal("no inference")
	}
	if !got.Ambiguous || len(got.Alternatives) == 0 {
		t.Fatalf("ambiguous inference was hidden: %+v", got)
	}
	found := false
	for _, alternative := range got.Alternatives {
		if alternative.Value == "Orbit Labs" && alternative.Rule == "employment-phrase-v1" && alternative.Confidence > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing alternative audit: %+v", got.Alternatives)
	}
}

func TestConflictingDomainAndTextAreAmbiguous(t *testing.T) {
	got, ok := ExtractCompany(Input{
		Name: "Nadia Pratama",
		URL:  "https://orbit-labs.co.id/people/nadia-pratama/",
		Facts: map[string]string{
			"search_title": "Nadia Pratama",
			"page_text":    "Company: Nusantara University",
			"indexed_url":  "https://orbit-labs.co.id/people/nadia-pratama/",
		},
	})
	if !ok {
		t.Fatal("no inference")
	}
	if !got.Ambiguous {
		t.Fatalf("conflicting domain and text were hidden: %+v", got)
	}
	found := false
	for _, alternative := range got.Alternatives {
		if alternative.Value == "Orbit Labs" {
			found = true
		}
	}
	if !found {
		t.Fatalf("domain alternative was not retained: %+v", got.Alternatives)
	}
}

func TestRelatedOrganizationAliases(t *testing.T) {
	related := [][2]string{
		{"Binus", "Bina Nusantara University"},
		{"Binus University", "Bina Nusantara University"},
		{"Binus Center", "Binuscenter"},
		{"BNI", "Bank Negara Indonesia"},
		{"IBM", "International Business Machines"},
		{"MIT", "Massachusetts Institute of Technology"},
		{"Siemens AG", "Siemens"},
		{"Acme Pty Ltd", "Acme"},
		{"Sakura Systems K.K.", "Sakura Systems"},
	}
	for _, pair := range related {
		if !relatedOrganization(pair[0], pair[1]) {
			t.Errorf("%q and %q should be related", pair[0], pair[1])
		}
	}
	if relatedOrganization("Binus", "Orbit Labs") {
		t.Error("unrelated names should remain distinct")
	}
}

func TestAbbreviatedCompanyNamesAreRelated(t *testing.T) {
	got, ok := ExtractCompany(Input{
		Name: "Nadia Pratama",
		URL:  "https://binus.ac.id/people/nadia-pratama/",
		Facts: map[string]string{
			"search_title": "Nadia Pratama | Bina Nusantara University",
			"indexed_url":  "https://binus.ac.id/people/nadia-pratama/",
		},
	})
	if !ok {
		t.Fatal("no inference")
	}
	if got.Ambiguous {
		t.Fatalf("abbreviation marked as an unrelated conflict: %+v", got)
	}
}

func TestRelatedCompanyNamesAreNotAmbiguous(t *testing.T) {
	got, ok := ExtractCompany(Input{
		Name: "Nadia Pratama",
		URL:  "https://nusantara.ac.id/people/nadia-pratama/",
		Facts: map[string]string{
			"search_title": "Nadia Pratama | Nusantara University",
			"indexed_url":  "https://nusantara.ac.id/people/nadia-pratama/",
		},
	})
	if !ok {
		t.Fatal("no inference")
	}
	if got.Ambiguous {
		t.Fatalf("related organization names marked ambiguous: %+v", got)
	}
	if len(got.Alternatives) == 0 {
		t.Fatalf("related alternative was not retained for audit: %+v", got)
	}
}

func TestDomainAgreementRaisesConfidence(t *testing.T) {
	withoutDomain, ok := ExtractCompany(Input{Name: "Yohan Muliono", Facts: map[string]string{"search_title": "Yohan Muliono | Binus University"}})
	if !ok {
		t.Fatal("no baseline inference")
	}
	withDomain, ok := ExtractCompany(Input{Name: "Yohan Muliono", URL: "https://socs.binus.ac.id/people/yohan-muliono/", Facts: map[string]string{"search_title": "Yohan Muliono | Binus University", "indexed_url": "https://socs.binus.ac.id/people/yohan-muliono/"}})
	if !ok {
		t.Fatal("no domain inference")
	}
	if withDomain.Value != withoutDomain.Value || withDomain.Confidence <= withoutDomain.Confidence {
		t.Fatalf("domain agreement did not raise confidence: %+v vs %+v", withDomain, withoutDomain)
	}
}
