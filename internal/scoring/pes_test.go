package scoring

import (
	"reflect"
	"testing"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/storage"
)

func fixture(t *testing.T) (domain.Case, domain.Policy) {
	t.Helper()
	var c domain.Case
	var p domain.Policy
	if err := storage.Read("../../fixtures/kevin.json", &c); err != nil {
		t.Fatal(err)
	}
	if err := storage.Read("../../configs/experimental-v1.json", &p); err != nil {
		t.Fatal(err)
	}
	return c, p
}

func TestDuplicateResistanceAndReproducibility(t *testing.T) {
	c, p := fixture(t)
	first, err := Calculate(c, p)
	if err != nil {
		t.Fatal(err)
	}
	copy := c.Evidence[0]
	copy.ID = "duplicate"
	c.Evidence = append(c.Evidence, copy)
	second, err := Calculate(c, p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Dimensions, second.Dimensions) || *first.PES != *second.PES {
		t.Fatal("duplicate changed scoring")
	}
	n, err := evidence.Normalize(c)
	if err != nil {
		t.Fatal(err)
	}
	again, err := evidence.Normalize(n)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(n, again) {
		t.Fatal("normalization is not idempotent")
	}
}

func TestAbstentionAndCoverage(t *testing.T) {
	for _, name := range []string{"empty", "low_identity", "failed_source", "contradiction"} {
		t.Run(name, func(t *testing.T) {
			c, p := fixture(t)
			switch name {
			case "empty":
				c.Evidence = nil
			case "low_identity":
				for i := range c.Candidates {
					c.Candidates[i].Confidence = 0.1
					c.Candidates[i].Decision = "PENDING"
				}
			case "failed_source":
				for i := range c.Coverage {
					c.Coverage[i].Status = "FAILED"
				}
			case "contradiction":
				for i := range c.Candidates {
					c.Candidates[i].Contradictions = []string{"conflicting identifier"}
				}
			}
			r, err := Calculate(c, p)
			if err != nil {
				t.Fatal(err)
			}
			if r.PES != nil || r.Status != "INSUFFICIENT_EVIDENCE" {
				t.Fatal("expected abstention")
			}
		})
	}
}

func TestInvalidInputs(t *testing.T) {
	c, p := fixture(t)
	c.Evidence[0].Sources = nil
	if _, err := Calculate(c, p); err == nil {
		t.Fatal("missing source accepted")
	}
	c, p = fixture(t)
	p.Weights["linkability"] = -1
	if _, err := Calculate(c, p); err == nil {
		t.Fatal("negative weight accepted")
	}
}

func TestDifferentProfiles(t *testing.T) {
	c, p := fixture(t)
	a, err := Calculate(c, p)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Read("../../fixtures/laura.json", &c); err != nil {
		t.Fatal(err)
	}
	b, err := Calculate(c, p)
	if err != nil {
		t.Fatal(err)
	}
	if a.PES == nil || b.PES == nil || *a.PES == *b.PES {
		t.Fatal("expected distinct scores")
	}
}
