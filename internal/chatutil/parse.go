// internal/chatutil/parse.go
package chatutil

import (
	"regexp"
	"strings"
)

var thinkRe = regexp.MustCompile(`(?s)<think>(.*?)</think>`)

func SplitThinking(raw string) (content string, thinking string) {
	matches := thinkRe.FindStringSubmatch(raw)
	if len(matches) == 2 {
		thinking = strings.TrimSpace(matches[1])
		content = strings.TrimSpace(thinkRe.ReplaceAllString(raw, ""))
		return content, thinking
	}
	return strings.TrimSpace(raw), ""
}
