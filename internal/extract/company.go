package extract

import (
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type Input struct {
	Name  string
	Email string
	URL   string
	Facts map[string]string
}

type Alternative struct {
	Value      string
	Rule       string
	Confidence float64
}

type Inference struct {
	Value           string
	Confidence      float64
	Rule            string
	Quote           string
	QuoteType       string
	SupportingTypes []string
	Alternatives    []Alternative
	Ambiguous       bool
}

type textRule struct {
	name       string
	re         *regexp.Regexp
	confidence float64
}

type inferenceCandidate struct {
	value      string
	confidence float64
	rule       string
	quote      string
	quoteType  string
	support    map[string]bool
}

var textFactTypes = []string{"page_title", "search_title", "search_snippet", "page_description", "profile_description", "page_text", "bio"}

var textRules = []textRule{
	{
		name: "explicit-company-label-v2",
		re: regexp.MustCompile(`(?i)\b(?:company|employer|organi[sz]ation|workplace|perusahaan|instansi|kantor|` +
			`empresa|empleador|organizaci[oó]n|trabajo|entreprise|employeur|unternehmen|arbeitgeber|` +
			`organisatie|bedrijf|azienda|datore\s+di\s+lavoro|organizzazione|empregador|organiza[cç][aã]o|` +
			`local\s+de\s+trabalho)\s*[:\-\x{2013}\x{2014}]\s*([^|\n\r;]{2,120})`),
		confidence: 0.75,
	},
	{
		name:       "employment-phrase-v1",
		re:         regexp.MustCompile(`(?i)\b(?:works?|working|worked|employed|employment)\s+(?:at|for|with|di|pada)\s+([^|\n\r;]{2,100})`),
		confidence: 0.7,
	},
	{
		name:       "indonesian-employment-phrase-v1",
		re:         regexp.MustCompile(`(?i)\b(?:bekerja|berkarya)\s+(?:di|pada)\s+([^|\n\r;]{2,100})`),
		confidence: 0.7,
	},
	{
		name:       "international-employment-phrase-v1",
		re:         regexp.MustCompile(`(?i)\b(?:trabaja(?:ndo)?|travaille(?:r)?|arbeitet|trabalha|lavora|werkt)\s+(?:en|para|chez|pour|bei|für|fur|na|no|em|presso|per|bij|voor|a|\x{e0})\s+([^|\n\r;]{2,100})`),
		confidence: 0.7,
	},
	{
		name: "employer-label-v1",
		re: regexp.MustCompile(`(?i)\b(?:employer|pemberi\s+kerja|employeur|arbeitgeber|empleador|empregador|datore\s+di\s+lavoro)\b` +
			`\s*[:\-\x{2013}\x{2014}]?\s+([^|\n\r;]{2,100})`),
		confidence: 0.72,
	},
	{
		name: "role-organization-phrase-v2",
		re: regexp.MustCompile(`(?i)\b(?:head|director|manager|professor|lecturer|dosen|guru|teacher|staff|engineer|developer|analyst|consultant|researcher|scientist|architect|specialist|officer|intern|founder|co-founder|owner|ceo|cto|cfo|coo|president|dean|chair|lead|principal|partner|associate|assistant|coordinator|supervisor|administrator|trainer|instructor)\b` +
			`(?:\s+of\s+[^,.;!?|\n\r]{1,60}?\s+(?:at|in|for|with|di|pada)|\s+(?:at|for|with|di|pada))\s+([^|\n\r;]{2,100})`),
		confidence: 0.64,
	},
	{
		name: "organization-suffix-v2",
		re: regexp.MustCompile(`(?i)\b((?:[\p{L}\p{N}][\p{L}\p{N}&.'-]*\s+){0,4}(?:(?:consulting|advisory|technology|financial|professional)\s+(?:group|services|solutions)|` +
			`universitas|university|université|universidad|universität|hochschule|institut|institute|college|academy|akademi|sekolah|school|politeknik|` +
			`corporation|corp|incorporated|inc|limited|ltd|llc|llp|plc|gmbh|aktiengesellschaft|ag|s\.?\s?a\.?|sas|sarl|b\.?\s?v\.?|n\.?\s?v\.?|` +
			`s\.?\s?p\.?\s?a\.?|s\.?\s?r\.?\s?l\.?|ltda|limitada|k\.?\s?k\.?|co|kg|group|holdings|foundation|stiftung|fondation|fundación|fundação|` +
			`yayasan|bank|hospital|centre|center|laboratory|labs?|technologies|technology|solutions|systems|consulting|company|agency|ministry|department))\b`),
		confidence: 0.5,
	},
	{
		name: "organization-infix-v1",
		re: regexp.MustCompile(`(?i)\b((?:university|college|institute|academy|université|universidad|universität|hochschule)` +
			`\s+(?:of|de|del|de\s+la|du|des|di|da|do|der|für|for)\s+(?:[\p{L}\p{N}][\p{L}\p{N}&.'-]*\s*){1,4})`),
		confidence: 0.52,
	},
	{
		name: "organization-prefix-v2",
		re: regexp.MustCompile(`(?i)\b((?:universitas|institut|politeknik|yayasan|kementerian|ministry|dinas|badan|lembaga|department|agency|bureau|` +
			`pt|cv|ud|pd|perusahaan|bank|rumah\s+sakit|groupe|grupo|gruppe|société|societe|sociedad|empresa|companhia|compagnie)` +
			`\s+(?:[\p{L}\p{N}][\p{L}\p{N}&.'-]*\s*){1,5})`),
		confidence: 0.55,
	},
}

var (
	freeEmailDomains = map[string]bool{
		"163.com": true, "126.com": true, "aol.com": true, "bk.ru": true,
		"bol.com.br": true, "centrum.cz": true, "daum.net": true,
		"docomo.ne.jp": true, "ezweb.ne.jp": true, "free.fr": true,
		"gmail.com": true, "gmx.com": true, "gmx.de": true, "googlemail.com": true,
		"hanmail.net": true, "hey.com": true, "hotmail.com": true,
		"icloud.com": true, "ig.com.br": true, "inbox.ru": true,
		"interia.pl": true, "laposte.net": true, "libero.it": true,
		"list.ru": true, "live.com": true, "mac.com": true, "mail.com": true,
		"mail.ru": true, "me.com": true, "msn.com": true, "naver.com": true,
		"o2.pl": true, "onet.pl": true, "orange.fr": true, "outlook.com": true,
		"pm.me": true, "proton.me": true, "protonmail.com": true, "qq.com": true,
		"rambler.ru": true, "rediffmail.com": true, "seznam.cz": true,
		"sfr.fr": true, "sina.com": true, "softbank.ne.jp": true, "sohu.com": true,
		"t-online.de": true, "terra.com.br": true, "tin.it": true,
		"tutanota.com": true, "uol.com.br": true, "virgilio.it": true,
		"wanadoo.fr": true, "web.de": true, "wp.pl": true, "yahoo.co.id": true,
		"yahoo.co.jp": true, "yahoo.com": true, "yandex.com": true,
		"yandex.ru": true, "ymail.com": true, "zoho.com": true,
	}
	blockedHosts = map[string]bool{
		"about.me": true, "academia.edu": true, "beacons.ai": true, "bing.com": true,
		"bio.link": true, "bit.ly": true, "blogspot.com": true, "carrd.co": true,
		"coursera.org": true, "crunchbase.com": true, "discord.gg": true,
		"duckduckgo.com": true, "edx.org": true, "facebook.com": true,
		"fb.com": true, "firebaseapp.com": true, "github.com": true,
		"github.io": true, "gitlab.com": true, "glassdoor.com": true,
		"goodreads.com": true, "google.com": true, "herokuapp.com": true,
		"imdb.com": true, "indeed.com": true,
		"instagram.com": true, "linkedin.com": true, "linkin.bio": true,
		"linktr.ee": true, "medium.com": true, "netlify.app": true,
		"notion.site": true, "orcid.org": true, "pages.dev": true,
		"pinterest.com": true, "reddit.com": true, "researchgate.net": true,
		"scholar.google.com": true, "searx.be": true, "snapchat.com": true, "soundcloud.com": true,
		"spotify.com": true, "squarespace.com": true, "substack.com": true, "t.co": true,
		"t.me": true, "telegram.me": true, "threads.net": true, "tiktok.com": true,
		"tinyurl.com": true, "tumblr.com": true, "twitch.tv": true, "twitter.com": true,
		"udemy.com": true, "vercel.app": true, "wa.me": true, "web.app": true,
		"weebly.com": true, "wellfound.com": true, "wikibooks.org": true,
		"wikidata.org": true, "wikimedia.org": true, "wikinews.org": true,
		"wikipedia.org": true, "wikiquote.org": true, "wikisource.org": true,
		"wikiversity.org": true, "wikivoyage.org": true, "wiktionary.org": true,
		"wixsite.com": true, "wordpress.com": true, "x.com": true,
		"xing.com": true, "youtu.be": true, "youtube.com": true,
		"example.com": true, "example.org": true, "localhost": true,
	}
	genericDomainLabels = map[string]bool{
		"about": true, "account": true, "ac": true, "app": true, "blog": true,
		"careers": true, "co": true, "com": true, "contact": true, "dev": true,
		"docs": true, "edu": true, "email": true, "gov": true, "help": true,
		"home": true, "id": true, "io": true, "jobs": true, "login": true,
		"mail": true, "net": true, "news": true, "org": true, "people": true,
		"profile": true, "sch": true, "shop": true, "smtp": true,
		"staging": true, "store": true, "support": true, "team": true,
		"test": true, "webmail": true, "www": true,
	}
	genericOrganizationWords = map[string]bool{
		"a": true, "academy": true, "ag": true, "agency": true, "akademi": true,
		"aktiengesellschaft": true, "an": true, "and": true, "at": true,
		"b.v": true, "bank": true, "bureau": true, "bv": true, "centre": true,
		"center": true, "co": true, "college": true, "companhia": true,
		"company": true, "compagnie": true, "confidential": true, "consultant": true,
		"corporation": true, "corp": true, "department": true, "di": true,
		"ecole": true, "école": true, "empresa": true, "employed": true,
		"escuela": true, "faculdade": true, "facultad": true, "fondation": true,
		"foundation": true, "freelance": true, "freelancer": true, "fundação": true,
		"fundación": true, "gmbh": true, "group": true, "groupe": true,
		"grupo": true, "gruppe": true, "hochschule": true, "holding": true,
		"holdings": true, "hospital": true, "hybrid": true, "inc": true,
		"incorporated": true, "independent": true, "institut": true,
		"institute": true, "k.k": true, "kg": true, "kk": true, "labs": true,
		"laboratory": true, "limitada": true, "limited": true, "llc": true,
		"llp": true, "ltda": true, "ltd": true, "ministry": true, "n.v": true,
		"nv": true, "of": true, "office": true, "online": true, "onsite": true,
		"open": true, "pada": true, "plc": true, "politeknik": true, "pte": true,
		"pty": true, "remote": true, "s.a": true, "s.a.s": true, "s.a.r.l": true,
		"sa": true, "sarl": true, "sas": true, "school": true, "sekolah": true,
		"self": true, "sociedad": true, "société": true, "societe": true,
		"stiftung": true, "the": true, "to": true, "training": true,
		"universidad": true, "universitas": true, "universität": true,
		"université": true, "university": true, "various": true, "with": true,
		"work": true, "yayasan": true,
	}
	genericSingleOrganizationWords = map[string]bool{
		"amsterdam": true, "bandung": true, "bangkok": true, "barcelona": true,
		"berlin": true, "brussels": true, "confidential": true, "dubai": true,
		"dublin": true, "freelance": true, "freelancer": true, "geneva": true,
		"hybrid": true, "independent": true, "indonesia": true, "jakarta": true,
		"london": true, "madrid": true, "malaysia": true, "manila": true,
		"melbourne": true, "milan": true, "mumbai": true, "office": true,
		"online": true, "onsite": true, "paris": true, "remote": true,
		"seoul": true, "self-employed": true, "shanghai": true, "singapore": true,
		"surabaya": true, "sydney": true, "tokyo": true, "toronto": true,
		"training":   true,
		"unemployed": true, "various": true, "vienna": true, "yogyakarta": true,
		"zurich": true,
	}
	leadingStopWords = map[string]bool{
		"a": true, "administrator": true, "after": true, "all": true,
		"alumni": true, "also": true, "an": true, "analista": true,
		"analyste": true, "analyst": true, "and": true, "another": true,
		"are": true, "as": true, "became": true, "because": true,
		"become": true, "becomes": true, "been": true, "before": true,
		"being": true, "both": true, "but": true, "during": true,
		"each": true, "either": true, "had": true, "has": true,
		"have": true, "he": true, "her": true, "his": true, "how": true,
		"is": true, "it": true, "its": true, "later": true, "many": true,
		"most": true, "neither": true, "nor": true, "or": true, "other": true,
		"our": true, "she": true, "since": true, "so": true, "some": true,
		"such": true, "that": true, "the": true, "their": true,
		"then": true, "these": true, "they": true, "this": true,
		"those": true, "to": true, "until": true, "was": true, "we": true,
		"were": true, "when": true, "where": true, "which": true,
		"while": true, "who": true, "whom": true, "whose": true, "why": true,
		"yet":       true,
		"assistant": true, "associate": true, "associé": true, "associée": true,
		"at": true, "attended": true, "awareness": true, "certified": true,
		"cloud": true, "consultant": true, "consultor": true, "coordinator": true,
		"cyber": true, "data": true, "dati": true, "datos": true, "dados": true,
		"followers": true, "following": true,
		"desarrollador": true, "desenvolvedor": true, "developer": true,
		"development": true, "développement": true, "développeur": true,
		"directeur": true, "director": true, "directora": true, "directrice": true,
		"direttore": true, "dosen": true, "dozent": true, "educación": true,
		"educação": true, "educated": true, "education": true, "empleado": true,
		"empleada": true, "engenheiro": true, "engineer": true, "especialista": true,
		"ethical": true, "finance": true, "finanzas": true, "finanças": true,
		"fondateur": true, "fondatore": true, "fondatrice": true, "for": true,
		"founder": true, "freelance": true, "from": true, "fundador": true,
		"fundadora": true, "gerente": true, "gestión": true, "gestão": true,
		"gestor": true, "gestora": true, "gestion": true, "graduate": true,
		"graduated": true, "guru": true, "hacker": true, "head": true,
		"health": true, "home": true, "hybrid": true, "in": true,
		"information": true, "ingegnere": true, "ingeniero": true,
		"ingénieur": true, "instructor": true, "investigación": true,
		"investigador": true, "joined": true, "jefe": true, "jefa": true,
		"junior": true, "lead": true, "leader": true, "leaders": true,
		"lecturer": true, "management": true, "manager": true, "markas": true,
		"marketing": true,
		"meet":      true, "member": true, "nube": true, "nuvem": true, "nuvola": true,
		"of": true, "officer": true, "online": true, "operaciones": true,
		"operações": true, "operations": true, "owner": true,
		"partner": true, "penetration": true, "personal": true, "pesquisa": true,
		"pesquisador": true, "principal": true, "profesor": true, "profesora": true,
		"professeur": true, "professore": true, "profile": true, "program": true,
		"qualification": true, "receive": true, "received": true, "receives": true,
		"recherche": true, "remote": true, "research": true,
		"researcher": true, "ricerca": true, "ricercatore": true, "salud": true,
		"salute": true, "santé": true, "saúde": true, "science": true,
		"security": true, "seguridad": true, "segurança": true, "sécurité": true,
		"senior": true, "sicurezza": true, "software": true, "socio": true,
		"socia": true, "specialist": true, "specialista": true, "spécialiste": true,
		"staff": true, "student": true, "studied": true, "study": true,
		"studies": true, "supervisor": true, "sviluppatore": true, "teacher": true,
		"teaching": true, "tecnología": true, "tecnologia": true,
		"technologie": true, "technology": true, "tester": true,
		"trainer": true, "ventas": true, "visited": true, "with": true,
		"works": true, "wolke": true,
	}
	affiliationContextWords = []string{
		"alma mater", "almamater", "alumni", "alumna", "alumnus", "bachelor",
		"conferencia", "conference",
		"conferenza", "degree", "doctorate", "doctoral", "educación", "education",
		"educação", "estudiante", "estudia", "estudió", "étudiant", "formación",
		"formazione", "graduate", "graduated", "graduation", "guest", "konferenz",
		"kuliah", "laurea", "lecture", "licenciatura", "lulus", "mahasiswa",
		"master", "maestría", "palestra", "phd", "ponente", "seminario",
		"seminar", "séminaire", "speaker", "spoke", "student", "studentessa",
		"studente", "studied", "studies", "study", "studiert", "visit", "visited",
		"webinar", "workshop",
	}
	trailingOrgAffixes = map[string]bool{
		"academy": true, "ag": true, "akademi": true, "aktiengesellschaft": true,
		"b.v": true, "bv": true, "center": true, "centre": true, "co": true,
		"college": true, "companhia": true, "company": true, "compagnie": true,
		"corporation": true, "corp": true, "ecole": true, "école": true,
		"empresa": true, "escuela": true, "faculdade": true, "facultad": true,
		"fondation": true, "foundation": true, "fundação": true, "fundación": true,
		"gmbh": true, "group": true, "groupe": true, "grupo": true, "gruppe": true,
		"hochschule": true, "holding": true, "holdings": true, "hospital": true,
		"inc": true, "incorporated": true, "institut": true, "institute": true,
		"k.k": true, "kg": true, "kk": true, "laboratory": true, "labs": true,
		"limitada": true, "limited": true, "llc": true, "llp": true, "ltda": true,
		"ltd": true, "n.v": true, "nv": true, "plc": true, "pte": true,
		"pty": true, "s.a": true, "s.a.s": true, "s.a.r.l": true, "sa": true,
		"sarl": true, "sas": true, "school": true, "sekolah": true,
		"sociedad": true, "société": true, "societe": true, "stiftung": true,
		"universidad": true, "universitas": true, "universität": true,
		"université": true, "university": true, "yayasan": true,
	}
	organizationSignalWords = map[string]bool{
		"academy": true, "ag": true, "agency": true, "akademi": true,
		"aktiengesellschaft": true, "b.v": true, "bank": true, "bureau": true,
		"bv": true, "center": true, "centre": true, "co": true,
		"college": true, "companhia": true, "company": true, "compagnie": true,
		"consulting": true, "corp": true, "corporation": true, "cv": true,
		"department": true, "ecole": true, "école": true, "empresa": true,
		"escuela": true, "faculdade": true, "facultad": true, "fondation": true,
		"foundation": true, "fundação": true, "fundación": true, "gmbh": true,
		"group": true, "groupe": true, "grupo": true, "gruppe": true,
		"hochschule": true, "holding": true, "holdings": true, "hospital": true,
		"inc": true, "incorporated": true, "institut": true, "institute": true,
		"k.k": true, "kg": true, "kk": true, "lab": true, "laboratory": true,
		"labs": true, "limitada": true, "limited": true, "llc": true, "llp": true,
		"ltda": true, "ltd": true, "ministry": true, "n.v": true, "nv": true,
		"pd": true, "plc": true, "politeknik": true, "pte": true, "pt": true,
		"pty": true, "s.a": true, "s.a.s": true, "s.a.r.l": true, "sa": true,
		"sarl": true, "sas": true, "school": true, "sekolah": true,
		"sociedad": true, "société": true, "societe": true, "solutions": true,
		"stiftung": true, "systems": true, "technologies": true, "technology": true,
		"ud": true, "universidad": true, "universitas": true, "universität": true,
		"université": true, "university": true, "yayasan": true,
	}
	trailingStopWords = map[string]bool{
		"a": true, "an": true, "and": true, "at": true, "di": true,
		"for": true, "from": true, "in": true, "of": true, "pada": true,
		"the": true, "with": true,
	}
	connectorWords = map[string]bool{
		"and": true, "dan": true, "di": true, "for": true,
		"of": true, "the": true,
	}
	shortAcronyms = map[string]string{
		"ag": "AG", "b.v": "B.V.", "bca": "BCA", "bni": "BNI", "bri": "BRI",
		"bv": "BV", "co": "Co", "cv": "CV", "gmbh": "GmbH", "ibm": "IBM",
		"k.k": "K.K.", "kk": "K.K.", "llc": "LLC", "llp": "LLP",
		"ltda": "Ltda", "n.v": "N.V.", "nv": "NV", "pd": "PD", "plc": "PLC",
		"pt": "PT", "pte": "Pte", "pty": "Pty", "s.a": "S.A.",
		"s.a.s": "S.A.S.", "s.a.r.l": "S.A.R.L.", "sa": "SA", "sarl": "SARL",
		"sas": "SAS", "s.p.a": "S.p.A.", "s.r.l": "S.r.l.", "ud": "UD",
		"xl": "XL",
	}
	twoLevelSuffixes = map[string]bool{
		"ac.at": true, "ac.be": true, "ac.id": true, "ac.in": true, "ac.jp": true,
		"ac.kr": true, "ac.nz": true, "ac.th": true, "ac.uk": true, "ac.za": true,
		"biz.id": true, "co.id": true, "co.il": true, "co.in": true, "co.jp": true,
		"co.ke": true, "co.kr": true, "co.nz": true, "co.th": true, "co.uk": true,
		"co.za": true, "com.ar": true, "com.au": true, "com.br": true,
		"com.cn": true, "com.co": true, "com.eg": true, "com.hk": true,
		"com.mx": true, "com.my": true, "com.ng": true, "com.pe": true,
		"com.ph": true, "com.pk": true, "com.qa": true, "com.sa": true,
		"com.sg": true, "com.tr": true, "com.ua": true, "com.vn": true,
		"edu.au": true, "edu.br": true, "edu.cn": true, "edu.hk": true,
		"edu.my": true, "edu.pe": true, "edu.ph": true, "edu.pk": true,
		"edu.sg": true, "edu.tw": true, "edu.vn": true, "go.id": true,
		"go.jp": true, "go.th": true, "gov.br": true, "gov.in": true,
		"gov.uk": true, "gov.za": true, "in.th": true, "mil.id": true,
		"my.id": true, "net.au": true, "net.id": true, "net.za": true,
		"or.id": true, "or.ke": true, "org.au": true, "org.br": true,
		"org.id": true, "org.il": true, "org.ng": true, "org.uk": true,
		"org.za": true, "sch.id": true, "sch.uk": true, "web.id": true,
	}
	rulePriority = map[string]int{
		"explicit-company-label-v2":          6,
		"employment-phrase-v1":               5,
		"indonesian-employment-phrase-v1":    5,
		"international-employment-phrase-v1": 5,
		"employer-label-v1":                  5,
		"role-organization-phrase-v2":        4,
		"organization-prefix-v2":             3,
		"organization-infix-v1":              3,
		"organization-suffix-v2":             2,
		"email-domain-v1":                    1,
		"profile-domain-v1":                  0,
	}
)

func ExtractCompany(in Input) (Inference, bool) {
	nameWords := words(in.Name)
	available := map[string]string{}
	for typ, value := range in.Facts {
		if strings.TrimSpace(value) != "" {
			available[typ] = value
		}
	}
	if strings.TrimSpace(in.Email) != "" {
		available["email"] = in.Email
	}
	facts := map[string]string{}
	combined := []string{}
	for _, typ := range textFactTypes {
		value := strings.TrimSpace(in.Facts[typ])
		if value == "" {
			continue
		}
		facts[typ] = value
		combined = append(combined, value)
	}
	if name := strings.TrimSpace(available["name"]); name != "" {
		combined = append(combined, name)
	}
	relevant := nameRelevant(nameWords, strings.Join(combined, " "))
	candidates := map[string]*inferenceCandidate{}
	domainTokens := map[string]bool{}

	add := func(raw, rule string, confidence float64, support, quote, context string) {
		value := normalizeOrganization(raw, in.Name)
		if value == "" || !organizationLike(value, raw, in.Name) {
			return
		}
		bareOrganizationRule := rule == "organization-suffix-v2" || rule == "organization-prefix-v2" || rule == "organization-infix-v1"
		if bareOrganizationRule && (support == "page_title" || (support == "page_text" && awardContext(context))) {
			return
		}
		if bareOrganizationRule && educationAffiliation(context) {
			return
		}
		if (rule == "organization-suffix-v2" || rule == "organization-infix-v1") && organizationSpanHasContextPreposition(value) {
			return
		}
		if bareOrganizationRule && organizationSpanHasProseMarker(value) {
			return
		}
		if rule == "role-organization-phrase-v2" && !organizationTargetStartsProper(raw) {
			return
		}
		key := strings.ToLower(value)
		candidate, ok := candidates[key]
		if !ok {
			candidate = &inferenceCandidate{value: value, confidence: confidence, rule: rule, quote: auditQuote(quote, value, support != "email" && support != "indexed_url"), quoteType: support, support: map[string]bool{}}
			candidates[key] = candidate
		}
		candidate.support[support] = true
		if confidence > candidate.confidence || (confidence == candidate.confidence && rulePriority[rule] > rulePriority[candidate.rule]) {
			candidate.confidence = confidence
			candidate.rule = rule
			candidate.quote = auditQuote(quote, value, support != "email" && support != "indexed_url")
			candidate.quoteType = support
		}
	}

	if relevant {
		for _, typ := range textFactTypes {
			value := facts[typ]
			if value == "" {
				continue
			}
			for _, rule := range textRules {
				for _, match := range rule.re.FindAllStringSubmatchIndex(value, -1) {
					if len(match) < 4 {
						continue
					}
					raw := value[match[2]:match[3]]
					add(raw, rule.name, rule.confidence, typ, raw, sentenceContext(value, match[0], match[1]))
				}
			}
		}
	}

	if host, ok := emailHost(in.Email); ok {
		if !freeEmailHost(host) {
			if label, ok := registrableLabel(host); ok {
				for _, token := range words(strings.ReplaceAll(label, "-", " ")) {
					domainTokens[token] = true
				}
				if value := domainOrganization(label, in.Name); value != "" {
					add(value, "email-domain-v1", 0.4, "email", in.Email, "")
				}
			}
		}
	}

	if host, ok := urlHost(in.URL); ok {
		if label, ok := registrableLabel(host); ok {
			for _, token := range words(strings.ReplaceAll(label, "-", " ")) {
				domainTokens[token] = true
			}
			if relevant && profileLikeURLPath(in.URL) {
				if value := domainOrganization(label, in.Name); value != "" {
					add(value, "profile-domain-v1", 0.35, "indexed_url", in.URL, "")
				}
			}
		}
	}

	type rankedCandidate struct {
		candidate *inferenceCandidate
		support   []string
	}
	ranked := []rankedCandidate{}
	for _, candidate := range candidates {
		if candidateMatchesDomain(candidate.value, domainTokens) {
			candidate.confidence += 0.08
		}
		if candidate.confidence > 1 {
			candidate.confidence = 1
		}
		support := usableSupport(candidate.support, available)
		if len(support) == 0 {
			continue
		}
		ranked = append(ranked, rankedCandidate{candidate: candidate, support: support})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return candidateBetter(ranked[i].candidate, ranked[j].candidate)
	})
	if len(ranked) == 0 || ranked[0].candidate.confidence < 0.3 {
		return Inference{}, false
	}
	best := ranked[0].candidate
	alternatives := []Alternative{}
	ambiguous := false
	for _, item := range ranked[1:] {
		if strings.EqualFold(item.candidate.value, best.value) {
			continue
		}
		if len(alternatives) < 3 {
			alternatives = append(alternatives, Alternative{Value: item.candidate.value, Rule: item.candidate.rule, Confidence: item.candidate.confidence})
		}
		if (item.candidate.confidence >= best.confidence-0.08 || domainRule(best.rule) != domainRule(item.candidate.rule)) && !relatedOrganization(item.candidate.value, best.value) {
			ambiguous = true
		}
	}
	return Inference{
		Value:           best.value,
		Confidence:      best.confidence,
		Rule:            best.rule,
		Quote:           best.quote,
		QuoteType:       best.quoteType,
		SupportingTypes: ranked[0].support,
		Alternatives:    alternatives,
		Ambiguous:       ambiguous,
	}, true
}

