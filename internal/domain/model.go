package domain

type Target struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Candidate struct {
	Label               string   `json:"label,omitempty"`
	RequiresReview      bool     `json:"requires_review,omitempty"`
	ProfileURL          string   `json:"profile_url,omitempty"`
	Username            string   `json:"username,omitempty"`
	DisplayName         string   `json:"display_name,omitempty"`
	Company             string   `json:"company,omitempty"`
	CompanyInferred     bool     `json:"company_inferred,omitempty"`
	CompanyAmbiguous    bool     `json:"company_ambiguous,omitempty"`
	CompanyAlternatives []string `json:"company_alternatives,omitempty"`
	Location            string   `json:"location,omitempty"`
	ID                  string   `json:"id"`
	Matches             []string `json:"matches"`
	Contradictions      []string `json:"contradictions"`
	Missing             []string `json:"missing"`
	Confidence          float64  `json:"confidence"`
	Decision            string   `json:"decision"`
}

type Source struct {
	ResponseSHA256 string `json:"response_sha256,omitempty"`
	Rule           string `json:"rule,omitempty"`
	URL            string `json:"url"`
	ObservedAt     string `json:"observed_at"`
	Collector      string `json:"collector"`
}

type InferenceAlternative struct {
	Value      string  `json:"value"`
	Rule       string  `json:"rule"`
	Confidence float64 `json:"confidence"`
}

type Evidence struct {
	ID                    string                 `json:"id"`
	CandidateID           string                 `json:"candidate_id"`
	Type                  string                 `json:"type"`
	Value                 string                 `json:"value"`
	Status                string                 `json:"status"`
	Confidence            float64                `json:"confidence"`
	Discoverability       float64                `json:"discoverability"`
	Sources               []Source               `json:"sources"`
	SupportingIDs         []string               `json:"supporting_ids,omitempty"`
	InferenceRule         string                 `json:"inference_rule,omitempty"`
	InferenceQuote        string                 `json:"inference_quote,omitempty"`
	InferenceQuoteType    string                 `json:"inference_quote_type,omitempty"`
	InferenceAlternatives []InferenceAlternative `json:"inference_alternatives,omitempty"`
	Aliases               []string               `json:"aliases,omitempty"`
}

type Coverage struct {
	Source string `json:"source"`
	Status string `json:"status"`
}

type Case struct {
	Receipts      []CollectionReceipt `json:"receipts,omitempty"`
	Search        *SearchSession      `json:"search,omitempty"`
	SchemaVersion string              `json:"schema_version"`
	Target        Target              `json:"target"`
	Candidates    []Candidate         `json:"candidates"`
	Evidence      []Evidence          `json:"evidence"`
	Coverage      []Coverage          `json:"coverage"`
}

type CollectionReceipt struct {
	Platform       string `json:"platform"`
	URL            string `json:"url"`
	ObservedAt     string `json:"observed_at"`
	Status         string `json:"status"`
	Rule           string `json:"rule"`
	Reason         string `json:"reason"`
	ResponseSHA256 string `json:"response_sha256,omitempty"`
	Bytes          int    `json:"bytes,omitempty"`
}

type SearchSession struct {
	Provider     string        `json:"provider,omitempty"`
	Platform     string        `json:"platform,omitempty"`
	URL          string        `json:"url,omitempty"`
	ID           string        `json:"id"`
	StartedAt    string        `json:"started_at"`
	UpdatedAt    string        `json:"updated_at"`
	Name         string        `json:"name"`
	Username     string        `json:"username,omitempty"`
	Company      string        `json:"company,omitempty"`
	Location     string        `json:"location,omitempty"`
	Attempts     int           `json:"attempts"`
	RequestsUsed int           `json:"requests_used"`
	Status       string        `json:"status"`
	Scope        string        `json:"scope"`
	ConfigJSON   string        `json:"config_json"`
	Events       []SearchEvent `json:"events"`
}

type SearchEvent struct {
	At        string `json:"at"`
	Operation string `json:"operation"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
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
