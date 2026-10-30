package domain

// SocialPost is a view rebuilt from source evidence, not an identity decision.
type SocialPost struct {
	PublisherIDType   string        `json:"publisher_id_type,omitempty"`
	TimestampStatus   string        `json:"timestamp_status,omitempty"`
	EvidenceID        string        `json:"evidence_id"`
	CandidateID       string        `json:"candidate_id"`
	Platform          string        `json:"platform"`
	AccountURL        string        `json:"account_url"`
	PublisherUsername string        `json:"publisher_username"`
	PublisherID       string        `json:"publisher_id"`
	URL               string        `json:"url"`
	PublishedAt       string        `json:"published_at"`
	Caption           *string       `json:"caption"`
	Mentions          []string      `json:"caption_mentions"`
	Hashtags          []string      `json:"caption_hashtags"`
	Clues             []CaptionClue `json:"clues,omitempty"`
	Status            string        `json:"status"`
	Sources           []Source      `json:"sources"`
}

type CaptionClue struct {
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	Quote  string `json:"quote"`
	Status string `json:"status"`
}
