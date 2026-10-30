package domain

type Target struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Candidate struct {
	EducationClaims     []SummaryClaim `json:"education_claims,omitempty"`
	DiscoveryBasis      string         `json:"discovery_basis,omitempty"`
	LinkedFrom          string         `json:"linked_from,omitempty"`
	RoleClaims          []SummaryClaim `json:"role_claims,omitempty"`
	CompanyClaims       []SummaryClaim `json:"company_claims,omitempty"`
	Label               string         `json:"label,omitempty"`
	RequiresReview      bool           `json:"requires_review,omitempty"`
	ProfileURL          string         `json:"profile_url,omitempty"`
	Username            string         `json:"username,omitempty"`
	DisplayName         string         `json:"display_name,omitempty"`
	Company             string         `json:"company,omitempty"`
	CompanyStatus       string         `json:"company_status,omitempty"`
	CompanyInferred     bool           `json:"company_inferred,omitempty"`
	CompanyTemporal     string         `json:"company_temporal,omitempty"`
	CompanyRelationIDs  []string       `json:"company_relation_ids,omitempty"`
	CompanyAmbiguous    bool           `json:"company_ambiguous,omitempty"`
	CompanyAlternatives []string       `json:"company_alternatives,omitempty"`
	Role                string         `json:"role,omitempty"`
	RoleInferred        bool           `json:"role_inferred,omitempty"`
	RoleTemporal        string         `json:"role_temporal,omitempty"`
	RoleRelationIDs     []string       `json:"role_relation_ids,omitempty"`
	RoleAlternatives    []string       `json:"role_alternatives,omitempty"`
	Location            string         `json:"location,omitempty"`
	ID                  string         `json:"id"`
	Matches             []string       `json:"matches"`
	Contradictions      []string       `json:"contradictions"`
	Missing             []string       `json:"missing"`
	Confidence          float64        `json:"confidence"`
	Decision            string         `json:"decision"`
}

type SummaryClaim struct {
	Value       string   `json:"value"`
	Kind        string   `json:"kind"`
	Temporal    string   `json:"temporal"`
	RelationIDs []string `json:"relation_ids"`
}

