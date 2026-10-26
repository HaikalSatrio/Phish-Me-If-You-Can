package extract

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var roleWords = map[string]bool{
	"chef": true, "restaurateur": true, "historian": true, "author": true, "poet": true,
	"presenter": true, "producer": true, "dancer": true, "player": true,
	"streamer": true, "youtuber": true, "influencer": true, "rapper": true,
	"singer": true, "songwriter": true, "musician": true, "comedian": true,
	"actor": true, "actress": true, "penyanyi": true, "musisi": true,
	"komedian": true, "pelawak": true, "aktor": true, "aktris": true,
	"singer-songwriter": true, "penyanyi-penulis": true, "wrestler": true, "pegulat": true, "selebritas": true,
	"footballer": true, "athlete": true, "atlet": true, "pemain": true, "pesepakbola": true,
	"activist": true, "aktivis": true, "advocate": true, "blogger": true, "bloger": true, "entrepreneur": true, "mentor": true, "pegiat": true, "fellow": true,
	"accountant": true, "administrator": true, "advisor": true, "adviser": true,
	"akuntan": true, "analis": true, "analyst": true, "apoteker": true,
	"architect": true, "arsitek": true, "assistant": true, "asisten": true,
	"associate": true, "bendahara": true, "ceo": true, "cfo": true,
	"chair": true, "chairman": true, "chairwoman": true, "chief": true,
	"chro": true, "cio": true, "cmo": true, "cofounder": true,
	"consultant": true, "coordinator": true, "coo": true, "co-founder": true,
	"cto": true, "dean": true, "dentist": true,
	"designer": true, "desainer": true, "developer": true, "director": true,
	"direktur": true, "doctor": true, "dokter": true, "dosen": true,
	"editor": true, "engineer": true, "executive": true, "founder": true,
	"graduate": true, "guru": true, "head": true, "ilmuwan": true,
	"instructor": true, "instruktur": true, "intern": true, "kepala": true,
	"ketua": true, "konsultan": true, "koordinator": true, "lawyer": true,
	"lead": true, "lecturer": true, "magang": true, "mahasiswa": true,
	"mahasiswi": true, "manager": true, "manajer": true, "mitra": true,
	"nurse": true, "officer": true, "operator": true, "owner": true,
	"partner": true, "pelatih": true, "pemilik": true, "pengacara": true,
	"pengembang": true, "peneliti": true, "penulis": true, "perawat": true,
	"petugas": true, "pharmacist": true, "physician": true, "postgraduate": true,
	"president": true, "principal": true, "profesor": true, "professor": true,
	"programmer": true, "rekayasawan": true, "researcher": true,
	"scientist": true, "secretary": true, "sekretaris": true, "specialist": true,
	"spesialis": true, "staff": true, "student": true, "supervisor": true,
	"teacher": true, "technician": true, "technoking": true, "tester": true,
	"trainer": true, "treasurer": true, "undergraduate": true,
	"vice": true, "wakil": true, "writer": true,
}

var leadershipRoleWords = map[string]bool{
	"bendahara": true, "ceo": true, "cfo": true, "chair": true, "chairman": true,
	"chairwoman": true, "chief": true, "chro": true, "cio": true, "cmo": true,
	"cofounder": true, "coo": true, "co-founder": true, "cto": true, "direktur": true,
	"founder": true, "kepala": true, "ketua": true, "manajer": true, "owner": true,
	"pendiri": true, "pemilik": true, "president": true, "secretary": true,
	"sekretaris": true, "technoking": true, "treasurer": true, "vice": true,
	"wakil": true,
}

var roleOrganizationPrefixes = []string{
	"association", "badan", "bem", "campus", "club", "community", "department",
	"faculty", "hima", "himti", "himpunan", "institut", "institute", "kampus",
	"komunitas", "lembaga", "organization", "organisasi", "perhimpunan", "program studi",
	"school", "sekolah", "society", "study program", "universitas", "university",
}

var roleRulePriority = map[string]int{
	"role-label-v1":       5,
	"role-head-v1":        3,
	"role-is-v1":          4,
	"role-as-v1":          4,
	"role-id-as-v1":       4,
	"role-headline-of-v1": 4,
	"role-headline-at-v1": 3,
	"role-title-v1":       2,
	"role-headline-v1":    1,
}

