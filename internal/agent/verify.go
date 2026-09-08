// internal/agent/verify.go

package agent

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