type Source struct {
	RequestURL     string `json:"request_url,omitempty"`
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
	SupersededBy          string                 `json:"superseded_by,omitempty"`
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

type PersonRecord struct {
	Name        string   `json:"name"`
	JobTitles   []string `json:"job_titles,omitempty"`
	Employers   []string `json:"employers,omitempty"`
	Schools     []string `json:"schools,omitempty"`
	Memberships []string `json:"memberships,omitempty"`
	BirthPlaces []string `json:"birth_places,omitempty"`
}

type Coverage struct {
	Source string `json:"source"`
	Status string `json:"status"`
}

type Case struct {
	SocialPosts     []SocialPost        `json:"social_posts,omitempty"`
	NameClaims      []NameClaim         `json:"name_claims,omitempty"`
	IdentityReviews []IdentityReview    `json:"identity_reviews,omitempty"`
	Relations       []Relation          `json:"relations,omitempty"`
	Extraction      []ExtractionRun     `json:"extraction,omitempty"`
	Receipts        []CollectionReceipt `json:"receipts,omitempty"`
	Search          *SearchSession      `json:"search,omitempty"`
	SchemaVersion   string              `json:"schema_version"`
	Target          Target              `json:"target"`
	Candidates      []Candidate         `json:"candidates"`
	Evidence        []Evidence          `json:"evidence"`
	Coverage        []Coverage          `json:"coverage"`
}

// NameClaim records an explicit public name relationship, not account ownership.
type NameClaim struct {
	ID          string `json:"id"`
	CandidateID string `json:"candidate_id"`
	Alias       string `json:"alias"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	EvidenceID  string `json:"evidence_id"`
	Quote       string `json:"quote"`
	Method      string `json:"method"`
	Status      string `json:"status"`
}

// Relation is an unverified statement about a subject, not an identity decision.
type Relation struct {
	SubjectEvidenceID string           `json:"subject_evidence_id,omitempty"`
	SubjectQuote      string           `json:"subject_quote,omitempty"`
	Reviews           []RelationReview `json:"reviews,omitempty"`
	ID                string           `json:"id"`
	CandidateID       string           `json:"candidate_id"`
	Subject           string           `json:"subject"`
	Kind              string           `json:"kind"`
	Organization      string           `json:"organization,omitempty"`
	Role              string           `json:"role,omitempty"`
	StartYear         int              `json:"start_year,omitempty"`
	EndYear           int              `json:"end_year,omitempty"`
	Temporal          string           `json:"temporal"`
	EvidenceID        string           `json:"evidence_id"`
	Quote             string           `json:"quote"`
	Method            string           `json:"method"`
	Status            string           `json:"status"`
}

type CandidateDecision struct {
	CandidateID string `json:"candidate_id"`
	Decision    string `json:"decision"`
}

type IdentityReview struct {
	At        string              `json:"at"`
	Reason    string              `json:"reason"`
	Decisions []CandidateDecision `json:"decisions"`
}

type RelationReview struct {
	Decision string `json:"decision"`
	At       string `json:"at"`
	Reason   string `json:"reason"`
}

func RelationDecision(r Relation) string {
	if len(r.Reviews) == 0 {
		return "PENDING"
	}
	return r.Reviews[len(r.Reviews)-1].Decision
}

type ExtractionRejection struct {
	Index    int      `json:"index"`
	Code     string   `json:"code"`
	Detail   string   `json:"detail"`
	Proposal Relation `json:"proposal"`
}

type ExtractionAdjustment struct {
	Index  int    `json:"index"`
	Field  string `json:"field"`
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

type ExtractionRun struct {
	Passages       []ExtractionPassage    `json:"passages,omitempty"`
	Adjustments    []ExtractionAdjustment `json:"adjustments,omitempty"`
	Rejections     []ExtractionRejection  `json:"rejections,omitempty"`
	PromptSHA256   string                 `json:"prompt_sha256,omitempty"`
	ModelDigest    string                 `json:"model_digest,omitempty"`
	RuntimeOptions map[string]int         `json:"runtime_options,omitempty"`
	ErrorCode      string                 `json:"error_code,omitempty"`
	RawResponse    string                 `json:"raw_response,omitempty"`
	CandidateID    string                 `json:"candidate_id"`
	Version        string                 `json:"version"`
	Mode           string                 `json:"mode"`
	Model          string                 `json:"model,omitempty"`
	Status         string                 `json:"status"`
	ObservedAt     string                 `json:"observed_at"`
	InputSHA256    string                 `json:"input_sha256"`
	OutputSHA256   string                 `json:"output_sha256,omitempty"`
	DurationMS     int64                  `json:"duration_ms"`
	Accepted       int                    `json:"accepted"`
	Rejected       int                    `json:"rejected"`
	Notes          []string               `json:"notes,omitempty"`
}

type ExtractionPassage struct {
	ID         string `json:"id"`
	EvidenceID string `json:"evidence_id"`
	Type       string `json:"type"`
	Text       string `json:"text"`
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

type AccountClue struct {
	Platform string `json:"platform"`
	Username string `json:"username"`
}

type SearchSession struct {
	Access       []AccessEvent  `json:"access,omitempty"`
	Research     []ResearchStep `json:"research,omitempty"`
	Accounts     []AccountClue  `json:"accounts,omitempty"`
	Provider     string         `json:"provider,omitempty"`
	Platform     string         `json:"platform,omitempty"`
	URL          string         `json:"url,omitempty"`
	ID           string         `json:"id"`
	StartedAt    string         `json:"started_at"`
	UpdatedAt    string         `json:"updated_at"`
	Name         string         `json:"name"`
	Username     string         `json:"username,omitempty"`
	Company      string         `json:"company,omitempty"`
	Location     string         `json:"location,omitempty"`
	Attempts     int            `json:"attempts"`
	RequestsUsed int            `json:"requests_used"`
	Status       string         `json:"status"`
	Scope        string         `json:"scope"`
	ConfigJSON   string         `json:"config_json"`
	Events       []SearchEvent  `json:"events"`
}

// Research steps describe retrieval choices, never verified identity links.
type ResearchStep struct {
	Action   string `json:"action"`
	FromURL  string `json:"from_url,omitempty"`
	URL      string `json:"url,omitempty"`
	Query    string `json:"query,omitempty"`
	Reason   string `json:"reason"`
	Status   string `json:"status"`
	Requests int    `json:"requests"`
}

type SearchEvent struct {
	At        string `json:"at"`
	Operation string `json:"operation"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
}

// Transport observations are separate from evidence coverage and identity confidence.
type AccessEvent struct {
	At         string `json:"at"`
	Host       string `json:"host"`
	Outcome    string `json:"outcome"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Reason     string `json:"reason"`
	DurationMS int64  `json:"duration_ms"`
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
