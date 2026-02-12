package graph

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewGraph_CreatesDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	g, err := NewGraph(dbPath)
	if err != nil {
		t.Fatalf("NewGraph error: %v", err)
	}
	defer g.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestGraph_AddFeed(t *testing.T) {
	g := newTestGraph(t)
	defer g.Close()

	feed := &FeedNode{
		URL:   "https://simonwillison.net/",
		Title: "Simon Willison's Blog",
	}

	id, err := g.AddFeed(feed)
	if err != nil {
		t.Fatalf("AddFeed error: %v", err)
	}
	if id == 0 {
		t.Error("Expected non-zero ID")
	}
}

func TestGraph_AddFeed_Duplicate(t *testing.T) {
	g := newTestGraph(t)
	defer g.Close()

	feed := &FeedNode{
		URL:   "https://example.com/",
		Title: "Example",
	}

	id1, err := g.AddFeed(feed)
	if err != nil {
		t.Fatalf("First AddFeed error: %v", err)
	}

	// Adding same URL should return existing ID
	id2, err := g.AddFeed(feed)
	if err != nil {
		t.Fatalf("Second AddFeed error: %v", err)
	}

	if id1 != id2 {
		t.Errorf("Expected same ID for duplicate URL, got %d and %d", id1, id2)
	}
}

func TestGraph_GetFeedByURL(t *testing.T) {
	g := newTestGraph(t)
	defer g.Close()

	feed := &FeedNode{
		URL:   "https://hamel.dev/",
		Title: "Hamel's Blog",
	}
	g.AddFeed(feed)

	found, err := g.GetFeedByURL("https://hamel.dev/")
	if err != nil {
		t.Fatalf("GetFeedByURL error: %v", err)
	}
	if found == nil {
		t.Fatal("Expected to find feed")
	}
	if found.Title != "Hamel's Blog" {
		t.Errorf("Expected title 'Hamel's Blog', got '%s'", found.Title)
	}
}

func TestGraph_GetFeedByURL_NotFound(t *testing.T) {
	g := newTestGraph(t)
	defer g.Close()

	found, err := g.GetFeedByURL("https://notfound.example.com/")
	if err != nil {
		t.Fatalf("GetFeedByURL error: %v", err)
	}
	if found != nil {
		t.Error("Expected nil for non-existent URL")
	}
}

func TestGraph_AddLink(t *testing.T) {
	g := newTestGraph(t)
	defer g.Close()

	// Add two feeds
	source := &FeedNode{URL: "https://source.com/", Title: "Source Blog"}
	target := &FeedNode{URL: "https://target.com/", Title: "Target Blog"}

	sourceID, _ := g.AddFeed(source)
	targetID, _ := g.AddFeed(target)

	// Add link between them
	link := &LinkEdge{
		SourceID:  sourceID,
		TargetID:  targetID,
		Context:   "Great article!",
		PostURL:   "https://source.com/post/123",
		PostTitle: "My Recommendations",
	}

	err := g.AddLink(link)
	if err != nil {
		t.Fatalf("AddLink error: %v", err)
	}
}

func TestGraph_GetOutboundLinks(t *testing.T) {
	g := newTestGraph(t)
	defer g.Close()

	// Set up: A links to B and C
	a := &FeedNode{URL: "https://a.com/", Title: "A"}
	b := &FeedNode{URL: "https://b.com/", Title: "B"}
	c := &FeedNode{URL: "https://c.com/", Title: "C"}

	aID, _ := g.AddFeed(a)
	bID, _ := g.AddFeed(b)
	cID, _ := g.AddFeed(c)

	g.AddLink(&LinkEdge{SourceID: aID, TargetID: bID, Context: "A->B"})
	g.AddLink(&LinkEdge{SourceID: aID, TargetID: cID, Context: "A->C"})

	links, err := g.GetOutboundLinks(aID)
	if err != nil {
		t.Fatalf("GetOutboundLinks error: %v", err)
	}

	if len(links) != 2 {
		t.Errorf("Expected 2 outbound links, got %d", len(links))
	}
}

func TestGraph_GetInboundLinks(t *testing.T) {
	g := newTestGraph(t)
	defer g.Close()

	// Set up: A and B both link to C
	a := &FeedNode{URL: "https://a.com/", Title: "A"}
	b := &FeedNode{URL: "https://b.com/", Title: "B"}
	c := &FeedNode{URL: "https://c.com/", Title: "C"}

	aID, _ := g.AddFeed(a)
	bID, _ := g.AddFeed(b)
	cID, _ := g.AddFeed(c)

	g.AddLink(&LinkEdge{SourceID: aID, TargetID: cID, Context: "A->C"})
	g.AddLink(&LinkEdge{SourceID: bID, TargetID: cID, Context: "B->C"})

	links, err := g.GetInboundLinks(cID)
	if err != nil {
		t.Fatalf("GetInboundLinks error: %v", err)
	}

	if len(links) != 2 {
		t.Errorf("Expected 2 inbound links, got %d", len(links))
	}
}

func TestGraph_GetMostLinked(t *testing.T) {
	g := newTestGraph(t)
	defer g.Close()

	// Set up: A links to B, C, D. B also links to D. So D has most inbound.
	a := &FeedNode{URL: "https://a.com/", Title: "A"}
	b := &FeedNode{URL: "https://b.com/", Title: "B"}
	c := &FeedNode{URL: "https://c.com/", Title: "C"}
	d := &FeedNode{URL: "https://d.com/", Title: "D (Popular)"}

	aID, _ := g.AddFeed(a)
	bID, _ := g.AddFeed(b)
	cID, _ := g.AddFeed(c)
	dID, _ := g.AddFeed(d)

	g.AddLink(&LinkEdge{SourceID: aID, TargetID: bID})
	g.AddLink(&LinkEdge{SourceID: aID, TargetID: cID})
	g.AddLink(&LinkEdge{SourceID: aID, TargetID: dID})
	g.AddLink(&LinkEdge{SourceID: bID, TargetID: dID})

	ranked, err := g.GetMostLinked(10)
	if err != nil {
		t.Fatalf("GetMostLinked error: %v", err)
	}

	if len(ranked) == 0 {
		t.Fatal("Expected ranked results")
	}
	if ranked[0].Feed.URL != "https://d.com/" {
		t.Errorf("Expected D to be most linked, got %s", ranked[0].Feed.URL)
	}
	if ranked[0].InboundCount != 2 {
		t.Errorf("Expected 2 inbound links for D, got %d", ranked[0].InboundCount)
	}
}

func TestGraph_LinkTimestamp(t *testing.T) {
	g := newTestGraph(t)
	defer g.Close()

	a := &FeedNode{URL: "https://a.com/", Title: "A"}
	b := &FeedNode{URL: "https://b.com/", Title: "B"}

	aID, _ := g.AddFeed(a)
	bID, _ := g.AddFeed(b)

	g.AddLink(&LinkEdge{SourceID: aID, TargetID: bID})

	links, _ := g.GetOutboundLinks(aID)
	if len(links) == 0 {
		t.Fatal("Expected link")
	}

	// Timestamp should be recent (within last minute)
	// Using a window because SQLite CURRENT_TIMESTAMP is UTC
	if time.Since(links[0].DiscoveredAt) > time.Minute {
		t.Errorf("DiscoveredAt timestamp too old: %v", links[0].DiscoveredAt)
	}
}

// Helper to create in-memory test graph
func newTestGraph(t *testing.T) *Graph {
	t.Helper()
	g, err := NewGraph(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test graph: %v", err)
	}
	return g
}