func candidateBetter(a, b *inferenceCandidate) bool {
	if a.confidence != b.confidence {
		return a.confidence > b.confidence
	}
	if rulePriority[a.rule] != rulePriority[b.rule] {
		return rulePriority[a.rule] > rulePriority[b.rule]
	}
	if len(a.value) != len(b.value) {
		return len(a.value) > len(b.value)
	}
	return a.value < b.value
}

func domainRule(rule string) bool {
	return rule == "email-domain-v1" || rule == "profile-domain-v1"
}

func relatedOrganization(a, b string) bool {
	aWords, bWords := words(a), words(b)
	if len(aWords) == 0 || len(bWords) == 0 {
		return false
	}
	if wordSubset(aWords, bWords) || wordSubset(bWords, aWords) {
		return true
	}
	aCompact, bCompact := strings.Join(aWords, ""), strings.Join(bWords, "")
	if strings.Contains(aCompact, bCompact) || strings.Contains(bCompact, aCompact) {
		return true
	}
	aMeaningful, bMeaningful := meaningfulOrgTokens(aWords), meaningfulOrgTokens(bWords)
	if len(aMeaningful) == 1 && len(bMeaningful) > 1 {
		return prefixAcronym(aMeaningful[0], bMeaningful) || initialAcronym(aMeaningful[0], bMeaningful)
	}
	if len(bMeaningful) == 1 && len(aMeaningful) > 1 {
		return prefixAcronym(bMeaningful[0], aMeaningful) || initialAcronym(bMeaningful[0], aMeaningful)
	}
	return false
}

