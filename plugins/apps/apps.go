package main

import (
	"encoding/json"
	"path/filepath"
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

var apps []struct{ name, path string }

func init() {
	apps = findApps("/Applications", ".app")
}

func Search(query string) string {
	if query == "" {
		return defaults()
	}

	var results []ResultItem
	q := strings.ToLower(query)

	for _, app := range apps {
		if strings.Contains(strings.ToLower(app.name), q) {
			score := 50
			if strings.HasPrefix(strings.ToLower(app.name), q) {
				score = 80
			}
			results = append(results, ResultItem{
				ID:       "app:" + app.path,
				Title:    app.name,
				Subtitle: app.path,
				Action:   "launch",
				Payload:  app.path,
				Score:    score,
			})
		}
	}

	if len(results) == 0 {
		return ""
	}

	rs := ResultSet{Items: results, ProviderName: "apps"}
	data, _ := json.Marshal(rs)
	return string(data)
}

func defaults() string {
	var results []ResultItem
	for i, app := range apps {
		if i >= 10 {
			break
		}
		results = append(results, ResultItem{
			ID:       "app:" + app.path,
			Title:    app.name,
			Subtitle: app.path,
			Action:   "launch",
			Payload:  app.path,
			Score:    50,
		})
	}

	rs := ResultSet{Items: results, ProviderName: "apps"}
	data, _ := json.Marshal(rs)
	return string(data)
}

func findApps(dir, ext string) []struct{ name, path string } {
	var result []struct{ name, path string }
	files, _ := filepath.Glob(filepath.Join(dir, "*"+ext))
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ext)
		result = append(result, struct{ name, path string }{name, f})
	}
	return result
}
