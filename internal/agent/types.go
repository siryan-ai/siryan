package agent

type RunRequest struct {
	UserMessage string
	Locale      string
	MaxURLs     int
	UserID      string // Layer A — sonra
}

type SearchHit struct {
	URL     string
	Title   string
	Snippet string
}

type Source struct {
	URL       string  `json:"url"`
	Title     string  `json:"title"`
	Snippet   string  `json:"snippet"`
	Text      string  `json:"text"`
	Score     float64 `json:"score"`
	QueryUsed string  `json:"query_used"`
	Relevant  bool    `json:"relevant"`
}

type Claim struct {
	Text       string   `json:"text"`
	Support    []string `json:"support_urls"`
	Conflict   []string `json:"conflict_urls"`
	Confidence float64  `json:"confidence"`
}

type RunResult struct {
	UsedAgent bool
	Blocks    []map[string]interface{}
	Sources   []Source
	Claims    []Claim
	Notes     string // sadece log / debug — UI'ya gösterme zorunlu değil
	Content   string
}
