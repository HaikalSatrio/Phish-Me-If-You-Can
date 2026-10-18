package collector

import "context"

type Query struct {
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
	Key       string
	Platform  string
	URL       string
	SourceURL string
	Facts     []Fact
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Company   string `json:"company"`
	Location  string `json:"location"`
	Email     string `json:"email"`
	Blog      string `json:"blog"`
	Type      string `json:"type"`
}

type Fact struct{ Type, Value string }

type Event struct {
	Operation string
	Status    string
	Detail    string
}

type Result struct {
	Profiles   []Profile
	Events     []Event
	Requests   int
	Incomplete bool
	Status     string
}

type Collector interface {
	Collect(context.Context, Query) Result
}
