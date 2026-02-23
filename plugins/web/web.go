package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type ResultItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Action   string `json:"action"`
	Payload  string `json:"payload"`
	Score    int    `json:"score"`
}

type ResultSet struct {
	Items        []ResultItem `json:"items"`
	ProviderName string       `json:"provider_name"`
}

var engines = []struct {
	prefix, name, tmpl string
}{
	{"g:", "Google", "https://www.google.com/search?q="},
	{"gh:", "GitHub", "https://github.com/search?q="},
	{"ddg:", "DuckDuckGo", "https://duckduckgo.com/?q="},
	{"wiki:", "Wikipedia", "https://en.wikipedia.org/wiki/Special:Search?search="},
	{"yt:", "YouTube", "https://www.youtube.com/results?search_query="},
	{"so:", "Stack Overflow", "https://stackoverflow.com/search?q="},
}

func Search(query string) string {
	var results []ResultItem

	for _, e := range engines {
		if strings.HasPrefix(query, e.prefix) {
			term := query[len(e.prefix):]
			if term == "" {
				continue
			}
			u := e.tmpl + url.QueryEscape(term)
			results = append(results, ResultItem{
				ID:       "web:" + e.prefix + term,
				Title:    fmt.Sprintf("Search %s for \"%s\"", e.name, term),
				Subtitle: u,
				Action:   "open",
				Payload:  u,
				Score:    95,
			})
		}
	}

	if len(results) == 0 {
		return ""
	}

	rs := ResultSet{Items: results, ProviderName: "web"}
	data, _ := json.Marshal(rs)
	return string(data)
}
