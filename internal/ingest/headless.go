package ingest

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// EnableHeadless turns on the headless-browser fallback for JavaScript-rendered
// pages. Set from config at startup. Requires Chrome/Chromium to be installed;
// when it isn't, fetchHeadless fails gracefully and the HTTP result is kept.
var EnableHeadless bool

// HeadlessWait is how long to let a page's JavaScript run before snapshotting.
var HeadlessWait = 2500 * time.Millisecond

// headlessNavTimeout bounds a single headless render.
const headlessNavTimeout = 45 * time.Second

// fetchHeadless renders rawURL in headless Chrome and extracts text from the
// fully rendered DOM. Returns nil on any failure (caller falls back).
func fetchHeadless(ctx context.Context, rawURL string) *Fetched {
	actx, acancel := chromedp.NewContext(ctx)
	defer acancel()
	tctx, tcancel := context.WithTimeout(actx, headlessNavTimeout)
	defer tcancel()

	var htmlContent string
	err := chromedp.Run(tctx,
		chromedp.Navigate(rawURL),
		chromedp.Sleep(HeadlessWait),
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	)
	if err != nil {
		log.Printf("ingest: headless render of %s failed: %v", rawURL, err)
		return nil
	}
	if htmlContent == "" {
		return nil
	}
	return buildFetched([]byte(htmlContent))
}

// fetchHeadlessWithCookies renders rawURL in headless Chrome after setting the
// given cookies (for authenticated pages). Returns nil on any failure.
func fetchHeadlessWithCookies(ctx context.Context, rawURL string, cookies []*http.Cookie) *Fetched {
	actx, acancel := chromedp.NewContext(ctx)
	defer acancel()
	tctx, tcancel := context.WithTimeout(actx, headlessNavTimeout)
	defer tcancel()

	setCookies := chromedp.ActionFunc(func(ctx context.Context) error {
		for _, c := range cookies {
			if err := network.SetCookie(c.Name, c.Value).WithDomain(c.Domain).WithPath(c.Path).Do(ctx); err != nil {
				return err
			}
		}
		return nil
	})

	var htmlContent string
	err := chromedp.Run(tctx,
		network.Enable(),
		setCookies,
		chromedp.Navigate(rawURL),
		chromedp.Sleep(HeadlessWait),
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	)
	if err != nil || htmlContent == "" {
		log.Printf("ingest: authenticated headless render of %s failed: %v", rawURL, err)
		return nil
	}
	return buildFetched([]byte(htmlContent))
}
