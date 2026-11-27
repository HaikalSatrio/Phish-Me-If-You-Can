// Package graph builds the exposure graph: the subject, the accounts linked to
// the subject, and the public facts those accounts expose. A fact observed on
// several accounts is one node with several edges, which makes the combination
// of exposures explicit for later attribution and remediation. The graph only
// reorganises evidence already in a case; it adds no new claims.
package graph

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/identity"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/profile"
	"golang.org/x/text/unicode/norm"
)

const SchemaVersion = "exposure-graph-v1"

// Categories follow the exposure model in Chapter 3; "other" keeps unmapped
// observed types visible instead of dropping them silently.
var Categories = []string{"identity", "contact", "affiliation", "routine", "social", "other"}

var categoryOf = map[string]string{
	"name": "identity", "username": "identity", "profile_display_name": "identity", "alias": "identity", "birth_place": "identity",
	"email": "contact", "phone": "contact", "messaging_account": "contact", "website": "contact",
	"workplace": "affiliation", "role": "affiliation", "employment": "affiliation", "occupation": "affiliation",
	"education": "affiliation", "membership": "affiliation", "public_office": "affiliation", "organization_label": "affiliation",
	"location": "routine", "public_event": "routine", "hobby": "routine", "location_label": "routine", "event_label": "routine", "hashtag": "routine",
	"professional_relationship": "social", "post": "social", "mention": "social",
}

// Discovery metadata supports source review but is not a personal fact, the
// same rule PES applies. Instagram posts enter through Case.SocialPosts.
var metadataTypes = map[string]bool{
	"page_title": true, "page_description": true, "search_title": true, "search_snippet": true, "indexed_url": true,
	"page_text": true, "page_links": true, "profile_links": true, "person_jsonld": true, "profile_description": true,
	"instagram_collection": true, "instagram_post": true,
}

type Options struct {
	Policy             domain.Policy
	IncludeUnconfirmed bool
}

type Node struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	Category        string   `json:"category,omitempty"`
	Type            string   `json:"type,omitempty"`
	Label           string   `json:"label"`
	Platform        string   `json:"platform,omitempty"`
	URL             string   `json:"url,omitempty"`
	Status          string   `json:"status,omitempty"`
	IdentityState   string   `json:"identity_state,omitempty"`
	Confidence      float64  `json:"confidence"`
	Discoverability float64  `json:"discoverability,omitempty"`
	EvidenceIDs     []string `json:"evidence_ids,omitempty"`
	Sources         []string `json:"sources,omitempty"`
	Accounts        []string `json:"accounts,omitempty"`
}

type Edge struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Kind       string  `json:"kind"`
	Confidence float64 `json:"confidence"`
	Basis      string  `json:"basis,omitempty"`
}

type Summary struct {
	Accounts         int            `json:"accounts"`
	ExcludedAccounts int            `json:"excluded_accounts"`
	Facts            int            `json:"facts"`
	FactsByCategory  map[string]int `json:"facts_by_category"`
	SharedFacts      int            `json:"shared_facts"`
	Excluded         map[string]int `json:"excluded"`
}

type Graph struct {
	SchemaVersion string   `json:"schema_version"`
	TargetID      string   `json:"target_id"`
	TargetName    string   `json:"target_name"`
	Scope         string   `json:"scope"`
	Nodes         []Node   `json:"nodes"`
	Edges         []Edge   `json:"edges"`
	Summary       Summary  `json:"summary"`
	Limitations   []string `json:"limitations"`
}

type builder struct {
	g     *Graph
	index map[string]int
	edges map[string]bool
}

func normKey(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

func key(parts ...string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(parts, "\x00"))))[:12]
}

func (b *builder) node(n Node) *Node {
	if i, ok := b.index[n.ID]; ok {
		return &b.g.Nodes[i]
	}
	b.index[n.ID] = len(b.g.Nodes)
	b.g.Nodes = append(b.g.Nodes, n)
	return &b.g.Nodes[len(b.g.Nodes)-1]
}

func (b *builder) edge(e Edge) {
	k := e.From + "\x00" + e.To + "\x00" + e.Kind
	if b.edges[k] {
		return
	}
	b.edges[k] = true
	b.g.Edges = append(b.g.Edges, e)
}

func appendUnique(list []string, values ...string) []string {
	for _, v := range values {
		if v == "" {
			continue
		}
		found := false
		for _, x := range list {
			if x == v {
				found = true
				break
			}
		}
		if !found {
			list = append(list, v)
		}
	}
	return list
}

