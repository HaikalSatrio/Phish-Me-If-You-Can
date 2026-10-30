package collector

import (
	"context"
	"github.com/HaikalSatrio/Phish-Me-If-You-Can/internal/domain"
)

type Query struct {
	Purpose  string
	Accounts []domain.AccountClue
	Company  string
	Location string
	URL      string
	Platform string
	Name     string
	Username string
	Limit    int
	Budget   int
}

type Profile struct {
	DiscoveryBasis string
	LinkedFrom     string
	FactSources    map[string]FactSource
	ResponseSHA256 string
	Rule           string
	Key            string
	Platform       string
	URL            string
	SourceURL      string
	Facts          []Fact
	ID             int64  `json:"id"`
	Login          string `json:"login"`
	Name           string `json:"name"`
	Company        string `json:"company"`
	Location       string `json:"location"`
	Email          string `json:"email"`
	Blog           string `json:"blog"`
	Type           string `json:"type"`
	Bio            string `json:"bio"`
}

type FactSource struct{ URL, RequestURL, ObservedAt, ResponseSHA256, Rule string }

type Fact struct{ Type, Value string }

type Event struct {
	Operation string
	Status    string
	Detail    string
}

type Result struct {
	Access     []domain.AccessEvent
	Research   []domain.ResearchStep
	Receipts   []domain.CollectionReceipt
	Profiles   []Profile
	Events     []Event
	Requests   int
	Incomplete bool
	Status     string
}

type Collector interface {
	Collect(context.Context, Query) Result
}
