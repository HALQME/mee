package core

import (
	"sort"
	"strings"
	"sync"

	"github.com/halqme/mee/pkg/database"
	"github.com/halqme/mee/pkg/provider"
)

// RankingConfig holds ranking weights.
type RankingConfig struct {
	FuzzyScore     float64 // base fuzzy match score (0-100)
	FrequencyBoost float64 // boost from selection frequency (0-50)
	RecencyBoost   float64 // boost from recent selections (0-30)
	TriggerMatch   float64 // boost for trigger matches (0-20)
}

// DefaultRankingConfig returns the default ranking configuration.
func DefaultRankingConfig() RankingConfig {
	return RankingConfig{
		FuzzyScore:     1.0,
		FrequencyBoost: 1.0,
		RecencyBoost:   1.0,
		TriggerMatch:   1.0,
	}
}

// Ranker ranks search results from providers.
type Ranker struct {
	providers []provider.Provider
	triggers  []provider.TriggerInfo
	results   []provider.ResultItem
	mu        sync.RWMutex

	// Optional history store for ranking
	historyStore *database.HistoryStore
	config       RankingConfig
}

// NewRanker creates a ranker.
func NewRanker() *Ranker {
	return &Ranker{
		providers: make([]provider.Provider, 0),
		triggers:  make([]provider.TriggerInfo, 0),
		config:    DefaultRankingConfig(),
	}
}

// NewRankerWithHistory creates a ranker with history support.
func NewRankerWithHistory(db *database.DB) *Ranker {
	r := NewRanker()
	if db != nil {
		r.historyStore = database.NewHistoryStore(db)
	}
	return r
}

// SetHistoryStore sets the history store for ranking.
func (r *Ranker) SetHistoryStore(store *database.HistoryStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.historyStore = store
}

// SetRankingConfig sets the ranking configuration.
func (r *Ranker) SetRankingConfig(cfg RankingConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = cfg
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

	// Apply history-based boosting if available
	if r.historyStore != nil {
		r.applyHistoryBoost(query)
	}

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

// applyHistoryBoost applies boosting based on selection history.
func (r *Ranker) applyHistoryBoost(query string) {
	if r.historyStore == nil {
		return
	}

	// Get frequency boost
	freqBoost, err := r.historyStore.GetScore(query)
	if err != nil {
		return
	}

	// Get recency boost
	recencyBoost, err := r.historyStore.GetRecencyBoost(query)
	if err != nil {
		return
	}

	// Apply boosts to results
	for i := range r.results {
		// Only boost if this result matches the query
		if strings.Contains(strings.ToLower(r.results[i].Title), strings.ToLower(query)) ||
			strings.Contains(strings.ToLower(r.results[i].Subtitle), strings.ToLower(query)) {
			boost := float64(freqBoost)*r.config.FrequencyBoost + float64(recencyBoost)*r.config.RecencyBoost
			newScore := int(r.results[i].Score) + int(boost)
			if newScore > 100 {
				newScore = 100
			}
			r.results[i].Score = uint8(newScore)
		}
	}
}

func (r *Ranker) defaults() []provider.ResultItem {
	// Parallel default suggestions
	results := r.parallelDefaults()
	r.results = append(r.results, results...)

	// Add popular suggestions from history if available
	if r.historyStore != nil {
		r.addPopularFromHistory()
	}

	sort.Slice(r.results, func(i, j int) bool {
		return r.results[i].Score > r.results[j].Score
	})
	return r.results
}

// addPopularFromHistory adds popular items from search history.
func (r *Ranker) addPopularFromHistory() {
	items, err := r.historyStore.GetPopular(5)
	if err != nil || len(items) == 0 {
		return
	}

	for _, item := range items {
		// Check if this item is already in results
		exists := false
		for _, r := range r.results {
			if r.ID == "history:"+item.Query || r.Payload == item.SelectedItem {
				exists = true
				break
			}
		}
		if !exists {
			r.results = append(r.results, provider.ResultItem{
				ID:       "history:" + item.Query,
				Title:    item.Query,
				Subtitle: "Frequently used",
				Action:   "search",
				Payload:  item.Query,
				Score:    uint8(50 + min(50, item.SelectionCount*5)), // 50-100 based on frequency
			})
		}
	}
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
		if len(t.Prefix) >= len(query) && strings.HasPrefix(t.Prefix, query) {
			// Calculate trigger match score
			score := uint8(70 + 20*float64(len(query))/float64(len(t.Prefix)))
			r.results = append(r.results, provider.ResultItem{
				ID:       "trigger:" + t.Prefix,
				Title:    t.Prefix,
				Subtitle: t.Description,
				Action:   "trigger",
				Payload:  t.Prefix,
				Score:    score,
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

// RecordSelection records a selection in history.
func (r *Ranker) RecordSelection(pluginID, query, selectedItem string) {
	if r.historyStore != nil {
		_ = r.historyStore.Record(pluginID, query, selectedItem)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