func meaningfulOrgTokens(tokens []string) []string {
	kept := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if !connectorWords[token] {
			kept = append(kept, token)
		}
	}
	for len(kept) > 0 && trailingOrgAffixes[kept[len(kept)-1]] {
		kept = kept[:len(kept)-1]
	}
	return kept
}

func prefixAcronym(short string, parts []string) bool {
	if short == "" {
		return len(parts) == 0
	}
	if len(parts) == 0 {
		return false
	}
	first := parts[0]
	max := len(first)
	if max > 4 {
		max = 4
	}
	for length := 2; length <= max && length <= len(short); length++ {
		if strings.HasPrefix(short, first[:length]) && prefixAcronym(short[length:], parts[1:]) {
			return true
		}
	}
	return false
}

func initialAcronym(short string, parts []string) bool {
	if len(short) != len(parts) {
		return false
	}
	for index, part := range parts {
		if part == "" || short[index] != part[0] {
			return false
		}
	}
	return true
}

func wordSubset(a, b []string) bool {
	set := map[string]bool{}
	for _, word := range b {
		set[word] = true
	}
	for _, word := range a {
		if !set[word] {
			return false
		}
	}
	return true
}

func auditQuote(raw, value string, focus bool) string {
	raw = strings.Join(strings.Fields(raw), " ")
	if focus && value != "" {
		lowerRaw, lowerValue := strings.ToLower(raw), strings.ToLower(value)
		if index := strings.Index(lowerRaw, lowerValue); index >= 0 {
			raw = raw[index : index+len(lowerValue)]
		}
	}
	runes := []rune(raw)
	if len(runes) > 240 {
		return string(runes[:240])
	}
	return raw
}

