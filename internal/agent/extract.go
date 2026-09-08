// internal/agent/extract.go

package agent

import "net/url"

func DomainOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

func BuildSource(hit SearchHit, pageTitle, pageText string) Source {
	title := pageTitle
	if title == "" {
		title = hit.Title
	}
	snippet := hit.Snippet
	if snippet == "" && len(pageText) > 0 {
		if len(pageText) > 240 {
			snippet = pageText[:240] + "…"
		} else {
			snippet = pageText
		}
	}
	return Source{
		URL:     hit.URL,
		Title:   title,
		Snippet: snippet,
		Text:    pageText,
		Score:   0.5,
	}
}
