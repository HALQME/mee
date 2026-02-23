package main

import (
	"encoding/json"
	"strings"
)

// ResultItem for JSON output
type ResultItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Action   string `json:"action"`
	Payload  string `json:"payload"`
	Score    int    `json:"score"`
}

// ResultSet for JSON output
type ResultSet struct {
	Items        []ResultItem `json:"items"`
	ProviderName string       `json:"provider_name"`
}

// Emoji data
var emojis = []struct {
	char     string
	name     string
	keywords string
}{
	{"🔥", "fire", "hot lit"},
	{"❤️", "red heart", "love"},
	{"✨", "sparkles", "magic shiny"},
	{"💯", "hundred points", "100 perfect"},
	{"👍", "thumbs up", "yes approve"},
	{"🎉", "party popper", "celebrate confetti"},
	{"😀", "grinning face", "happy smile joy"},
	{"😂", "face with tears of joy", "laugh cry happy"},
	{"😍", "smiling face with heart-eyes", "love adore"},
	{"🤔", "thinking face", "hmm consider"},
}

// Search searches for emojis and returns JSON
func Search(query string) string {
	query = strings.ToLower(strings.TrimPrefix(query, "emoji:"))
	if len(query) == 0 {
		return ""
	}

	var results []ResultItem
	for _, e := range emojis {
		if strings.Contains(e.name, query) || strings.Contains(e.keywords, query) {
			results = append(results, ResultItem{
				ID:       "emoji:" + e.name,
				Title:    e.char,
				Subtitle: e.name,
				Action:   "copy",
				Payload:  e.char,
				Score:    90,
			})
		}
	}

	if len(results) == 0 {
		return ""
	}

	rs := ResultSet{
		Items:        results,
		ProviderName: "emoji",
	}

	data, _ := json.Marshal(rs)
	return string(data)
}

// New creates a new provider instance
func New() interface{} {
	return &EmojiProvider{}
}

// EmojiProvider for duck typing
type EmojiProvider struct{}

func (p *EmojiProvider) Name() string {
	return "emoji"
}

func (p *EmojiProvider) Search(query string) (string, error) {
	return Search(query), nil
}