func usableSupport(support map[string]bool, facts map[string]string) []string {
	out := []string{}
	for _, typ := range textFactTypes {
		if support[typ] && facts[typ] != "" {
			out = append(out, typ)
		}
	}
	if support["email"] && facts["email"] != "" {
		out = append(out, "email")
	}
	if support["indexed_url"] && facts["indexed_url"] != "" {
		out = append(out, "indexed_url")
	}
	return out
}

func candidateMatchesDomain(value string, domainTokens map[string]bool) bool {
	for _, token := range words(value) {
		if domainTokens[token] {
			return true
		}
	}
	return false
}

func nameRelevant(nameWords []string, text string) bool {
	if len(nameWords) == 0 {
		return false
	}
	haystack := " " + strings.Join(words(text), " ") + " "
	for _, word := range nameWords {
		if !strings.Contains(haystack, " "+word+" ") {
			return false
		}
	}
	return true
}

func organizationLike(value, raw, name string) bool {
	valueWords := strings.Fields(value)
	if len(valueWords) == 0 || len(valueWords) > 8 || len([]rune(value)) > 100 {
		return false
	}
	if !hasUpper(raw) {
		return false
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "http") || strings.ContainsAny(value, "/\\@<>") {
		return false
	}
	if strings.EqualFold(strings.Join(words(value), " "), strings.Join(words(name), " ")) {
		return false
	}
	if len(valueWords) == 1 && genericSingleOrganizationWords[strings.ToLower(strings.Trim(valueWords[0], ".,&"))] {
		return false
	}
	allGeneric := true
	for _, word := range valueWords {
		if !genericOrganizationWords[strings.ToLower(strings.Trim(word, ".,&"))] {
			allGeneric = false
			break
		}
	}
	if allGeneric {
		return false
	}
	for _, word := range valueWords {
		lowerWord := strings.ToLower(word)
		if strings.HasPrefix(lowerWord, "http") || strings.HasPrefix(lowerWord, "www.") {
			return false
		}
	}
	return true
}

