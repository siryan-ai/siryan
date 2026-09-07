package chatutil

import (
	"regexp"
	"strings"
)

var (
	thinkBlockRe = regexp.MustCompile(`(?is)<think\b[^>]*>.*?</think>`)
	thinkOpenRe  = regexp.MustCompile(`(?is)<think\b[^>]*>.*`)
	// İngilizce CoT başlangıçları
	cotStartRe = regexp.MustCompile(`(?is)^\s*(here'?s a thinking process|thinking process|let me think|internal monologue|step-by-step|analyze user input)\b.*`)
)

func SplitThinking(raw string) (content string, thinking string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}

	// 1) Kapalı <think>...</think> bloklarını thinking'e al
	if matches := thinkBlockRe.FindAllString(raw, -1); len(matches) > 0 {
		thinking = strings.TrimSpace(strings.Join(matches, "\n\n"))
		thinking = stripTags(thinking)
		raw = thinkBlockRe.ReplaceAllString(raw, "")
	}

	// 2) Kapanmamış <think>... varsa tamamını thinking say
	if thinkOpenRe.MatchString(raw) {
		loc := thinkOpenRe.FindStringIndex(raw)
		if loc != nil {
			chunk := strings.TrimSpace(raw[loc[0]:])
			if thinking != "" {
				thinking += "\n\n" + stripTags(chunk)
			} else {
				thinking = stripTags(chunk)
			}
			raw = strings.TrimSpace(raw[:loc[0]])
		}
	}

	raw = strings.TrimSpace(raw)

	// 3) Hâlâ İngilizce CoT ile başlıyorsa: son "temiz" Türkçe/cevap bloğunu bul
	if looksLikeChainOfThought(raw) {
		if thinking == "" {
			thinking = raw
		} else {
			thinking += "\n\n" + raw
		}
		content = extractFinalAnswer(raw)
		return strings.TrimSpace(content), strings.TrimSpace(thinking)
	}

	content = strings.TrimSpace(raw)
	// 4) Content içinde kaçmış etiket kalmasın
	content = thinkBlockRe.ReplaceAllString(content, "")
	content = stripTags(content)

	return strings.TrimSpace(content), strings.TrimSpace(thinking)
}

func stripTags(s string) string {
	s = regexp.MustCompile(`(?is)</?think\b[^>]*>`).ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func looksLikeChainOfThought(s string) bool {
	lower := strings.ToLower(s)
	markers := []string{
		"here's a thinking process",
		"thinking process:",
		"analyze user input",
		"**analyze user input**",
		"apply persona rules",
		"draft - mental",
		"check against rules",
		"internal:",
		"step-by-step",
		"no results found for",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	// Çok uzun + bol İngilizce yapı → CoT ihtimali
	if len(s) > 600 && strings.Count(lower, "constraint") > 0 {
		return true
	}
	return cotStartRe.MatchString(s)
}

// CoT metninin içinden nihai kısa cevabı çıkarmaya çalış
func extractFinalAnswer(s string) string {
	// Sık görülen finale yakın kalıplar
	markers := []string{
		"Refined:",
		"Final:",
		"Final answer:",
		"Output:",
		"[Response Text]",
		"Response:",
		"Cevap:",
		"→",
	}
	lower := s
	for _, m := range markers {
		idx := strings.LastIndex(strings.ToLower(lower), strings.ToLower(m))
		if idx >= 0 {
			part := strings.TrimSpace(s[idx+len(m):])
			part = strings.Split(part, "\n\n")[0]
			part = strings.Trim(part, " \"'\n\r\t")
			if len(part) > 10 && len(part) < 500 && !looksLikeChainOfThought(part) {
				return part
			}
		}
	}

	// Son 1-5 satır, kısa ve CoT değilse
	lines := strings.Split(s, "\n")
	var buf []string
	for i := len(lines) - 1; i >= 0 && len(buf) < 6; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			if len(buf) > 0 {
				break
			}
			continue
		}
		l := strings.ToLower(line)
		if strings.HasPrefix(l, "**") || strings.HasPrefix(l, "- ") ||
			strings.Contains(l, "constraint") || strings.Contains(l, "check against") ||
			strings.Contains(l, "thinking") || strings.Contains(l, "analyze") {
			if len(buf) > 0 {
				break
			}
			continue
		}
		buf = append([]string{line}, buf...)
	}
	out := strings.TrimSpace(strings.Join(buf, " "))
	if len(out) > 15 {
		return out
	}
	return "" // bulunamadı → handler boş cevap hatası verebilir
}
