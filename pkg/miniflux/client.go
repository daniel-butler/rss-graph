// Package miniflux provides a client for the Miniflux RSS reader API.
package miniflux

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a Miniflux API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Feed represents a Miniflux feed subscription.
type Feed struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	FeedURL  string `json:"feed_url"`
	SiteURL  string `json:"site_url"`
	Category struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
	} `json:"category"`
}

// Entry represents a feed entry/post.
type Entry struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
	Author  string `json:"author"`
	FeedID  int64  `json:"feed_id"`
}

// EntriesResponse is the API response for entries.
type EntriesResponse struct {
	Total   int     `json:"total"`
	Entries []Entry `json:"entries"`
}

// NewClient creates a new Miniflux API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetFeeds returns all feed subscriptions.
func (c *Client) GetFeeds() ([]Feed, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/v1/feeds", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var feeds []Feed
	if err := json.NewDecoder(resp.Body).Decode(&feeds); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return feeds, nil
}

// GetEntries returns entries for a specific feed.
func (c *Client) GetEntries(feedID int64, limit int) ([]Entry, error) {
	url := fmt.Sprintf("%s/v1/feeds/%d/entries?limit=%d", c.baseURL, feedID, limit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var response EntriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return response.Entries, nil
}

// GetAllEntries returns all recent entries across all feeds.
func (c *Client) GetAllEntries(limit int) ([]Entry, error) {
	url := fmt.Sprintf("%s/v1/entries?limit=%d&order=published_at&direction=desc", c.baseURL, limit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var response EntriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return response.Entries, nil
}