func organizationSpanHasContextPreposition(value string) bool {
	for _, word := range words(value) {
		switch word {
		case "at", "in", "on", "with", "from", "by", "to":
			return true
		}
	}
	return false
}

func organizationSpanHasProseMarker(value string) bool {
	if organizationSpanStartsWithYear(value) {
		return true
	}
	seen := map[string]bool{}
	previous := ""
	for _, word := range words(value) {
		switch word {
		case "and", "atau", "dan", "or", "serta", "with":
			return true
		case "article", "articles", "award", "book", "books", "buku", "chapter",
			"conference", "course", "curriculum", "dissertation", "journal",
			"makalah", "medal", "medali", "paper", "papers", "penghargaan",
			"penerima", "prize", "proceedings", "thesis":
			return true
		}
		if word == previous {
			return true
		}
		if seen[word] && len([]rune(word)) > 3 {
			return true
		}
		seen[word] = true
		previous = word
	}
	return false
}

func awardContext(context string) bool {
	if context == "" {
		return false
	}
	haystack := " " + strings.Join(words(context), " ") + " "
	for _, cue := range []string{
		"award", "awarded", "beasiswa", "fellowship", "laureate", "medal",
		"medali", "penghargaan", "prize", "scholarship", "winner",
	} {
		if strings.Contains(haystack, " "+cue+" ") {
			return true
		}
	}
	return false
}

