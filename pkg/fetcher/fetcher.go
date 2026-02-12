// Package fetcher provides HTTP fetching of RSS feeds.
package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// Fetcher downloads RSS feeds over HTTP.
type Fetcher struct {
	client    *http.Client
	userAgent string
}

// Option configures a Fetcher.
type Option func(*Fetcher)

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option {
	return func(f *Fetcher) {
		f.client.Timeout = d
	}
}

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) Option {
	return func(f *Fetcher) {
		f.userAgent = ua
	}
}

// New creates a new Fetcher with the given options.
func New(opts ...Option) *Fetcher {
	f := &Fetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: "rss-graph/1.0",
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Fetch downloads the content at the given URL.
func (f *Fetcher) Fetch(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return body, nil
}
