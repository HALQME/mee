// Package provider defines the Provider interface and ResultItem types.
package provider

// ResultItem represents a single search result.
type ResultItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Action   string `json:"action"` // "launch", "open", "print", "copy", "trigger"
	Payload  string `json:"payload"`
	Score    uint8  `json:"score"` // 0-100
}

// ResultSet contains search results from a provider.
type ResultSet struct {
	Items        []ResultItem
	ProviderName string
}

// TriggerInfo represents a trigger prefix suggestion.
type TriggerInfo struct {
	Prefix      string
	Description string
}

// Provider is the interface that all search providers must implement.
type Provider interface {
	// Name returns the provider's name.
	Name() string

	// Search performs a search with the given query.
	// Returns nil if the provider doesn't handle this query.
	Search(query string) (*ResultSet, error)

	// DefaultSuggestions returns suggestions when the query is empty.
	// Returns nil if no default suggestions.
	DefaultSuggestions() (*ResultSet, error)
}

// FuzzyMatch performs fuzzy matching and returns a score (0-100).
// Returns 0 if no match.
func FuzzyMatch(query, text string) uint8 {
	if len(query) == 0 {
		return 50
	}
	if len(text) == 0 {
		return 0
	}

	qi := 0
	consecutive := 0
	maxConsecutive := 0
	startsAtZero := false

	for ti, c := range text {
		if qi < len(query) && toLower(byte(c)) == toLower(query[qi]) {
			if qi == 0 && ti == 0 {
				startsAtZero = true
			}
			qi++
			consecutive++
			if consecutive > maxConsecutive {
				maxConsecutive = consecutive
			}
		} else {
			consecutive = 0
		}
	}

	if qi != len(query) {
		return 0
	}

	var score uint8 = 50
	score += min(20, uint8(maxConsecutive*4))
	if startsAtZero {
		score += 15
	}

	// Exact match bonus
	if len(query) == len(text) {
		exactMatch := true
		for i := range query {
			if toLower(query[i]) != toLower(text[i]) {
				exactMatch = false
				break
			}
		}
		if exactMatch {
			score += 20
		}
	}

	// Coverage bonus
	coverage := float64(len(query)) / float64(len(text))
	score += uint8(coverage * 15)

	// Short query bonus
	if len(query) <= 3 {
		score += 10
	} else if len(query) <= 5 {
		score += 5
	}

	return min(100, score)
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}
