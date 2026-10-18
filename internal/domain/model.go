package domain

type Target struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Candidate struct {
	ID             string   `json:"id"`
	Matches        []string `json:"matches"`
	Contradictions []string `json:"contradictions"`
	Missing        []string `json:"missing"`
	Confidence     float64  `json:"confidence"`
	Decision       string   `json:"decision"`
}

type Source struct {
	URL        string `json:"url"`
	ObservedAt string `json:"observed_at"`
	Collector  string `json:"collector"`
}

type Evidence struct {
	ID              string   `json:"id"`
	CandidateID     string   `json:"candidate_id"`
	Type            string   `json:"type"`
	Value           string   `json:"value"`
	Status          string   `json:"status"`
	Confidence      float64  `json:"confidence"`
	Discoverability float64  `json:"discoverability"`
	Sources         []Source `json:"sources"`
	SupportingIDs   []string `json:"supporting_ids,omitempty"`
	Aliases         []string `json:"aliases,omitempty"`
}

type Coverage struct {
	Source string `json:"source"`
	Status string `json:"status"`
}

type Case struct {
	SchemaVersion string      `json:"schema_version"`
	Target        Target      `json:"target"`
	Candidates    []Candidate `json:"candidates"`
	Evidence      []Evidence  `json:"evidence"`
	Coverage      []Coverage  `json:"coverage"`
}

type Policy struct {
	Version         string              `json:"version"`
	AcceptThreshold float64             `json:"accept_threshold"`
	ReviewThreshold float64             `json:"review_threshold"`
	MinimumCoverage float64             `json:"minimum_coverage"`
	Weights         map[string]float64  `json:"weights"`
	ContactTypes    []string            `json:"contact_types"`
	ContextTypes    []string            `json:"context_types"`
	Scenarios       map[string][]string `json:"scenarios"`
}

type Dimension struct {
	Value       float64  `json:"value"`
	EvidenceIDs []string `json:"evidence_ids"`
	Explanation string   `json:"explanation"`
}

type Scenario struct {
	ID          string   `json:"id"`
	Support     float64  `json:"support"`
	Missing     []string `json:"missing"`
	EvidenceIDs []string `json:"evidence_ids"`
}

type Report struct {
	SchemaVersion  string               `json:"schema_version"`
	EngineVersion  string               `json:"engine_version"`
	InputSHA256    string               `json:"input_sha256"`
	PolicySHA256   string               `json:"policy_sha256"`
	Target         Target               `json:"target"`
	PolicyVersion  string               `json:"policy_version"`
	Status         string               `json:"status"`
	PES            *float64             `json:"pes"`
	Coverage       float64              `json:"coverage"`
	Dimensions     map[string]Dimension `json:"dimensions"`
	IdentityStates map[string]string    `json:"identity_states"`
	ExcludedIDs    []string             `json:"excluded_ids"`
	Scenarios      []Scenario           `json:"scenarios"`
	Limitations    []string             `json:"limitations"`
}
