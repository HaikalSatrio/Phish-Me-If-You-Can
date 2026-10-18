package evidence

import (
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
	"testing"
)

func TestInvalidProvenanceGraph(t *testing.T) {
	for _, kind := range []string{"cycle", "cross_candidate", "alias_collision", "missing_reference"} {
		t.Run(kind, func(t *testing.T) {
			var c domain.Case
			if err := storage.Read("../../fixtures/kevin.json", &c); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "cycle":
				c.Evidence[0].SupportingIDs = []string{c.Evidence[1].ID}
				c.Evidence[1].SupportingIDs = []string{c.Evidence[0].ID}
			case "cross_candidate":
				c.Evidence[0].SupportingIDs = []string{c.Evidence[len(c.Evidence)-1].ID}
			case "alias_collision":
				c.Evidence[0].Aliases = []string{c.Evidence[1].ID}
			case "missing_reference":
				c.Evidence[0].SupportingIDs = []string{"nonexistent"}
			}
			if _, err := Normalize(c); err == nil {
				t.Fatal("invalid graph accepted")
			}
		})
	}
}
