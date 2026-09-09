package chatutil

import (
	"regexp"
	"strings"
)

var (
	thinkBlockRe = regexp.MustCompile(`(?is)<think\b[^>]*>.*?</think>`)
	thinkOpenRe  = regexp.MustCompile(`(?is)<think\b[^>]*>.*`)
	cotStartRe   = regexp.MustCompile(`(?is)^\s*(here'?s a thinking process|thinking process|let me think|internal monologue|step-by-step|analyze user input)\b.*`)
)

func SplitThinking(raw string) (content string, thinking string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}

	if matches := thinkBlockRe.FindAllString(raw, -1); len(matches) > 0 {
		thinking = strings.TrimSpace(strings.Join(matches, "\n\n"))
		thinking = stripTags(thinking)
		raw = thinkBlockRe.ReplaceAllString(raw, "")
	}

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
		"drafting response",
		"revised draft",
		"check against rules",
		"response strategy:",
		"json output only",
		"internal:",
		"step-by-step",
		"no results found for",
		"let me draft",
	}
	hits := 0
	for _, m := range markers {
		if strings.Contains(lower, m) {
			hits++
		}
	}
	if hits >= 1 {
		return true
	}
	if len(s) > 600 && strings.Count(lower, "constraint") > 0 {
		return true
	}
	if strings.Count(lower, "kim olduğunu doğrulayamam") > 2 {
		return true
	}
	return cotStartRe.MatchString(s)
}

func extractFinalAnswer(s string) string {
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
	for _, m := range markers {
		idx := strings.LastIndex(strings.ToLower(s), strings.ToLower(m))
		if idx >= 0 {
			part := strings.TrimSpace(s[idx+len(m):])
			part = strings.Split(part, "\n\n")[0]
			part = strings.Trim(part, " \"'\n\r\t")
			if len(part) > 10 && len(part) < 800 && !looksLikeChainOfThought(part) {
				return part
			}
		}
	}

	lines := strings.Split(s, "\n")
	var buf []string
	for i := len(lines) - 1; i >= 0 && len(buf) < 8; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			if len(buf) > 0 {
				break
			}
			continue
		}
		l := strings.ToLower(line)
		if strings.Contains(l, "constraint") || strings.Contains(l, "check against") ||
			strings.Contains(l, "thinking") || strings.Contains(l, "analyze") ||
			strings.Contains(l, "draft") || strings.Contains(l, "persona") {
			if len(buf) > 0 {
				break
			}
			continue
		}
		buf = append([]string{line}, buf...)
	}
	out := strings.TrimSpace(strings.Join(buf, " "))
	if len(out) > 15 && !looksLikeChainOfThought(out) {
		return out
	}
	return ""
}
