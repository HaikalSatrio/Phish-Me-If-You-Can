package personname

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

const Version = "public-names-v1"

// Only explicit, bounded naming clauses are supported. Names are never guessed
// from usernames, transliteration, model memory, or similarity.
const token = `(?:[\p{Lu}]\.|(?:Jr|Sr)\.|[\p{L}][\p{L}\p{M}'\x{2019}\-]*)`
const name = token + `(?:[ \t]+` + token + `){0,7}`
const boundary = `(?:^|[.!?\n][ \t]*)`
const known = `(?i:(?:(?:also|better|best|professionally|widely|commonly) )?known (?:(?:professionally |online |mononymously )?as|by (?:his |her |the )?(?:(?:stage|pen|ring) name|nickname))|(?:yang )?(?:(?:lebih|juga) )?dikenal (?:secara profesional )?(?:dengan nama(?: panggung| ring| pena)?|sebagai))`
const predicate = `(?i:is|was|adalah|merupakan)`

var forward = regexp.MustCompile(boundary + `(` + name + `)(?:\s+\([^\n)]{0,240}\))?,?\s+` + known + `\s+([^,;!?()\n]{1,100}?)(?:\s+\([^\n)]{0,160}\))?,?\s+` + predicate + `(?:\s+|$)`)
var reverse = regexp.MustCompile(boundary + `([^,;!?\n]{1,80}?),\s*(?i:whose real name is|whose birth name is|bernama asli|memiliki nama asli)\s+(` + name + `),`)
var possessive = regexp.MustCompile(boundary + `([^,;!?\n]{1,80}?)(?i:'s real name is|'s birth name is)\s+(` + name + `)(?:[.!?]|$)`)
var parenthetical = regexp.MustCompile(boundary + `([^,;!?\n]{1,80}?)\s+\((?i:real name|birth name|nama asli)[: ]+(` + name + `)\)`)
var stage = regexp.MustCompile(boundary + `([^,;!?\n]{1,80}?)\s+(?i:is the (?:stage|pen) name of|adalah nama (?:panggung|pena) dari)\s+(` + name + `)(?:[.!?]|$)`)
var aliasSplit = regexp.MustCompile(`(?i)\s+(?:or|atau|and|dan)\s+(?:(?:simply|just)\s+)?`)
var unsafe = regexp.MustCompile(`(?i)\b(?:not|never|rumou?red|allegedly|reportedly|might|maybe|possibly|bukan|tidak|diduga|dikabarkan|konon|claims?|claimed|pretends?|impersonat\w*|fanpage|fan\s+(?:page|account|club)|parody|parodi|akun\s+penggemar|supposedly|if)\b`)
var unknownName = regexp.MustCompile(`(?i)\b(?:unknown|undisclosed|anonymous|unconfirmed|rahasia|anonim)\b`)
var nonPerson = regexp.MustCompile(`(?i)^(?:an? |the |sebuah )?(?:[\p{L}-]+ ){0,3}(?:band|company|corporation|team|game|group|perusahaan|grup|kelompok|fictional character)\b`)
var postnominals = regexp.MustCompile(`(?:\s+(?:KBE|OBE|CBE|MBE|FRSA|FRS|PhD|Ph\.D\.|MD|MSc|BSc))+$`)

func WithoutPostnominals(name string) string { return postnominals.ReplaceAllString(name, "") }

func Equal(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}

func TextType(typ string) bool {
	switch typ {
	case "search_snippet", "page_description", "profile_description", "page_text":
		return true
	}
	return false
}

func validName(s string) bool {
	if unknownName.MatchString(s) {
		return false
	}
	parts := strings.Fields(s)
	if len(parts) == 0 || len(parts) > 8 || len([]rune(s)) > 120 {
		return false
	}
	for i, p := range parts {
		if i > 0 && strings.Contains(" van von de del da dos di la der den bin binti al ", " "+p+" ") {
			continue
		}
		first := []rune(p)[0]
		if !unicode.IsUpper(first) && !(unicode.IsLetter(first) && !unicode.IsLower(first)) {
			return false
		}
	}
	return !unsafe.MatchString(s)
}

// Find returns source assertions for the exact supplied alias or public name.
// Every quote remains a literal span of text and is scoped to one introduction.
func Find(subject, text string) []domain.NameClaim {
	out := []domain.NameClaim{}
	if strings.TrimSpace(subject) == "" {
		return out
	}
	seen := map[string]bool{}
	add := func(alias, realName, quote, kind string) {
		alias, realName = strings.TrimSpace(alias), strings.TrimSpace(realName)
		realName = WithoutPostnominals(realName)
		// A terminal full stop is not part of a name, except for common suffixes/initials.
		if strings.HasSuffix(realName, ".") && !strings.HasSuffix(realName, "Jr.") && !strings.HasSuffix(realName, "Sr.") {
			realName = strings.TrimSuffix(realName, ".")
		}
		if (!Equal(subject, alias) && !Equal(subject, realName)) || Equal(alias, realName) || !validName(realName) || unsafe.MatchString(quote) {
			return
		}
		if len([]rune(alias)) > 80 || len(strings.Fields(alias)) > 6 || strings.ContainsAny(alias, "()[]:") {
			return
		}
		key := strings.ToLower(alias + "\x00" + realName + "\x00" + quote)
		if !seen[key] {
			seen[key] = true
			out = append(out, domain.NameClaim{Alias: alias, Name: realName, Kind: kind, Quote: quote, Method: Version, Status: "UNVERIFIED"})
		}
	}
	for _, at := range forward.FindAllStringSubmatchIndex(text, 32) {
		if nonPerson.MatchString(text[at[1]:]) {
			continue
		}
		m := []string{text[at[0]:at[1]], text[at[2]:at[3]], text[at[4]:at[5]]}
		quote := strings.TrimLeft(m[0], ".!?\n \t")
		for _, alias := range aliasSplit.Split(m[2], -1) {
			add(alias, m[1], strings.TrimSpace(quote), "known_as")
		}
	}
	for _, pattern := range []*regexp.Regexp{reverse, possessive, parenthetical, stage} {
		for _, m := range pattern.FindAllStringSubmatch(text, 32) {
			add(m[1], m[2], strings.TrimSpace(strings.TrimLeft(m[0], ".!?\n \t")), "explicit_name")
		}
	}
	return out
}

// ExcludedContext prevents a copied biography being presented as a fan account's name.
func ExcludedContext(title string) bool { return unsafe.MatchString(title) }