// fact adds or merges a fact node and links the account to it. Merged nodes keep
// the lowest confidence and highest discoverability of their observations.
func (b *builder) fact(account string, category, typ, value, status string, confidence, discoverability float64, evidenceIDs, sources []string) string {
	id := "fact:" + typ + ":" + key(category, typ, normKey(value))
	n := b.node(Node{ID: id, Kind: "fact", Category: category, Type: typ, Label: value, Status: status, Confidence: confidence, Discoverability: discoverability})
	if confidence < n.Confidence {
		n.Confidence = confidence
	}
	if discoverability > n.Discoverability {
		n.Discoverability = discoverability
	}
	if n.Status != status && status == "UNVERIFIED" {
		n.Status = status
	}
	n.EvidenceIDs = appendUnique(n.EvidenceIDs, evidenceIDs...)
	n.Sources = appendUnique(n.Sources, sources...)
	n.Accounts = appendUnique(n.Accounts, account)
	b.edge(Edge{From: account, To: id, Kind: "exposes", Confidence: confidence})
	return id
}

func sourceURLs(sources []domain.Source) []string {
	out := []string{}
	for _, s := range sources {
		out = appendUnique(out, s.URL)
	}
	return out
}

func platformOf(c domain.Candidate) (string, string) {
	if platform, handle, ok := profile.Account(c.ProfileURL); ok {
		return platform, handle
	}
	if u, err := url.Parse(c.ProfileURL); err == nil && u.Hostname() != "" {
		return "web", u.Hostname()
	}
	return "unknown", ""
}

