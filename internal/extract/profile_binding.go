package extract

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/profile"
)

const ProfileBindingVersion = "profile-binding-v1"

var bioOccupation = regexp.MustCompile(`\b(?:(?:Cyber|Security|Cybersecurity|Software|Computer|Senior|Junior|Digital|Information|Technical|Business)\s+){0,4}(?:Lecturer|Engineer|Developer|Researcher|Teacher|Professor|Consultant|Analyst|Dosen|Peneliti)\b`)
var otherBioSubject = regexp.MustCompile(`(?i)\b(?:husband|wife|friend|daughter|son|father|mother|suami|istri|ayah|ibu|teman|hiring|seeking|aspiring|wannabe|aspire|looking for|want to be|dream of|interviewed|visited)\b`)
var indexedEducation = regexp.MustCompile(`(?i)(?:^|[\x{00b7}\x{2022}])\s*(?:Pendidikan|Education):\s*([^|\x{00b7}\x{2022}\n]+)(?:[\x{00b7}\x{2022}]|$)`)

// Profile claims can cite a title for their subject and a separate literal body span.
func BoundProfileRelations(subject, raw string, docs []Document, asOf time.Time) []domain.Relation {
	platform, handle, ok := profile.Account(raw)
	if !ok {
		return nil
	}
	var title Document
	for _, doc := range docs {
		if (doc.Type != "page_title" && doc.Type != "search_title") || len([]rune(doc.Text)) > 1000 {
			continue
		}
		name, valid := profile.NameFromTitle(raw, doc.Text)
		if !valid {
			continue
		}
		if !strings.EqualFold(compact(name), compact(subject)) {
			return nil
		}
		if title.ID == "" || doc.Type == "page_title" {
			title = doc
		}
	}
	if title.ID == "" {
		return nil
	}
	out := []domain.Relation{}
	add := func(doc Document, quote, kind, org, role string) {
		if len([]rune(quote)) > 1000 || negatedRelation.MatchString(quote) {
			return
		}
		s, e, temporal := temporal(quote, "", asOf)
		if temporal == "unknown" {
			temporal = "headline"
		}
		r := domain.Relation{Subject: subject, SubjectEvidenceID: title.ID, SubjectQuote: title.Text, EvidenceID: doc.ID, Quote: quote, Kind: kind, Organization: org, Role: role, StartYear: s, EndYear: e, Temporal: temporal, Status: "UNVERIFIED", Method: ProfileBindingVersion}
		if ValidateRelation(r, subject, doc) == nil {
			out = append(out, r)
		}
	}
	for _, doc := range docs {
		if doc.Type != "search_snippet" && doc.Type != "page_description" && doc.Type != "profile_description" {
			continue
		}
		if len([]rune(doc.Text)) > 1600 || negatedRelation.MatchString(doc.Text) {
			continue
		}
		if platform == "instagram" {
			marker := regexp.MustCompile(`(?i)(?:^|\s)@` + regexp.QuoteMeta(handle) + `:\s*["\x{201c}]`)
			loc := marker.FindStringIndex(doc.Text)
			if loc == nil {
				continue
			}
			body := doc.Text[loc[1]:]
			if stop := strings.IndexAny(body, "\"\u201d"); stop >= 0 {
				body = body[:stop]
			}
			if otherBioSubject.MatchString(body) {
				continue
			}
			for _, match := range bioOccupation.FindAllStringIndex(body, -1) {
				role := body[match[0]:match[1]]
				// The quote retains the account marker and context before the role.
				quote := strings.TrimSpace(doc.Text[loc[0] : loc[1]+match[1]])
				add(doc, quote, "occupation", "", role)
			}
			continue
		}
		rest := strings.TrimSpace(doc.Text)
		for i := 0; i < 10; i++ {
			loc := credentialPrefix.FindStringIndex(rest)
			if loc == nil {
				break
			}
			rest = strings.TrimLeft(rest[loc[1]:], " |\u00b7\u2022")
		}
		start := strings.Index(doc.Text, rest)
		if start < 0 || rest == "" {
			continue
		}
		segments := segmentRanges(rest)
		if len(segments) > 0 {
			first := strings.TrimSpace(rest[segments[0][0]:segments[0][1]])
			kind, _, _ := profileRelationFields(first)
			if kind != "" && !otherBioSubject.MatchString(rest) {
				for _, match := range indexedEducation.FindAllStringSubmatchIndex(rest, -1) {
					org := strings.TrimSpace(rest[match[2]:match[3]])
					if !strings.Contains(org, "...") && !strings.Contains(org, "\u2026") && validLiteral(org, 14) && organizationMarker.MatchString(org) && organizationTargetStartsProper(org) {
						add(doc, strings.TrimSpace(rest[match[0]:match[1]]), "education", org, "")
					}
				}
			}
		}
		for _, loc := range segments {
			part := strings.TrimSpace(rest[loc[0]:loc[1]])
			if strings.Contains(part, "...") || strings.Contains(part, "\u2026") {
				break
			}
			kind, org, role := profileRelationFields(part)
			if kind == "" {
				break
			}
			add(doc, strings.TrimSpace(doc.Text[:start+loc[1]]), kind, org, role)
		}
	}
	return out
}

func ValidateProfileBinding(r domain.Relation, raw string, body, title Document) error {
	if r.Method != ProfileBindingVersion || r.SubjectEvidenceID != title.ID || r.SubjectQuote != title.Text {
		return fmt.Errorf("invalid profile subject reference")
	}
	for _, proof := range BoundProfileRelations(r.Subject, raw, []Document{title, body}, time.Now().UTC()) {
		if proof.Quote == r.Quote && proof.Kind == r.Kind && proof.Role == r.Role && proof.Organization == r.Organization {
			return nil
		}
	}
	return fmt.Errorf("profile title and body do not support this claim")
}
