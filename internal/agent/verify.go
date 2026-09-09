package agent

import "strings"

func FilterRelevant(userMessage string, sources []Source) []Source {
	tokens := tokenize(userMessage)
	var out []Source
	for _, s := range sources {
		blob := strings.ToLower(s.Title + " " + s.Snippet + " " + s.Text)
		score := 0
		for _, t := range tokens {
			if len(t) < 3 {
				continue
			}
			if strings.Contains(blob, t) {
				score++
			}
		}
		s.Relevant = score > 0 || len(tokens) == 0
		if s.Relevant {
			if s.Score < 0.3 {
				s.Score = 0.35 + float64(score)*0.05
			}
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		// hepsini eleme — sessizce en azından snippet'lileri tut
		return sources
	}
	return out
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == '?' || r == '!' || r == '/'
	})
	var out []string
	stop := map[string]bool{"ve": true, "ile": true, "bir": true, "bu": true, "şu": true, "icin": true, "için": true}
	for _, p := range parts {
		if stop[p] || len(p) < 2 {
			continue
		}
		out = append(out, p)
	}
	return out
}