var roleLabelPattern = regexp.MustCompile(`(?i)\b(?:job\s+title|role|position|title|occupation|pekerjaan|jabatan|posisi|profesi)\b\s*[:\-\x{2013}\x{2014}]?\s+([^|\n\r;]{2,90})`)

var roleAsPattern = regexp.MustCompile(`(?i)\b(?:works?|working|worked|serves?|serving|employed)\s+as\s+(?:an?\s+|the\s+)?([^|\n\r;.]{2,80})`)

var roleIDAsPattern = regexp.MustCompile(`(?i)\b(?:bekerja|menjabat|bertugas|berprofesi)\s+sebagai\s+(?:seorang\s+)?([^|\n\r;.]{2,80})`)

var roleRejectionPhrases = []string{" not ", " no longer ", " former ", " formerly ", " previously ", " bukan ", " tidak lagi ", " mantan "}

type roleCandidate struct {
	value      string
	confidence float64
	rule       string
	quote      string
	quoteType  string
	support    map[string]bool
}

func ExtractRole(in Input) (Inference, bool) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Inference{}, false
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
	if !nameRelevant(words(name), strings.Join(combined, " ")) {
		return Inference{}, false
	}

	candidates := map[string]*roleCandidate{}
	add := func(raw, rule string, confidence float64, support, quote string) {
		value := normalizeRole(raw)
		if value == "" || !roleLike(value) || roleRelatedToName(value, name) || roleLooksLikeOrganization(value) {
			return
		}
		value = canonicalEnglishRole(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		candidate, ok := candidates[key]
		if !ok {
			candidate = &roleCandidate{value: value, confidence: confidence, rule: rule, quote: quote, quoteType: support, support: map[string]bool{}}
			candidates[key] = candidate
		}
		candidate.support[support] = true
		if confidence > candidate.confidence || (confidence == candidate.confidence && roleRulePriority[rule] > roleRulePriority[candidate.rule]) {
			candidate.confidence = confidence
			candidate.rule = rule
			candidate.quote = quote
			candidate.quoteType = support
		}
	}

	for _, typ := range textFactTypes {
		value := facts[typ]
		if value == "" {
			continue
		}
		if (typ == "search_title" || typ == "page_title") && roleTitleContextRejected(value) {
			continue
		}
		for _, match := range roleLabelPattern.FindAllStringSubmatchIndex(value, -1) {
			if len(match) < 4 {
				continue
			}
			raw := value[match[2]:match[3]]
			add(raw, "role-label-v1", 0.8, typ, raw)
		}
		for _, match := range roleAsPattern.FindAllStringSubmatchIndex(value, -1) {
			if len(match) < 4 {
				continue
			}
			raw := value[match[2]:match[3]]
			add(raw, "role-as-v1", 0.72, typ, raw)
		}
		for _, match := range roleIDAsPattern.FindAllStringSubmatchIndex(value, -1) {
			if len(match) < 4 {
				continue
			}
			raw := value[match[2]:match[3]]
			add(raw, "role-id-as-v1", 0.72, typ, raw)
		}
		for _, raw := range roleHeadMatches(name, value) {
			add(raw, "role-head-v1", 0.7, typ, raw)
		}
		for _, match := range roleIsMatches(name, value) {
			add(match, "role-is-v1", 0.72, typ, match)
		}
		if (typ == "search_title" || typ == "page_title") && nameRelevant(words(name), value) {
			for _, raw := range roleTitleSegments(name, value) {
				add(raw, "role-title-v1", 0.55, typ, raw)
			}
		}
		if (typ == "search_snippet" || typ == "search_title" || typ == "page_title" || typ == "profile_description") &&
			strings.Contains(value, "|") && nameRelevant(words(name), value) {
			for _, match := range roleHeadlineMatches(name, value) {
				add(match.raw, match.rule, match.confidence, typ, match.raw)
			}
		}
	}

	ranked := make([]*roleCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.confidence < 0.5 {
			continue
		}
		ranked = append(ranked, candidate)
	}
	sort.Slice(ranked, func(i, j int) bool {
		return roleCandidateBetter(ranked[i], ranked[j])
	})
	if len(ranked) == 0 {
		return Inference{}, false
	}
	best := ranked[0]
	alternatives := []Alternative{}
	for _, item := range ranked[1:] {
		if strings.EqualFold(item.value, best.value) || len(alternatives) >= 3 {
			continue
		}
		alternatives = append(alternatives, Alternative{Value: item.value, Rule: item.rule, Confidence: item.confidence})
	}
	support := []string{}
	for typ := range best.support {
		support = append(support, typ)
	}
	sort.Strings(support)
	return Inference{
		Value:           best.value,
		Confidence:      best.confidence,
		Rule:            best.rule,
		Quote:           best.quote,
		QuoteType:       best.quoteType,
		SupportingTypes: support,
		Alternatives:    alternatives,
	}, true
}