func organizationSpanStartsWithYear(value string) bool {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return false
	}
	year := strings.Trim(fields[0], ".,")
	if len(year) != 4 {
		return false
	}
	for _, r := range year {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return year >= "1900" && year <= "2099"
}

func organizationTargetStartsProper(raw string) bool {
	value := strings.TrimSpace(raw)
	value = strings.Trim(value, "\"'`\u201c\u201d\u2018\u2019()[]{}<>")
	fields := strings.Fields(value)
	for len(fields) > 0 {
		switch strings.ToLower(strings.Trim(fields[0], ".,")) {
		case "a", "an", "sebuah", "seorang", "the":
			fields = fields[1:]
			continue
		}
		break
	}
	if len(fields) == 0 {
		return false
	}
	first := []rune(fields[0])[0]
	return (unicode.IsLetter(first) && !unicode.IsLower(first)) || unicode.IsDigit(first)
}

func normalizeOrganization(raw, name string) string {
	value := strings.TrimSpace(raw)
	value = strings.Trim(value, "\"'`\u201c\u201d\u2018\u2019()[]{}<>")
	value = cutAny(value, []string{"\n", "\r", "|", ";", " \u2022 "})
	for _, separator := range []string{" - ", " \u2013 ", " \u2014 "} {
		if index := strings.Index(value, separator); index > 0 {
			value = value[:index]
		}
	}
	if index := strings.Index(value, ","); index > 0 {
		value = value[:index]
	}
	value = cutFieldLabel(value)
	value = cutSentence(value)
	value = strings.Join(strings.Fields(value), " ")
	value = strings.Trim(value, " .,:;!?*")
	value = trimLeadingName(value, name)
	value = trimSocialCounts(value)
	value = trimStopWords(value, leadingStopWords, false)
	value = trimStopWords(value, trailingStopWords, true)
	return displayOrganization(value)
}

