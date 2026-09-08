// internal/agent/types.go

package agent

type ResearchRequest struct {
	Query   string `json:"query"`
	Locale  string `json:"locale"`
	MaxURLs int    `json:"max_urls"`
}

type Source struct {
	URL     string  `json:"url"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Text    string  `json:"text"`
	Score   float64 `json:"score"`
}

type Claim struct {
	Text       string   `json:"text"`
	Support    []string `json:"support_urls"`
	Conflict   []string `json:"conflict_urls"`
	Confidence float64  `json:"confidence"`
}

type ResearchResult struct {
	Query   string                   `json:"query"`
	Sources []Source                 `json:"sources"`
	Claims  []Claim                  `json:"claims"`
	Blocks  []map[string]interface{} `json:"blocks"`
	Notes   string                   `json:"notes"`
}