func roleCandidateBetter(a, b *roleCandidate) bool {
	if a.confidence != b.confidence {
		return a.confidence > b.confidence
	}
	if roleRulePriority[a.rule] != roleRulePriority[b.rule] {
		return roleRulePriority[a.rule] > roleRulePriority[b.rule]
	}
	if len(a.value) != len(b.value) {
		return len(a.value) > len(b.value)
	}
	return a.value < b.value
}

func roleHeadMatches(name, text string) []string {
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(name) + `\b([^|\n\r]{0,140}?)[.]?\s+((?:head|chief|director|manager|president|vice\s+president|dean|chair(?:man|woman)?|lead|principal|coordinator|supervisor|founder|co-?founder|owner|ceo|cto|cfo|coo|cio|cmo|chro|treasurer|general\s+treasurer|secretary|technoking)\b[^|.;\n\r]{0,90})`)
	matches := pattern.FindAllStringSubmatch(text, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 || roleContextRejected(match[1]) {
			continue
		}
		out = append(out, match[2])
	}
	return out
}

func roleIsMatches(name, text string) []string {
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(name) + `\b[^.;!?|\n\r]{0,40}?\s+(?:is|adalah|merupakan)\s+(?:a\s+|an\s+|the\s+|seorang\s+)?([^.;!?|\n\r]{2,80})`)
	matches := pattern.FindAllStringSubmatch(text, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			out = append(out, match[1])
		}
	}
	return out
}

func roleTitleSegments(name, value string) []string {
	segments := []string{}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		return r == '|'
	}) {
		for _, segment := range strings.FieldsFunc(part, func(r rune) bool {
			return r == '\u2013' || r == '\u2014'
		}) {
			segments = append(segments, strings.Split(segment, " - ")...)
		}
	}
	out := []string{}
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" || roleRelatedToName(segment, name) {
			continue
		}
		out = append(out, segment)
	}
	return out
}

type roleHeadlineMatch struct {
	raw        string
	rule       string
	confidence float64
}

func roleHeadlineMatches(name, value string) []roleHeadlineMatch {
	out := []roleHeadlineMatch{}
	for _, segment := range strings.Split(value, "|") {
		segment = strings.TrimSpace(segment)
		if segment == "" || len([]rune(segment)) > 180 {
			continue
		}
		lower := strings.ToLower(segment)
		if index := strings.Index(lower, " of "); index > 0 {
			left := strings.TrimSpace(trimLeadingName(segment[:index], name))
			right := strings.TrimSpace(segment[index+4:])
			if leadershipRolePhrase(left) && organizationTargetStartsProper(right) && roleLike(normalizeRole(left)) {
				out = append(out, roleHeadlineMatch{raw: left, rule: "role-headline-of-v1", confidence: 0.7})
			}
		}
		if index := strings.Index(lower, " at "); index > 0 {
			left := strings.TrimSpace(trimLeadingName(segment[:index], name))
			if roleLike(normalizeRole(left)) {
				out = append(out, roleHeadlineMatch{raw: left, rule: "role-headline-at-v1", confidence: 0.64})
			}
		}
		bare := strings.TrimSpace(trimLeadingName(segment, name))
		if roleLike(normalizeRole(bare)) {
			out = append(out, roleHeadlineMatch{raw: bare, rule: "role-headline-v1", confidence: 0.52})
		}
	}
	return out
}