func sentenceContext(value string, start, end int) string {
	left := start
	for left > 0 && !strings.ContainsRune(".?!\n\r;|", rune(value[left-1])) {
		left--
	}
	right := end
	for right < len(value) && !strings.ContainsRune(".?!\n\r;|", rune(value[right])) {
		right++
	}
	return value[left:right]
}

func educationAffiliation(context string) bool {
	if context == "" {
		return false
	}
	haystack := " " + strings.Join(words(context), " ") + " "
	for _, cue := range affiliationContextWords {
		if strings.Contains(haystack, " "+cue+" ") {
			return true
		}
	}
	return false
}

func cutAny(value string, separators []string) string {
	for _, separator := range separators {
		if index := strings.Index(value, separator); index >= 0 {
			value = value[:index]
		}
	}
	return value
}

func cutFieldLabel(value string) string {
	lower := strings.ToLower(value)
	for _, label := range []string{
		" location", " industry", " website", " email", " phone", " address",
		" description", " about", " employees", " followers", " employer",
		" occupation", " education", " known", " born", " spouse", " children",
		" parents", " citizenship", " relatives", " signature", " pekerjaan",
		" pemberi kerja", " pendidikan", " dikenal", " lahir", " pasangan",
		" orang tua", " kewarganegaraan", " kerabat", " tanda tangan",
		" almamater", " alma mater",
	} {
		if index := strings.Index(lower, label); index > 0 {
			value = value[:index]
			lower = lower[:index]
		}
	}
	return value
}

