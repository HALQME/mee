package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
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

func Search(query string) string {
	if !strings.HasPrefix(query, "unix:") {
		return ""
	}

	sub := query[5:]
	now := time.Now()
	var results []ResultItem

	switch {
	case sub == "now" || sub == "":
		ts := now.Unix()
		results = append(results, ResultItem{
			ID: "unix:now", Title: fmt.Sprintf("%d", ts),
			Subtitle: "Current Unix timestamp", Action: "copy",
			Payload: fmt.Sprintf("%d", ts), Score: 100,
		})
	case sub == "date":
		d := now.Format("2006-01-02")
		results = append(results, ResultItem{
			ID: "unix:date", Title: d,
			Subtitle: "Current date", Action: "copy",
			Payload: d, Score: 95,
		})
	case sub == "time":
		t := now.Format("15:04:05")
		results = append(results, ResultItem{
			ID: "unix:time", Title: t,
			Subtitle: "Current time", Action: "copy",
			Payload: t, Score: 95,
		})
	case sub == "iso":
		iso := now.Format(time.RFC3339)
		results = append(results, ResultItem{
			ID: "unix:iso", Title: iso,
			Subtitle: "ISO 8601", Action: "copy",
			Payload: iso, Score: 95,
		})
	default:
		var ts int64
		if _, err := fmt.Sscanf(sub, "%d", &ts); err == nil {
			t := time.Unix(ts, 0)
			s := t.Format("2006-01-02 15:04:05")
			results = append(results, ResultItem{
				ID: "unix:convert", Title: s,
				Subtitle: fmt.Sprintf("Unix %d converted", ts), Action: "copy",
				Payload: s, Score: 100,
			})
		}
	}

	if len(results) == 0 {
		return ""
	}

	rs := ResultSet{Items: results, ProviderName: "unixtime"}
	data, _ := json.Marshal(rs)
	return string(data)
}
