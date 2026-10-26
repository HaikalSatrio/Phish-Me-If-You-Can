package extract

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

func modelDigest(ctx context.Context, client *http.Client, endpoint, model string) string {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(endpoint, "/api/chat")+"/api/tags", nil)
	if err != nil {
		return ""
	}
	res, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return ""
	}
	var tags struct {
		Models []struct {
			Name   string `json:"name"`
			Model  string `json:"model"`
			Digest string `json:"digest"`
		} `json:"models"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&tags) != nil {
		return ""
	}
	for _, tag := range tags.Models {
		if tag.Name == model || tag.Model == model {
			return tag.Digest
		}
	}
	return ""
}

func groundProposal(r domain.Relation, subject string, doc Document, asOf time.Time, index int) (domain.Relation, []domain.ExtractionAdjustment, string, string) {
	adjustments := []domain.ExtractionAdjustment{}
	adjust := func(field, from, to, reason string) {
		if from != to {
			adjustments = append(adjustments, domain.ExtractionAdjustment{Index: index, Field: field, From: from, To: to, Reason: reason})
		}
	}
	fail := func(code, detail string) (domain.Relation, []domain.ExtractionAdjustment, string, string) {
		return r, nil, code, detail
	}
	if r.EvidenceID == "" || r.EvidenceID != doc.ID {
		return fail("EVIDENCE_ID", "The cited document was not supplied to this extraction")
	}
	if !strings.EqualFold(compact(r.Subject), compact(subject)) {
		return fail("SUBJECT", "The proposed subject differs from the requested subject")
	}
	if interfaceRole.MatchString(r.Role) {
		return fail("ROLE_IS_INTERFACE_LABEL", "The proposed role is a page label, not a profession")
	}
	if r.Quote == "" || len([]rune(r.Quote)) > 1000 {
		return fail("QUOTE_LIMIT", "Quote is empty or exceeds 1000 characters")
	}
	if !strings.Contains(doc.Text, r.Quote) {
		parts := strings.Fields(r.Quote)
		for i := range parts {
			parts[i] = regexp.QuoteMeta(parts[i])
		}
		literal := regexp.MustCompile(strings.Join(parts, `\s+`)).FindString(doc.Text)
		if literal == "" {
			return fail("QUOTE_NOT_FOUND", "Quote is not a contiguous source span, even allowing whitespace differences")
		}
		adjust("quote", r.Quote, literal, "Restored original source whitespace")
		r.Quote = literal
	}
	if !containsWords(r.Quote, subject) {
		return fail("SUBJECT_NOT_QUOTED", "Quote does not contain the requested subject")
	}
	if conflictingSubjectName(subject, r.Quote) {
		return fail("SUBJECT_NAME_CONFLICT", "The passage introduces a longer personal name, not the requested full name")
	}
	for _, v := range []struct{ name, value string }{{"role", r.Role}, {"organization", r.Organization}} {
		if v.value != "" && !containsWords(r.Quote, v.value) {
			return fail("FIELD_NOT_QUOTED", v.name+" is not literal in the source quote")
		}
	}
	if err := ValidateRelation(r, subject, doc); err != nil {
		return fail("INVALID_CLAIM", err.Error())
	}
	if negatedRelation.MatchString(r.Quote) {
		return fail("NEGATED_OR_UNCERTAIN", "Quote contains negation, a conditional, or uncertainty")
	}
	if regexp.MustCompile(`(?i)\b(?:interviewed|interviews|met|visited|mewawancarai|bertemu|mengunjungi)\b`).MatchString(r.Quote) {
		return fail("SUBJECT_ATTRIBUTION", "Quote describes an interaction; the role may belong to another person")
	}
	for _, clause := range relativeRelationship.FindAllString(r.Quote, -1) {
		if r.Role != "" && containsWords(clause, r.Role) || r.Organization != "" && containsWords(clause, r.Organization) {
			return fail("RELATIVE_ATTRIBUTION", "The quoted role or organization is attributed to a relative or friend")
		}
	}
	if r.Kind == "education" && r.Organization != "" && (headlineRole(r.Organization) || strings.EqualFold(r.Organization, r.Role)) {
		return fail("EDUCATION_IS_ROLE", "Education organization is a role or student title, not an institution")
	}
	// Deterministic corrections require an explicit relationship with the same fields.
	proofDoc := doc
	proofDoc.Text = r.Quote
	proved := false
	var proofStart, proofEnd int
	var proofTemporal string
	for _, proof := range Relations(subject, []Document{proofDoc}, asOf) {
		if proof.Kind == "public_office" && proof.Organization == "" && r.Organization != "" && strings.EqualFold(proof.Role, r.Role+" of "+r.Organization) {
			adjust("role", r.Role, proof.Role, "Preserved the public-office title boundary in the quoted source")
			adjust("organization", r.Organization, "", "Public-office jurisdiction is part of the title, not an employer")
			r.Role, r.Organization = proof.Role, ""
		}
		if proof.Kind == "occupation" && proof.Organization == "" && strings.EqualFold(r.Role, proof.Role) && strings.EqualFold(r.Organization, r.Role) {
			adjust("organization", r.Organization, "", "A supported function title was duplicated as an organization")
			r.Organization = ""
		}
		if proof.Kind == "employment" && proof.Role == "" && strings.EqualFold(proof.Organization, r.Organization) && relationshipVerbValue.MatchString(r.Role) {
			adjust("role", r.Role, "", "The proposed role is a relationship verb, not a job title")
			r.Role = ""
		}
		if proofRoleMatches(proof.Role, r.Role) && strings.EqualFold(proof.Organization, r.Organization) {
			proved = true
			proofStart, proofEnd, proofTemporal = proof.StartYear, proof.EndYear, proof.Temporal
			adjust("quote", r.Quote, proof.Quote, "Focused the selected passage on its explicit relationship")
			r.Quote = proof.Quote
			adjust("kind", r.Kind, proof.Kind, "Explicit relationship in the quoted source")
			r.Kind = proof.Kind
			adjust("temporal", r.Temporal, proof.Temporal, "Explicit tense or profile heading in the quoted source")
			r.Temporal = proof.Temporal
			break
		}
	}
	if relationshipVerbValue.MatchString(r.Role) {
		return fail("ROLE_IS_VERB", "The proposed role is a relationship verb, not a job title")
	}
	if strings.EqualFold(r.Role, "board of directors") {
		return fail("ROLE_IS_COLLECTIVE", "A governing body is not an individual's job title")
	}
	temporalText := r.Quote
	if !proved {
		// Dates and tense must belong to a clause containing the proposed fields.
		matches := []string{}
		for _, sentence := range Sentences(r.Quote) {
			if (r.Role == "" || containsWords(sentence, strings.TrimRight(r.Role, ". "))) && (r.Organization == "" || containsWords(sentence, strings.TrimRight(r.Organization, ". "))) {
				matches = append(matches, sentence)
			}
		}
		if len(matches) == 0 {
			return fail("RELATION_SCOPE", "The selected passage does not isolate the role and organization in one sentence")
		}
		temporalText = matches[0]
		start, end, state := temporal(temporalText, "", asOf)
		for _, match := range matches[1:] {
			s, e, status := temporal(match, "", asOf)
			if s != start || e != end || status != state || explicitPresent.MatchString(match) != explicitPresent.MatchString(temporalText) {
				return fail("RELATION_SCOPE", "Repeated relationship statements have incompatible periods or tense")
			}
		}
		if r.Kind == "education" && !educationCue.MatchString(temporalText) {
			return fail("EDUCATION_UNSUPPORTED", "The cited relationship has no explicit study or qualification statement")
		}
	}
	if !proved && r.Temporal == "present_assertion" && !explicitPresent.MatchString(temporalText) {
		state := "unknown"
		if doc.Type == "search_title" || doc.Type == "page_title" || doc.Type == "search_snippet" || doc.Type == "page_description" || doc.Type == "profile_description" {
			state = "headline"
		}
		adjust("temporal", r.Temporal, state, "No explicit present-tense relationship in the selected passage")
		r.Temporal = state
	}
	s, e, state := temporal(temporalText, "", asOf)
	if proved {
		s, e, state = proofStart, proofEnd, proofTemporal
	}
	if r.StartYear != 0 && r.StartYear != s || r.EndYear != 0 && r.EndYear != e {
		return fail("PERIOD_UNSUPPORTED", "Proposed start/end years do not match a supported date expression in the quote")
	}
	if s != 0 && r.StartYear == 0 || e != 0 && r.EndYear == 0 {
		return fail("PERIOD_OMITTED", "The quote specifies a period but the proposal omits it")
	}
	if state == "historical" || state == "future" {
		adjust("temporal", r.Temporal, state, "Explicit source period or past-tense statement")
		r.Temporal = state
	}
	switch r.Kind {
	case "employment":
		if r.Organization == "" {
			return fail("RELATION_SHAPE", "Employment must identify an employer")
		}
	case "occupation":
		if r.Organization != "" || r.Role == "" {
			return fail("RELATION_SHAPE", "Occupation requires a role without an employer; use employment for an explicit employer")
		}
	case "public_office":
		if r.Role == "" || !publicOffice.MatchString(r.Role) {
			return fail("RELATION_SHAPE", "Public office requires an explicit government position")
		}
	case "birthplace", "education", "membership":
		if r.Organization == "" {
			return fail("RELATION_SHAPE", "This relationship must identify a place or organization")
		}
	}
	return r, adjustments, "", ""
}

var relationshipVerbValue = regexp.MustCompile(`(?i)^(?:works?|worked|serves?|served|plays?|played)(?:\s+(?:at|as|in|for))?$|^(?:pernah\s+)?(?:bekerja|menjabat)(?:\s+(?:di|sebagai))?$`)

var educationCue = regexp.MustCompile(`(?i)\b(?:studied|studies|studying|student|undergraduate|postgraduate|graduated|graduate|alumnus|alumna|alumni|attended|trained|training|enrolled|degree|doctorate|bachelor|master|PhD|Ph\.D|belajar|kuliah|mahasiswa|mahasiswi|lulusan|lulus|sarjana|pendidikan|pendidikannya|bersekolah)\b`)

var explicitPresent = regexp.MustCompile(`(?i)\b(?:is|works|serves|adalah|merupakan|menjabat|bekerja|I am|I'm|I\x{2019}m)\b`)

var relativeRelationship = regexp.MustCompile(`(?i)\b(?:husband|wife|mother|father|daughter|son|sister|brother|friend|suami|istri|ayah|ibu|anak|kakak|adik|teman)\s+(?:is|was|works\s+(?:as|at)|worked\s+(?:as|at)|adalah|merupakan|bekerja\s+(?:sebagai|di))\s+[^.;\n]{1,180}`)

func proofRoleMatches(proof, proposed string) bool {
	if strings.EqualFold(proof, proposed) {
		return true
	}
	for _, part := range roleConjunction.Split(proof, -1) {
		if strings.EqualFold(strings.TrimSpace(part), proposed) {
			return true
		}
	}
	return false
}

func conflictingSubjectName(subject, text string) bool {
	// A biographical introduction may reveal a different, longer personal name.
	pattern := regexp.MustCompile(`(?m)^([\p{Lu}][\p{L}'-]+(?:\s+[\p{Lu}][\p{L}'-]+){0,3})\s+` + regexp.QuoteMeta(subject) + `(?:\s*\([^)]{0,220}\))?\s+(?:is|was|adalah|merupakan)\b`)
	for _, m := range pattern.FindAllStringSubmatch(text, -1) {
		if !literalRole(m[1]) && !strings.EqualFold(m[1], "Dr") && !strings.EqualFold(m[1], "Professor") {
			return true
		}
	}
	return false
}