func cutSentence(value string) string {
	segments := []string{}
	start := 0
	for index := 0; index+1 < len(value); index++ {
		if (value[index] == '.' || value[index] == '?' || value[index] == '!') && unicode.IsSpace(rune(value[index+1])) {
			segments = append(segments, strings.TrimSpace(value[start:index]))
			start = index + 2
		}
	}
	segments = append(segments, strings.TrimSpace(value[start:]))
	if len(segments) < 2 {
		return value
	}
	for index := len(segments) - 1; index >= 0; index-- {
		if containsOrganizationSignal(segments[index]) {
			return segments[index]
		}
	}
	return segments[0]
}

func containsOrganizationSignal(value string) bool {
	for _, word := range words(value) {
		if organizationSignalWords[word] {
			return true
		}
	}
	return false
}

func trimLeadingName(value, name string) string {
	tokens := strings.Fields(value)
	nameWords := words(name)
	if len(nameWords) == 0 || len(tokens) < len(nameWords) {
		return value
	}
	for i, word := range nameWords {
		if strings.ToLower(strings.Trim(tokens[i], ".,")) != word {
			return value
		}
	}
	return strings.Join(tokens[len(nameWords):], " ")
}

func trimSocialCounts(value string) string {
	tokens := strings.Fields(value)
	for len(tokens) >= 2 {
		count := strings.ToLower(strings.Trim(tokens[0], ".,"))
		label := strings.ToLower(strings.Trim(tokens[1], ".,"))
		numeric := true
		for _, r := range count {
			if !unicode.IsDigit(r) && r != 'k' && r != 'm' {
				numeric = false
				break
			}
		}
		if !numeric || (label != "follower" && label != "followers" && label != "following") {
			break
		}
		tokens = tokens[2:]
	}
	return strings.Join(tokens, " ")
}

func trimStopWords(value string, stopWords map[string]bool, trailing bool) string {
	tokens := strings.Fields(value)
	if trailing {
		for len(tokens) > 0 && stopWords[strings.ToLower(strings.Trim(tokens[len(tokens)-1], ".,"))] {
			tokens = tokens[:len(tokens)-1]
		}
	} else {
		for len(tokens) > 0 && stopWords[strings.ToLower(strings.Trim(tokens[0], ".,"))] {
			tokens = tokens[1:]
		}
	}
	return strings.Join(tokens, " ")
}

func displayOrganization(value string) string {
	tokens := strings.Fields(value)
	for i, token := range tokens {
		lower := strings.ToLower(strings.Trim(token, ".,"))
		if i > 0 && connectorWords[lower] {
			tokens[i] = lower
			continue
		}
		if acronym, ok := shortAcronyms[lower]; ok {
			tokens[i] = acronym
			continue
		}
		if isAllUpper(token) && len([]rune(token)) > 4 {
			tokens[i] = titleWord(token)
			continue
		}
		if isAllLower(token) {
			tokens[i] = titleWord(token)
		}
	}
	return strings.Join(tokens, " ")
}

func titleWord(word string) string {
	runes := []rune(strings.ToLower(word))
	if len(runes) == 0 {
		return word
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func isAllUpper(value string) bool {
	hasLetter := false
	for _, r := range value {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsUpper(r) {
				return false
			}
		}
	}
	return hasLetter
}

func isAllLower(value string) bool {
	hasLetter := false
	for _, r := range value {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsLower(r) {
				return false
			}
		}
	}
	return hasLetter
}

func hasUpper(value string) bool {
	for _, r := range value {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func words(value string) []string {
	return strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func emailHost(value string) (string, bool) {
	address, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil {
		return "", false
	}
	at := strings.LastIndex(address.Address, "@")
	if at < 1 || at == len(address.Address)-1 {
		return "", false
	}
	return strings.ToLower(address.Address[at+1:]), true
}

func freeEmailHost(host string) bool {
	for domain := range freeEmailDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func profileLikeURLPath(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	segments := strings.FieldsFunc(strings.ToLower(parsed.Path), func(r rune) bool {
		return r == '/'
	})
	for index, segment := range segments {
		switch segment {
		case "author", "authors", "employee", "employees", "faculty", "in", "member",
			"members", "people", "profile", "profiles", "staff", "team", "user", "users":
			return index <= 1
		}
	}
	return false
}

func urlHost(value string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return "", false
	}
	return strings.ToLower(parsed.Hostname()), true
}

func registrableLabel(host string) (string, bool) {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" || strings.Contains(host, ":") {
		return "", false
	}
	for blocked := range blockedHosts {
		if host == blocked || strings.HasSuffix(host, "."+blocked) {
			return "", false
		}
	}
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return "", false
	}
	suffixLength := 1
	if len(parts) >= 3 && twoLevelSuffixes[parts[len(parts)-2]+"."+parts[len(parts)-1]] {
		suffixLength = 2
	}
	index := len(parts) - suffixLength - 1
	if index < 0 {
		return "", false
	}
	label := parts[index]
	if len(label) < 3 || genericDomainLabels[label] {
		return "", false
	}
	for _, r := range label {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return "", false
		}
	}
	if strings.Trim(label, "-_0123456789") == "" {
		return "", false
	}
	return label, true
}

func domainOrganization(label, name string) string {
	value := displayOrganization(strings.Join(strings.FieldsFunc(label, func(r rune) bool {
		return r == '-' || r == '_'
	}), " "))
	if value == "" || genericDomainLabels[strings.ToLower(value)] {
		return ""
	}
	compact := strings.ReplaceAll(strings.Join(words(value), " "), " ", "")
	for _, nameWord := range words(name) {
		if len(nameWord) > 3 && strings.Contains(compact, nameWord) {
			return ""
		}
	}
	if !organizationLike(value, value, name) {
		return ""
	}
	return value
}
