package core

import (
	"sort"
	"sync"

	"github.com/halqme/mee/pkg/provider"
)

// Ranker ranks search results from providers.
type Ranker struct {
	providers []provider.Provider
	triggers  []provider.TriggerInfo
	results   []provider.ResultItem
	mu        sync.RWMutex
}

// NewRanker creates a ranker.
func NewRanker() *Ranker {
	return &Ranker{
		providers: make([]provider.Provider, 0),
		triggers:  make([]provider.TriggerInfo, 0),
	}
}

// AddProvider adds a provider.
func (r *Ranker) AddProvider(p provider.Provider) {
	r.mu.Lock()
	r.providers = append(r.providers, p)
	r.mu.Unlock()
}

// AddTrigger adds a trigger.
func (r *Ranker) AddTrigger(t provider.TriggerInfo) {
	r.mu.Lock()
	r.triggers = append(r.triggers, t)
	r.mu.Unlock()
}

// Search performs search.
func (r *Ranker) Search(query string) []provider.ResultItem {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.results = r.results[:0]

	if query == "" {
		return r.defaults()
	}

	r.matchTriggers(query)

	for _, p := range r.providers {
		if rs, err := p.Search(query); err == nil && rs != nil {
			r.results = append(r.results, rs.Items...)
		}
	}

	sort.Slice(r.results, func(i, j int) bool {
		return r.results[i].Score > r.results[j].Score
	})
	return r.results
}

func (r *Ranker) defaults() []provider.ResultItem {
	for _, p := range r.providers {
		if rs, err := p.DefaultSuggestions(); err == nil && rs != nil {
			r.results = append(r.results, rs.Items...)
		}
	}
	sort.Slice(r.results, func(i, j int) bool {
		return r.results[i].Score > r.results[j].Score
	})
	return r.results
}

func (r *Ranker) matchTriggers(query string) {
	for _, t := range r.triggers {
		if len(t.Prefix) > len(query) && t.Prefix[:len(query)] == query {
			r.results = append(r.results, provider.ResultItem{
				ID:       "trigger:" + t.Prefix,
				Title:    t.Prefix,
				Subtitle: t.Description,
				Action:   "trigger",
				Payload:  t.Prefix,
				Score:    90,
			})
		}
	}
}

// Limit limits results.
func (r *Ranker) Limit(max int) {
	r.mu.Lock()
	if len(r.results) > max {
		r.results = r.results[:max]
	}
	r.mu.Unlock()
}

// Results returns current results.
func (r *Ranker) Results() []provider.ResultItem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.results
}
