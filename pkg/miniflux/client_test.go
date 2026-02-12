package miniflux

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_GetFeeds(t *testing.T) {
	// Mock Miniflux API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/feeds" {
			t.Errorf("Expected path /v1/feeds, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Auth-Token") != "test-api-key" {
			t.Error("Missing or wrong API key header")
		}

		feeds := []Feed{
			{ID: 1, Title: "Simon Willison", FeedURL: "https://simonwillison.net/atom/everything/", SiteURL: "https://simonwillison.net/"},
			{ID: 2, Title: "Hamel Husain", FeedURL: "https://hamel.dev/feed.xml", SiteURL: "https://hamel.dev/"},
		}
		json.NewEncoder(w).Encode(feeds)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	feeds, err := client.GetFeeds()
	if err != nil {
		t.Fatalf("GetFeeds error: %v", err)
	}

	if len(feeds) != 2 {
		t.Fatalf("Expected 2 feeds, got %d", len(feeds))
	}
	if feeds[0].Title != "Simon Willison" {
		t.Errorf("Expected first feed title 'Simon Willison', got '%s'", feeds[0].Title)
	}
}

func TestClient_GetEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/feeds/1/entries" {
			t.Errorf("Expected path /v1/feeds/1/entries, got %s", r.URL.Path)
		}

		response := EntriesResponse{
			Total: 2,
			Entries: []Entry{
				{ID: 100, Title: "Post 1", URL: "https://example.com/post1", Content: "<p>Hello <a href=\"https://other.com\">world</a></p>"},
				{ID: 101, Title: "Post 2", URL: "https://example.com/post2", Content: "<p>Another post</p>"},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	entries, err := client.GetEntries(1, 100)
	if err != nil {
		t.Fatalf("GetEntries error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}
}

func TestClient_BadAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error_message": "Access Unauthorized"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-key")
	_, err := client.GetFeeds()
	if err == nil {
		t.Error("Expected error for bad API key")
	}
}

func TestClient_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "key")
	_, err := client.GetFeeds()
	if err == nil {
		t.Error("Expected error for server error")
	}
}
