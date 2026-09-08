package agent

import (
	"context"
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
	return title, text, nil
}

func compactText(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		return s[:max]
	}
	return s
}
