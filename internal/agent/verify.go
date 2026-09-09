package agent

import (
	"sort"
	"strings"
)

func FilterRelevant(userMessage string, sources []Source) []Source {
	tokens := tokenize(userMessage)
	type scored struct {
		src   Source
		score int
	}
	var ranked []scored
	for _, s := range sources {
		if strings.Contains(strings.ToLower(s.URL), "wikipedia.org") {
			continue
		}
		blob := strings.ToLower(s.Title + " " + s.Snippet + " " + s.Text)
		sc := 0
		for _, t := range tokens {
			if len(t) < 3 {
				continue
			}
			if strings.Contains(blob, t) {
				sc++
			}
		}
		s.Relevant = sc > 0 || len(tokens) == 0
		if !s.Relevant {
			continue
		}
		if s.Score < 0.3 {
			s.Score = 0.35 + float64(sc)*0.05
		}
		ranked = append(ranked, scored{src: s, score: sc})
	}

	if len(ranked) == 0 {
		var fb []Source
		for _, s := range sources {
			if strings.Contains(strings.ToLower(s.URL), "wikipedia.org") {
				continue
			}
			fb = append(fb, s)
			if len(fb) >= 3 {
				break
			}
		}
		if len(fb) == 0 {
			return nil
		}
		return fb
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})
	out := make([]Source, 0, 4)
	for _, r := range ranked {
		out = append(out, r.src)
		if len(out) >= 4 {
			break
		}
	}
	return out
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == '?' || r == '!' || r == '/' || r == '\''
	})
	stop := map[string]bool{
		"ve": true, "ile": true, "bir": true, "bu": true, "şu": true,
		"icin": true, "için": true, "nedir": true, "kimdir": true,
		"ne": true, "mi": true, "mı": true, "mu": true, "mü": true,
	}
	var out []string
	for _, p := range parts {
		if stop[p] || len(p) < 2 {
			continue
		}
		out = append(out, p)
	}
	return out
}

func UniqueDomains(sources []Source) int {
	m := map[string]bool{}
	for _, s := range sources {
		d := DomainOf(s.URL)
		if d != "" {
			m[d] = true
		}
	}
	return len(m)
}

func ScoreByDomains(n int) float64 {
	switch {
	case n >= 3:
		return 0.85
	case n == 2:
		return 0.65
	case n == 1:
		return 0.4
	default:
		return 0.2
	}
}