// Build turns a normalized case into an exposure graph. Rejected identities are
// always excluded; unconfirmed ones only enter with IncludeUnconfirmed and stay
// labelled with their identity state.
func Build(c domain.Case, opt Options) Graph {
	g := Graph{SchemaVersion: SchemaVersion, TargetID: c.Target.ID, TargetName: c.Target.Name, Nodes: []Node{}, Edges: []Edge{}}
	b := &builder{g: &g, index: map[string]int{}, edges: map[string]bool{}}
	g.Summary = Summary{FactsByCategory: map[string]int{}, Excluded: map[string]int{}}
	g.Scope = "Accounts with an accepted identity link only."
	if opt.IncludeUnconfirmed {
		g.Scope = "Accepted accounts plus unconfirmed accounts marked with their identity state; rejected accounts excluded."
	}
	b.node(Node{ID: "subject", Kind: "subject", Label: c.Target.Name, Confidence: 1})

	included := map[string]string{}
	for _, cand := range c.Candidates {
		state := identity.Resolve(cand, opt.Policy)
		if state == "REJECTED" || state != "ACCEPTED" && !opt.IncludeUnconfirmed {
			g.Summary.ExcludedAccounts++
			continue
		}
		platform, handle := platformOf(cand)
		label := cand.DisplayName
		if handle != "" {
			label = platform + ":" + handle
		} else if label == "" {
			label = cand.ID
		}
		id := "account:" + cand.ID
		included[cand.ID] = id
		b.node(Node{ID: id, Kind: "account", Label: label, Platform: platform, URL: cand.ProfileURL, IdentityState: state, Confidence: cand.Confidence})
		b.edge(Edge{From: "subject", To: id, Kind: "identity_link", Confidence: cand.Confidence, Basis: state})
		g.Summary.Accounts++
	}

	for _, e := range c.Evidence {
		account, ok := included[e.CandidateID]
		switch {
		case !ok:
			g.Summary.Excluded["evidence_of_excluded_account"]++
		case e.SupersededBy != "":
			g.Summary.Excluded["superseded_evidence"]++
		case metadataTypes[e.Type]:
			g.Summary.Excluded["discovery_metadata"]++
		case e.Status != "OBSERVED" && e.Status != "INFERRED" || strings.TrimSpace(e.Value) == "":
			g.Summary.Excluded["not_observed"]++
		default:
			category := categoryOf[e.Type]
			if category == "" {
				category = "other"
			}
			b.fact(account, category, e.Type, e.Value, e.Status, e.Confidence, e.Discoverability, []string{e.ID}, sourceURLs(e.Sources))
		}
	}

	evidenceByID := map[string]domain.Evidence{}
	for _, e := range c.Evidence {
		evidenceByID[e.ID] = e
	}
	for _, r := range c.Relations {
		account, ok := included[r.CandidateID]
		if !ok {
			g.Summary.Excluded["relation_of_excluded_account"]++
			continue
		}
		decision := domain.RelationDecision(r)
		if decision == "REJECT" {
			g.Summary.Excluded["rejected_relation"]++
			continue
		}
		label := strings.TrimSpace(strings.Join([]string{r.Role, r.Organization}, " @ "))
		label = strings.Trim(label, " @")
		if label == "" {
			g.Summary.Excluded["empty_relation"]++
			continue
		}
		if r.Temporal != "" {
			label += " (" + r.Temporal + ")"
		}
		status := "UNVERIFIED"
		if decision == "ACCEPT" {
			status = "ACCEPTED"
		}
		category := categoryOf[r.Kind]
		if category == "" {
			category = "affiliation"
		}
		src := evidenceByID[r.EvidenceID]
		confidence := src.Confidence
		if confidence == 0 {
			confidence = 0.5
		}
		b.fact(account, category, r.Kind, label, status, confidence, src.Discoverability, []string{r.EvidenceID}, sourceURLs(src.Sources))
	}

	for _, n := range c.NameClaims {
		if account, ok := included[n.CandidateID]; ok && n.Alias != "" {
			src := evidenceByID[n.EvidenceID]
			b.fact(account, "identity", "alias", n.Alias, "UNVERIFIED", src.Confidence, src.Discoverability, []string{n.EvidenceID}, sourceURLs(src.Sources))
		}
	}

	for _, p := range c.SocialPosts {
		account, ok := included[p.CandidateID]
		if !ok {
			g.Summary.Excluded["post_of_excluded_account"]++
			continue
		}
		label := "post " + p.PublishedAt
		if p.Caption != nil && strings.TrimSpace(*p.Caption) != "" {
			label = truncate(*p.Caption, 40)
		}
		src := evidenceByID[p.EvidenceID]
		postID := b.fact(account, "social", "post", p.URL, "UNVERIFIED", src.Confidence, src.Discoverability, []string{p.EvidenceID}, []string{p.URL})
		b.g.Nodes[b.index[postID]].Label = label
		b.g.Nodes[b.index[postID]].URL = p.URL
		for _, clue := range p.Clues {
			category := categoryOf[clue.Kind]
			if category == "" {
				category = "other"
			}
			clueID := "fact:" + clue.Kind + ":" + key(category, clue.Kind, normKey(clue.Value))
			n := b.node(Node{ID: clueID, Kind: "fact", Category: category, Type: clue.Kind, Label: clue.Value, Status: "UNVERIFIED", Confidence: src.Confidence, Discoverability: src.Discoverability})
			n.EvidenceIDs = appendUnique(n.EvidenceIDs, p.EvidenceID)
			n.Sources = appendUnique(n.Sources, p.URL)
			n.Accounts = appendUnique(n.Accounts, account)
			b.edge(Edge{From: postID, To: clueID, Kind: "reveals", Confidence: src.Confidence, Basis: "caption " + clue.Kind})
		}
	}

	for _, n := range g.Nodes {
		if n.Kind != "fact" {
			continue
		}
		g.Summary.Facts++
		g.Summary.FactsByCategory[n.Category]++
		if len(n.Accounts) > 1 {
			g.Summary.SharedFacts++
		}
	}
	sort.SliceStable(g.Nodes, func(i, j int) bool { return nodeOrder(g.Nodes[i]) < nodeOrder(g.Nodes[j]) })
	sort.SliceStable(g.Edges, func(i, j int) bool {
		if g.Edges[i].From != g.Edges[j].From {
			return g.Edges[i].From < g.Edges[j].From
		}
		return g.Edges[i].To < g.Edges[j].To
	})
	g.Limitations = []string{
		"Fact nodes are public self-descriptions or extracted claims; the graph does not verify them.",
		"A fact shared by several accounts is evidence of combination, not proof that the accounts belong to one person.",
		"Discovery metadata (titles, snippets, page text) is excluded, as in PES.",
	}
	return g
}

func nodeOrder(n Node) string {
	rank := map[string]string{"subject": "0", "account": "1", "fact": "2"}[n.Kind]
	cat := "9"
	for i, c := range Categories {
		if c == n.Category {
			cat = fmt.Sprint(i)
		}
	}
	return rank + cat + n.ID
}

// truncate folds styled Unicode letters (such as mathematical bold) to plain
// letters with NFKC so labels stay readable in figures, then shortens them.
func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(norm.NFKC.String(s)), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