func leadershipRolePhrase(value string) bool {
	for _, word := range words(value) {
		if leadershipRoleWords[word] {
			return true
		}
	}
	return false
}

func roleLooksLikeOrganization(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	for _, prefix := range roleOrganizationPrefixes {
		if lower == prefix || strings.HasPrefix(lower, prefix+" ") {
			return true
		}
	}
	return false
}

func normalizeRole(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.Trim(value, "\"'`\u201c\u201d\u2018\u2019()[]{}<>")
	value = cutAny(value, []string{"\n", "\r", "|", ";", " \u2022 "})
	for _, separator := range []string{". ", "?", "!", " - ", " \u2013 ", " \u2014 ", ", "} {
		if index := strings.Index(value, separator); index > 0 {
			value = value[:index]
		}
	}
	for _, separator := range []string{
		" at ", " in ", " for ", " with ", " di ", " pada ",
		" location", " company", " employer", " organization", " organisasi",
		" pemberi kerja", " pekerjaan", " perusahaan", " kantor",
	} {
		if index := strings.Index(strings.ToLower(value), separator); index > 0 {
			value = value[:index]
		}
	}
	value = strings.Join(strings.Fields(value), " ")
	value = trimRoleOrganization(value)
	value = trimRolePossessive(value)
	value = trimRolePredicate(value)
	value = strings.Trim(value, " .,:;!?*-\u2013\u2014")
	value = trimRolePrefix(value)
	if roleContextRejected(value) {
		return ""
	}
	return displayOrganization(value)
}

func roleTitleContextRejected(value string) bool {
	lower := " " + strings.ToLower(value) + " "
	for _, phrase := range []string{
		" author ", " biography ", " book ", " books ", " catalog ", " film ",
		" how the ", " is shaping ", " movie ", " novel ", " publisher ",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func roleContextRejected(value string) bool {
	lower := " " + strings.ToLower(value) + " "
	for _, phrase := range roleRejectionPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func trimRoleOrganization(value string) string {
	lower := strings.ToLower(value)
	index := strings.Index(lower, " of ")
	if index <= 0 {
		return value
	}
	left := strings.TrimSpace(value[:index])
	right := strings.TrimSpace(value[index+4:])
	if !leadershipRolePhrase(left) || !organizationTargetStartsProper(right) {
		return value
	}
	return left
}

func trimRolePossessive(value string) string {
	index := strings.Index(value, "'s ")
	if index <= 0 {
		return value
	}
	left := strings.TrimSpace(value[:index])
	right := strings.TrimSpace(value[index+3:])
	if roleLooksLikeOrganization(left) && roleLike(right) {
		return right
	}
	return value
}

func trimRolePredicate(value string) string {
	lower := strings.ToLower(value)
	for _, marker := range []string{
		" analyzing ", " building ", " conducting ", " coordinating ", " creating ",
		" delivering ", " designing ", " developing ", " directing ", " driving ",
		" executing ", " handling ", " implementing ", " leading ", " maintaining ",
		" managing ", " mentoring ", " organizing ", " overseeing ", " performing ",
		" planning ", " running ", " supervising ", " supporting ", " teaching ",
	} {
		if index := strings.Index(lower, marker); index > 0 {
			return strings.TrimSpace(value[:index])
		}
	}
	return value
}

func trimRolePrefix(value string) string {
	tokens := strings.Fields(value)
	for len(tokens) > 0 {
		switch strings.ToLower(strings.Trim(tokens[0], ".,")) {
		case "a", "adalah", "an", "as", "is", "merupakan", "sebagai", "seorang", "the":
			tokens = tokens[1:]
			continue
		}
		break
	}
	return strings.Join(tokens, " ")
}

func roleLike(value string) bool {
	valueWords := strings.Fields(value)
	if len(valueWords) == 0 || len(valueWords) > 10 || len([]rune(value)) > 90 {
		return false
	}
	if !hasLetter(value) {
		return false
	}
	for _, word := range words(value) {
		if roleWords[word] {
			return true
		}
	}
	return false
}

func roleRelatedToName(value, name string) bool {
	return strings.EqualFold(strings.Join(words(value), " "), strings.Join(words(name), " "))
}

func hasLetter(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}
