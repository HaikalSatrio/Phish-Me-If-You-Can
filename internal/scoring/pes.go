package scoring

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/evidence"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/identity"
)

var Names = []string{"discoverability", "linkability", "contactability", "context_richness", "scenario_support"}

func Validate(p domain.Policy) error {
	if p.Version == "" || !evidence.Unit(p.AcceptThreshold) || !evidence.Unit(p.ReviewThreshold) || p.ReviewThreshold >= p.AcceptThreshold || !evidence.Unit(p.MinimumCoverage) || p.MinimumCoverage == 0 {
		return fmt.Errorf("invalid policy identity thresholds or coverage")
	}
	if len(p.Weights) != len(Names) {
		return fmt.Errorf("policy requires exactly five dimension weights")
	}
	total := 0.0
	for _, n := range Names {
		v, ok := p.Weights[n]
		if !ok || !evidence.Unit(v) {
			return fmt.Errorf("invalid weight %s", n)
		}
		total += v
	}
	if total == 0 {
		return fmt.Errorf("weights must have a positive sum")
	}
	if len(p.ContactTypes) == 0 || len(p.ContextTypes) == 0 || len(p.Scenarios) == 0 {
		return fmt.Errorf("policy requires contact types, context types and scenarios")
	}
	groups := [][]string{p.ContactTypes, p.ContextTypes}
	for id, requirements := range p.Scenarios {
		if id == "" || len(requirements) == 0 {
			return fmt.Errorf("empty scenario")
		}
		groups = append(groups, requirements)
	}
	for _, group := range groups {
		seen := map[string]bool{}
		for _, s := range group {
			if s == "" || seen[s] {
				return fmt.Errorf("empty or duplicate policy type")
			}
			seen[s] = true
		}
	}
	return nil
}

func Calculate(c domain.Case, p domain.Policy) (domain.Report, error) {
	if err := Validate(p); err != nil {
		return domain.Report{}, err
	}
	var err error
	c, err = evidence.Normalize(c)
	if err != nil {
		return domain.Report{}, err
	}
	r := domain.Report{Target: c.Target, PolicyVersion: p.Version, Status: "INSUFFICIENT_EVIDENCE", Dimensions: map[string]domain.Dimension{}, IdentityStates: map[string]string{}, ExcludedIDs: []string{}, Scenarios: []domain.Scenario{}, Limitations: []string{"Experimental exposure index, not human phishing probability.", "Identity confidence and discoverability are supplied heuristic annotations, not calibrated probabilities.", "Coverage describes declared sources only; inferred and unresolved facts are excluded."}}
	inputJSON, err := json.Marshal(c)
	if err != nil {
		return domain.Report{}, err
	}
	policyJSON, err := json.Marshal(p)
	if err != nil {
		return domain.Report{}, err
	}
	r.SchemaVersion = "1"
	r.EngineVersion = "0.1.0"
	r.InputSHA256 = fmt.Sprintf("%x", sha256.Sum256(inputJSON))
	r.PolicySHA256 = fmt.Sprintf("%x", sha256.Sum256(policyJSON))
	confidence := map[string]float64{}
	for _, v := range c.Candidates {
		r.IdentityStates[v.ID] = identity.Resolve(v, p)
		confidence[v.ID] = v.Confidence
	}
	for _, v := range c.Coverage {
		if v.Status == "SUCCESS" || v.Status == "EMPTY" {
			r.Coverage++
		}
	}
	if len(c.Coverage) > 0 {
		r.Coverage /= float64(len(c.Coverage))
	}
	// Divergent values for single-valued fields require review before scoring.
	conflicts := map[string]map[string]bool{}
	for _, e := range c.Evidence {
		if e.Status == "OBSERVED" && r.IdentityStates[e.CandidateID] == "ACCEPTED" && (e.Type == "workplace" || e.Type == "role") {
			key := e.CandidateID + ":" + e.Type
			if conflicts[key] == nil {
				conflicts[key] = map[string]bool{}
			}
			conflicts[key][e.Value] = true
		}
	}
	byType := map[string][]string{}
	all := []string{}
	d, l := 0.0, 0.0
	for _, e := range c.Evidence {
		if e.Status != "OBSERVED" || e.Confidence == 0 || r.IdentityStates[e.CandidateID] != "ACCEPTED" || len(conflicts[e.CandidateID+":"+e.Type]) > 1 {
			r.ExcludedIDs = append(r.ExcludedIDs, e.ID)
			continue
		}
		all = append(all, e.ID)
		byType[e.Type] = append(byType[e.Type], e.ID)
		d += e.Discoverability * e.Confidence
		l += confidence[e.CandidateID] * e.Confidence
	}
	if len(all) > 0 {
		d /= float64(len(all))
		l /= float64(len(all))
	}
	r.Dimensions[Names[0]] = domain.Dimension{Value: d, EvidenceIDs: all, Explanation: "Mean discoverability annotation multiplied by evidence confidence across eligible distinct facts."}
	r.Dimensions[Names[1]] = domain.Dimension{Value: l, EvidenceIDs: all, Explanation: "Experimental linkage proxy: mean candidate confidence multiplied by evidence confidence. Not calibrated cross-platform linkage probability."}
	contact := []string{}
	contextIDs := []string{}
	contextCount := 0
	for _, t := range p.ContactTypes {
		contact = append(contact, byType[t]...)
	}
	for _, t := range p.ContextTypes {
		if len(byType[t]) > 0 {
			contextCount++
			contextIDs = append(contextIDs, byType[t]...)
		}
	}
	cv := 0.0
	if len(contact) > 0 {
		cv = 1
	}
	r.Dimensions[Names[2]] = domain.Dimension{Value: cv, EvidenceIDs: contact, Explanation: "One if at least one eligible public contact fact exists; otherwise zero within the observed profile."}
	r.Dimensions[Names[3]] = domain.Dimension{Value: float64(contextCount) / float64(len(p.ContextTypes)), EvidenceIDs: contextIDs, Explanation: "Fraction of configured context categories with eligible observed evidence."}
	scenarioIDs := []string{}
	for id := range p.Scenarios {
		scenarioIDs = append(scenarioIDs, id)
	}
	sort.Strings(scenarioIDs)
	best := domain.Dimension{EvidenceIDs: []string{}, Explanation: "Maximum fraction of satisfied prerequisites among configured scenario classes; partial support does not establish feasibility."}
	for _, id := range scenarioIDs {
		s := domain.Scenario{ID: id, Missing: []string{}, EvidenceIDs: []string{}}
		for _, t := range p.Scenarios[id] {
			if len(byType[t]) == 0 {
				s.Missing = append(s.Missing, t)
			} else {
				s.EvidenceIDs = append(s.EvidenceIDs, byType[t]...)
			}
		}
		s.Support = 1 - float64(len(s.Missing))/float64(len(p.Scenarios[id]))
		if s.Support > best.Value {
			best.Value = s.Support
			best.EvidenceIDs = s.EvidenceIDs
		}
		r.Scenarios = append(r.Scenarios, s)
	}
	r.Dimensions[Names[4]] = best
	if len(all) > 0 && r.Coverage >= p.MinimumCoverage {
		total, weight := 0.0, 0.0
		for _, n := range Names {
			total += r.Dimensions[n].Value * p.Weights[n]
			weight += p.Weights[n]
		}
		pes := math.Round(total/weight*10000) / 100
		r.PES = &pes
		r.Status = "EXPERIMENTAL"
	}
	return r, nil
}
