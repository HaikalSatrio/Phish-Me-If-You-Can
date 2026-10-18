package changes

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
)

type Field struct {
	Type              string   `json:"type"`
	State             string   `json:"state"`
	Before            []string `json:"before"`
	After             []string `json:"after"`
	BeforeEvidenceIDs []string `json:"before_evidence_ids"`
	AfterEvidenceIDs  []string `json:"after_evidence_ids"`
}

type Account struct {
	URL    string                   `json:"url"`
	State  string                   `json:"state"`
	Reason string                   `json:"reason"`
	Before domain.CollectionReceipt `json:"before_receipt"`
	After  domain.CollectionReceipt `json:"after_receipt"`
	Fields []Field                  `json:"fields"`
}

type Report struct {
	Version      string    `json:"version"`
	BeforeSHA256 string    `json:"before_snapshot_sha256"`
	AfterSHA256  string    `json:"after_snapshot_sha256"`
	Accounts     []Account `json:"accounts"`
	Limitations  []string  `json:"limitations"`
}

func snapshotHash(c domain.Case) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(b)), nil
}

func observations(c domain.Case, raw string) map[string][]domain.Evidence {
	ids := map[string]bool{}
	for _, candidate := range c.Candidates {
		if candidate.ProfileURL == raw {
			ids[candidate.ID] = true
		}
	}
	result := map[string][]domain.Evidence{}
	for _, e := range c.Evidence {
		if ids[e.CandidateID] && e.Status == "OBSERVED" {
			result[e.Type] = append(result[e.Type], e)
		}
	}
	return result
}

func values(es []domain.Evidence) ([]string, []string) {
	vs, ids := []string{}, []string{}
	for _, e := range es {
		vs = append(vs, e.Value)
		ids = append(ids, e.ID)
	}
	sort.Strings(vs)
	sort.Strings(ids)
	return vs, ids
}

func Compare(before, after domain.Case) (Report, error) {
	r := Report{Version: "snapshot-diff-v1", Accounts: []Account{}, Limitations: []string{
		"Compares observations at the same account URL, not verified ownership by a person.",
		"NOT_OBSERVED is not proof of deletion or successful privacy remediation.",
		"Unavailable pages and changed parser rules are incomparable, not reduced exposure.",
		"Response hashes identify collected bytes; they are not signatures or proof of authenticity. Raw HTML is not archived.",
		"Snapshot hashes cover canonical JSON serialization. This comparison does not calculate a PES change.",
	}}
	var err error
	before, err = evidence.Normalize(before)
	if err != nil {
		return r, fmt.Errorf("before: %w", err)
	}
	after, err = evidence.Normalize(after)
	if err != nil {
		return r, fmt.Errorf("after: %w", err)
	}
	if before.Search == nil || after.Search == nil || before.Search.Provider != "direct" || after.Search.Provider != "direct" || len(before.Receipts) == 0 || len(after.Receipts) == 0 {
		return r, fmt.Errorf("comparison requires two direct collection snapshots with receipts")
	}
	b, a := before.Search, after.Search
	if !strings.EqualFold(b.Username, a.Username) || b.Platform != a.Platform || b.Name != a.Name || b.Company != a.Company || b.Location != a.Location || b.ConfigJSON != a.ConfigJSON {
		return r, fmt.Errorf("snapshots must use the same username, platform, clues, and collector configuration")
	}
	r.BeforeSHA256, err = snapshotHash(before)
	if err != nil {
		return r, err
	}
	r.AfterSHA256, err = snapshotHash(after)
	if err != nil {
		return r, err
	}
	left, right := map[string]domain.CollectionReceipt{}, map[string]domain.CollectionReceipt{}
	keys := map[string]bool{}
	for _, v := range before.Receipts {
		left[v.URL] = v
		keys[v.URL] = true
	}
	for _, v := range after.Receipts {
		right[v.URL] = v
		keys[v.URL] = true
	}
	urls := []string{}
	for key := range keys {
		urls = append(urls, key)
	}
	sort.Strings(urls)
	for _, raw := range urls {
		old, oldOK := left[raw]
		next, nextOK := right[raw]
		account := Account{URL: raw, Before: old, After: next, Fields: []Field{}, State: "INCOMPARABLE", Reason: "A matching successful collection with the same extraction rule is required in both snapshots"}
		if oldOK && nextOK {
			oldTime, _ := time.Parse(time.RFC3339, old.ObservedAt)
			nextTime, _ := time.Parse(time.RFC3339, next.ObservedAt)
			if nextTime.Before(oldTime) {
				return r, fmt.Errorf("after snapshot predates before for %s", raw)
			}
		}
		if oldOK && nextOK && old.Status == "SUCCESS" && next.Status == "SUCCESS" && old.Rule == next.Rule {
			account.State, account.Reason = "UNCHANGED", "Same extracted observations under the same rule; response hashes may differ due to page rendering"
			previous, current := observations(before, raw), observations(after, raw)
			types := map[string]bool{}
			for kind := range previous {
				types[kind] = true
			}
			for kind := range current {
				types[kind] = true
			}
			ordered := []string{}
			for kind := range types {
				ordered = append(ordered, kind)
			}
			sort.Strings(ordered)
			for _, kind := range ordered {
				oldValues, oldIDs := values(previous[kind])
				nextValues, nextIDs := values(current[kind])
				field := Field{Type: kind, State: "UNCHANGED", Before: oldValues, After: nextValues, BeforeEvidenceIDs: oldIDs, AfterEvidenceIDs: nextIDs}
				switch {
				case len(oldValues) == 0:
					field.State = "NEWLY_OBSERVED"
				case len(nextValues) == 0:
					field.State = "NOT_OBSERVED"
				case !equalValues(oldValues, nextValues):
					field.State = "CHANGED"
				}
				if field.State != "UNCHANGED" {
					account.State, account.Reason = "OBSERVATIONS_CHANGED", "Extracted values differ; inspect evidence before interpreting exposure changes"
				}
				account.Fields = append(account.Fields, field)
			}
		}
		r.Accounts = append(r.Accounts, account)
	}
	return r, nil
}

func equalValues(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
