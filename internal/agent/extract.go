package agent

import (
	"net/url"
	"regexp"
	"strings"
)

var junkPrefixes = []string{
	"Jump to content",
	"Main menu",
	"Toggle the table of contents",
	"From Wikipedia, the free encyclopedia",
}

func DomainOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

func CleanPageText(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	// Wiki nav kırp
	for _, p := range junkPrefixes {
		if i := strings.Index(s, p); i >= 0 && i < 200 {
			// ilk junk'tan sonrası çoğu zaman asıl içerik değil; encyclopedia sonrası al
		}
	}
	if i := strings.Index(s, "From Wikipedia, the free encyclopedia"); i >= 0 {
		s = strings.TrimSpace(s[i+len("From Wikipedia, the free encyclopedia"):])
	}
	// References sonrası kes
	if i := strings.Index(s, " References "); i > 500 {
		s = s[:i]
	}
	if i := strings.Index(s, " External links "); i > 500 {
		s = s[:i]
	}
	// tekrar boşluk
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 8000 {
		s = s[:8000]
	}
	return s
}

func BuildSource(hit SearchHit, pageTitle, pageText string) Source {
	title := pageTitle
	if title == "" {
		title = hit.Title
	}
	text := CleanPageText(pageText)
	snippet := hit.Snippet
	if snippet == "" || strings.HasPrefix(snippet, "Jump to content") {
		if len(text) > 220 {
			snippet = text[:220] + "…"
		} else {
			snippet = text
		}
	} else {
		snippet = CleanPageText(snippet)
		if len(snippet) > 240 {
			snippet = snippet[:240] + "…"
		}
	}
	return Source{
		URL:     hit.URL,
		Title:   title,
		Snippet: snippet,
		Text:    text,
		Score:   0.5,
	}
}

// kullanılmayan import uyarısı olmasın diye
var _ = regexp.MustCompile
