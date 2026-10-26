package extract

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

// This optional audit revalidates saved accepted proposals, not fresh model outputs.
func TestMatrixProposalReplay(t *testing.T) {
	root, output := os.Getenv("PMIYC_MATRIX_REPLAY_INPUT"), os.Getenv("PMIYC_MATRIX_REPLAY_OUTPUT")
	if root == "" || output == "" {
		t.Skip("set PMIYC_MATRIX_REPLAY_INPUT and PMIYC_MATRIX_REPLAY_OUTPUT for an offline proposal audit")
	}
	paths, err := filepath.Glob(filepath.Join(root, "*", "*", "*-ollama.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("matrix inputs: %v", err)
	}
	rows := []map[string]any{}
	codes := map[string]int{}
	accepted := 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var c domain.Case
		if err := json.Unmarshal(data, &c); err != nil {
			t.Fatal(err)
		}
		docs := map[string]Document{}
		for _, e := range c.Evidence {
			if e.Status == "OBSERVED" {
				docs[e.ID] = Document{ID: e.ID, Type: e.Type, Text: e.Value}
			}
		}
		for _, r := range c.Relations {
			if !strings.HasPrefix(r.Method, "ollama/") {
				continue
			}
			asOf := time.Time{}
			for _, trace := range c.Extraction {
				if trace.Mode == "hybrid" && trace.CandidateID == r.CandidateID {
					asOf, err = time.Parse(time.RFC3339, trace.ObservedAt)
					if err != nil {
						t.Fatal(err)
					}
					break
				}
			}
			if asOf.IsZero() {
				t.Fatal("missing model extraction timestamp")
			}
			got, adjustments, code, detail := groundProposal(r, r.Subject, docs[r.EvidenceID], asOf, 0)
			if code == "" {
				accepted++
			} else {
				codes[code]++
			}
			rows = append(rows, map[string]any{
				"snapshot": path, "snapshot_sha256": fmt.Sprintf("%x", sha256.Sum256(data)),
				"relation_id": r.ID, "subject": r.Subject, "kind": r.Kind,
				"role": r.Role, "organization": r.Organization, "before_temporal": r.Temporal,
				"accepted": code == "", "code": code, "detail": detail,
				"after_temporal": got.Temporal, "adjustments": adjustments,
			})
		}
	}
	result := map[string]any{"version": RelationVersion, "method": "Revalidation of previously accepted model proposals with original source text and extraction timestamps. No new model calls; previously rejected proposals are unavailable.", "snapshots": len(paths), "proposals": len(rows), "accepted": accepted, "rejections": codes, "claims": rows}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(append(data, '\n')); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("revalidated %d proposals: %d accepted, rejections %v", len(rows), accepted, codes)
}
