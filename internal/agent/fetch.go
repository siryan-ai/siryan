package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	// Wikipedia → REST API (stabil, hızlı)
	if t, tx, ok := fetchWikipediaAPI(parent, rawURL); ok {
		return t, tx, nil
	}
	return fetchWithChrome(parent, rawURL)
}

func fetchWikipediaAPI(parent context.Context, rawURL string) (title, text string, ok bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", false
	}
	host := strings.ToLower(u.Host)
	if !strings.Contains(host, "wikipedia.org") {
		return "", "", false
	}
	// /wiki/Title
	path := strings.TrimPrefix(u.Path, "/wiki/")
	if path == "" || path == u.Path {
		return "", "", false
	}
	page, _ := url.PathUnescape(path)
	lang := "en"
	if strings.HasPrefix(host, "tr.") {
		lang = "tr"
	} else if i := strings.Index(host, "."); i > 0 {
		// xx.wikipedia.org
		lang = host[:i]
	}

	api := fmt.Sprintf(
		"https://%s.wikipedia.org/api/rest_v1/page/summary/%s",
		lang, url.PathEscape(page),
	)
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return "", "", false
	}
	req.Header.Set("User-Agent", "SiryanResearch/1.0 (siryan-ai)")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", false
	}

	var parsed struct {
		Title       string `json:"title"`
		Extract     string `json:"extract"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", false
	}
	if parsed.Extract == "" {
		return "", "", false
	}
	title = parsed.Title
	text = parsed.Extract
	if parsed.Description != "" {
		text = parsed.Description + ". " + text
	}
	return title, text, true
}

func fetchWithChrome(parent context.Context, rawURL string) (title, text string, err error) {
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()

	allocCtx, allocCancel := chromeAllocator(ctx)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	var body string
	err = chromedp.Run(taskCtx,
		chromedp.Navigate(rawURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(800*time.Millisecond),
		chromedp.Title(&title),
		chromedp.Evaluate(`document.body ? document.body.innerText : ''`, &body),
	)
	if err != nil {
		return "", "", err
	}
	text = compactText(body, 12000)
	text = CleanPageText(text)
	return title, text, nil
}

func compactText(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		return s[:max]
	}
	return s
}
