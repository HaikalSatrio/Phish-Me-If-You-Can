package extract

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

func TestGroundingCorrectionsAndDiagnostics(t *testing.T) {
	asOf := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, text, kind, role, org, typ, state, wantKind, wantState, code string
		start, end                                                         int
	}{
		{name: "taxonomy", text: "Ayu Lestari works as Engineer at Orbit Labs.", kind: "occupation", role: "Engineer", org: "Orbit Labs", typ: "page_text", state: "present_assertion", wantKind: "employment", wantState: "present_assertion"},
		{name: "heading", text: "Ayu Lestari - CEO of Example Software Ltd", kind: "occupation", role: "CEO", org: "Example Software Ltd", typ: "search_title", state: "present_assertion", wantKind: "employment", wantState: "headline"},
		{name: "full-date", text: "Ayu Lestari worked as Engineer at Orbit Labs from 1 January 2018 to 31 December 2022.", kind: "employment", role: "Engineer", org: "Orbit Labs", typ: "page_text", state: "present_assertion", wantKind: "employment", wantState: "historical", start: 2018, end: 2022},
		{name: "omitted-date", text: "Ayu Lestari worked as Engineer at Orbit Labs from 2018 to 2022.", kind: "employment", role: "Engineer", org: "Orbit Labs", typ: "page_text", state: "historical", code: "PERIOD_OMITTED"},
		{name: "wrong-person", text: "Ayu Lestari interviewed Budi Santoso, Engineer at Orbit Labs.", kind: "employment", role: "Engineer", org: "Orbit Labs", typ: "page_text", state: "present_assertion", code: "SUBJECT_ATTRIBUTION"},
		{name: "invented-value", text: "Ayu Lestari works at Orbit Labs.", kind: "employment", org: "Example Ltd", typ: "page_text", state: "present_assertion", code: "FIELD_NOT_QUOTED"},
		{name: "verb-is-not-role", text: "Ayu Lestari pernah bekerja di Orbit Labs.", kind: "employment", role: "pernah bekerja di", org: "Orbit Labs", typ: "page_text", state: "historical", wantKind: "employment", wantState: "historical"},
		{name: "function-is-not-org", text: "Ayu Lestari - Head of Cyber Security Program", kind: "occupation", role: "Head of Cyber Security Program", org: "Head of Cyber Security Program", typ: "search_title", state: "headline", wantKind: "occupation", wantState: "headline"},
		{name: "public-office-boundary", text: "Ayu Lestari served as President of Indonesia from 2014 to 2024.", kind: "public_office", role: "President", org: "Indonesia", typ: "page_text", state: "historical", wantKind: "public_office", wantState: "historical", start: 2014, end: 2024},
		{name: "longer-personal-name", text: "Sorie Ayu Lestari (lahir 28 Agustus 1958) adalah seorang atlet angkat besi.", kind: "occupation", role: "atlet angkat besi", typ: "page_text", state: "present_assertion", code: "SUBJECT_NAME_CONFLICT"},
		{name: "collective-is-not-role", text: "Ayu Lestari is the president of Example Foundation and serves on its Board of Directors.", kind: "employment", role: "Board of Directors", org: "Example Foundation", typ: "page_text", state: "present_assertion", code: "ROLE_IS_COLLECTIVE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := domain.Relation{Subject: "Ayu Lestari", Kind: tc.kind, Role: tc.role, Organization: tc.org, Temporal: tc.state, StartYear: tc.start, EndYear: tc.end, EvidenceID: "doc", Quote: tc.text, Status: "UNVERIFIED"}
			got, adjustments, code, detail := groundProposal(r, r.Subject, Document{ID: "doc", Type: tc.typ, Text: tc.text}, asOf, 0)
			if code != tc.code {
				t.Fatalf("code=%s detail=%s, want %s", code, detail, tc.code)
			}
			if code == "" && (got.Kind != tc.wantKind || got.Temporal != tc.wantState || len(adjustments) == 0) {
				t.Fatalf("missing grounded correction: %+v %+v", got, adjustments)
			}
			if tc.name == "verb-is-not-role" && got.Role != "" {
				t.Fatalf("verb remains in role: %+v", got)
			}
			if tc.name == "function-is-not-org" && got.Organization != "" {
				t.Fatalf("function remains in company: %+v", got)
			}
			if tc.name == "public-office-boundary" && (got.Role != "President of Indonesia" || got.Organization != "") {
				t.Fatalf("inconsistent office boundary: %+v", got)
			}
		})
	}
}

func TestLocalTraceAndSchema(t *testing.T) {
	quote := "Ayu Lestari works as Engineer at Orbit Labs."
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.Write([]byte(`{"models":[{"name":"fixture","digest":"sha256:test"}]}`))
			return
		}
		var req struct {
			Format map[string]any `json:"format"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		props := req.Format["properties"].(map[string]any)["relations"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
		if props["evidence_id"].(map[string]any)["enum"].([]any)[0] != "s1" || props["subject"].(map[string]any)["enum"].([]any)[0] != "Ayu Lestari" {
			t.Error("schema does not constrain source and subject")
		}
		proposals := []proposedRelation{{Subject: "Ayu Lestari", Kind: "employment", Role: "Engineer", Organization: "Invented Ltd", Temporal: "present_assertion", EvidenceID: "s1"}}
		body, _ := json.Marshal(map[string]any{"relations": proposals})
		json.NewEncoder(w).Encode(map[string]any{"done": true, "message": map[string]string{"content": string(body)}})
	}))
	defer srv.Close()
	for _, debug := range []bool{false, true} {
		_, trace := LocalRelations(context.Background(), Config{Mode: "hybrid", URL: srv.URL, Model: "fixture", SaveRawResponse: debug}, "Ayu Lestari", []Document{{ID: "doc", Type: "page_text", Text: quote}}, time.Now())
		if trace.Status != "REJECTED_ALL" || trace.ModelDigest != "sha256:test" || len(trace.PromptSHA256) != 64 || len(trace.Rejections) != 1 || trace.Rejections[0].Code != "FIELD_NOT_QUOTED" || (trace.RawResponse != "") != debug {
			t.Fatalf("incomplete diagnostics: %+v", trace)
		}
	}
}

func TestOrganizationMarkersDoNotConsumeFunctions(t *testing.T) {
	for _, tc := range []struct{ title, kind, org, role string }{
		{"CEO of Example Software Ltd", "employment", "Example Software Ltd", "CEO"},
		{"Head of Bank Security", "occupation", "", "Head of Bank Security"},
		{"Head of Company Research", "occupation", "", "Head of Company Research"},
	} {
		kind, org, role := profileRelationFields(tc.title)
		if kind != tc.kind || org != tc.org || role != tc.role {
			t.Fatalf("%s: got %s %s %s", tc.title, kind, org, role)
		}
	}
}
