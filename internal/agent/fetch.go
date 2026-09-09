package agent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func chromeAllocator(parent context.Context) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 SiryanResearch/1.0"),
	)
	if p := os.Getenv("CHROME_PATH"); p != "" {
		opts = append(opts, chromedp.ExecPath(p))
	}
	return chromedp.NewExecAllocator(parent, opts...)
}

func FetchPage(parent context.Context, rawURL string) (title, text string, err error) {
	// Önce hızlı HTTP (Chrome'dan çok daha hızlı)
	if t, tx, ok := fetchHTTPFast(parent, rawURL); ok {
		return t, tx, nil
	}
	return fetchWithChrome(parent, rawURL)
}

func fetchHTTPFast(parent context.Context, rawURL string) (title, text string, ok bool) {
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SiryanResearch/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 500_000))
	if err != nil || len(body) < 80 {
		return "", "", false
	}
	html := string(body)
	title = extractHTMLTitle(html)
	text = stripHTMLToText(html)
	text = CleanPageText(compactText(text, 8000))
	if len(text) < 40 {
		return "", "", false
	}
	return title, text, true
}

func extractHTMLTitle(html string) string {
	low := strings.ToLower(html)
	i := strings.Index(low, "<title")
	if i < 0 {
		return ""
	}
	i = strings.Index(html[i:], ">")
	if i < 0 {
		return ""
	}
	start := strings.Index(low, "<title")
	start = start + i + 1
	end := strings.Index(low[start:], "</title>")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(stripTagsSimple(html[start : start+end]))
}

func stripHTMLToText(html string) string {
	// script/style at
	low := html
	for _, tag := range []string{"script", "style", "noscript"} {
		for {
			open := strings.Index(strings.ToLower(low), "<"+tag)
			if open < 0 {
				break
			}
			close := strings.Index(strings.ToLower(low[open:]), "</"+tag+">")
			if close < 0 {
				low = low[:open]
				break
			}
			low = low[:open] + low[open+close+len(tag)+3:]
		}
	}
	return stripTagsSimple(low)
}

func stripTagsSimple(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
			b.WriteByte(' ')
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func fetchWithChrome(parent context.Context, rawURL string) (title, text string, err error) {
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()

	allocCtx, allocCancel := chromeAllocator(ctx)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	var body string
	err = chromedp.Run(taskCtx,
		chromedp.Navigate(rawURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(400*time.Millisecond),
		chromedp.Title(&title),
		chromedp.Evaluate(`document.body ? document.body.innerText : ''`, &body),
	)
	if err != nil {
		return "", "", err
	}
	text = CleanPageText(compactText(body, 8000))
	if text == "" {
		return title, text, fmt.Errorf("empty body")
	}
	return title, text, nil
}

func compactText(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		return s[:max]
	}
	return s
}
