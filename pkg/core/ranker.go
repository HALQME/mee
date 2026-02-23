package core

import (
	"sort"
	"strings"
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

// Search performs search with parallel execution and trigger-based filtering.
func (r *Ranker) Search(query string) []provider.ResultItem {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.results = r.results[:0]

	if query == "" {
		return r.defaults()
	}

	r.matchTriggers(query)

	// Filter providers based on trigger
	activeProviders := r.filterProviders(query)

	// Parallel search
	results := r.parallelSearch(activeProviders, query)
	r.results = append(r.results, results...)

	sort.Slice(r.results, func(i, j int) bool {
		return r.results[i].Score > r.results[j].Score
	})
	return r.results
}

// filterProviders returns providers that should be active for the given query.
// Providers with triggers are only included if the query starts with their trigger.
// Providers without triggers are always included.
func (r *Ranker) filterProviders(query string) []provider.Provider {
	var active []provider.Provider
	for _, p := range r.providers {
		trigger := p.Trigger()
		// No trigger = always active
		if trigger == "" {
			active = append(active, p)
			continue
		}
		// Has trigger = only active if query starts with trigger
		if strings.HasPrefix(query, trigger) {
			active = append(active, p)
		}
	}
	return active
}

// parallelSearch executes search on multiple providers concurrently.
func (r *Ranker) parallelSearch(providers []provider.Provider, query string) []provider.ResultItem {
	if len(providers) == 0 {
		return nil
	}

	type searchResult struct {
		items []provider.ResultItem
		err   error
	}

	results := make(chan searchResult, len(providers))
	var wg sync.WaitGroup

	for _, p := range providers {
		wg.Add(1)
		go func(prov provider.Provider) {
			defer wg.Done()
			rs, err := prov.Search(query)
			if err != nil || rs == nil {
				results <- searchResult{nil, err}
				return
			}
			results <- searchResult{rs.Items, nil}
		}(p)
	}

	// Wait for all goroutines in separate goroutine
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allItems []provider.ResultItem
	for res := range results {
		if res.items != nil {
			allItems = append(allItems, res.items...)
		}
	}

	return allItems
}

func (r *Ranker) defaults() []provider.ResultItem {
	// Parallel default suggestions
	results := r.parallelDefaults()
	r.results = append(r.results, results...)

	sort.Slice(r.results, func(i, j int) bool {
		return r.results[i].Score > r.results[j].Score
	})
	return r.results
}

// parallelDefaults executes DefaultSuggestions on multiple providers concurrently.
func (r *Ranker) parallelDefaults() []provider.ResultItem {
	type searchResult struct {
		items []provider.ResultItem
		err   error
	}

	results := make(chan searchResult, len(r.providers))
	var wg sync.WaitGroup

	for _, p := range r.providers {
		wg.Add(1)
		go func(prov provider.Provider) {
			defer wg.Done()
			rs, err := prov.DefaultSuggestions()
			if err != nil || rs == nil {
				results <- searchResult{nil, err}
				return
			}
			results <- searchResult{rs.Items, nil}
		}(p)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allItems []provider.ResultItem
	for res := range results {
		if res.items != nil {
			allItems = append(allItems, res.items...)
		}
	}

	return allItems
}

func (r *Ranker) matchTriggers(query string) {
	for _, t := range r.triggers {
		if t.Prefix == "" {
			continue
		}
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
