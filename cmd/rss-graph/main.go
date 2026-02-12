package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/daniel-butler/rss-graph/pkg/extractor"
	"github.com/daniel-butler/rss-graph/pkg/feed"
	"github.com/daniel-butler/rss-graph/pkg/fetcher"
	"github.com/daniel-butler/rss-graph/pkg/graph"
)

var Version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("rss-graph", flag.ContinueOnError)

	dbPath := fs.String("db", defaultDBPath(), "Path to SQLite database")
	_ = fs.Bool("version", false, "Show version")

	// Subcommands
	if len(args) == 0 {
		printUsage()
		return nil
	}

	cmd := args[0]

	// Handle version flag specially
	if cmd == "-version" || cmd == "--version" {
		fmt.Println(Version)
		return nil
	}

	switch cmd {
	case "add":
		return cmdAdd(fs, args[1:], dbPath)
	case "scan":
		return cmdScan(fs, args[1:], dbPath)
	case "rank":
		return cmdRank(fs, args[1:], dbPath)
	case "links":
		return cmdLinks(fs, args[1:], dbPath)
	case "version":
		fmt.Println(Version)
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func printUsage() {
	fmt.Println(`rss-graph - Discover RSS feed relationships

Commands:
  add <url>     Add a feed to the graph
  scan <url>    Fetch feed and extract outbound links
  rank          Show feeds ranked by inbound links
  links <url>   Show links to/from a feed
  version       Show version
  help          Show this help

Options:
  -db <path>    SQLite database path (default: ~/.rss-graph/graph.db)`)
}

func defaultDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".rss-graph", "graph.db")
}

func ensureDB(path string) (*graph.Graph, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}
	return graph.NewGraph(path)
}

func cmdAdd(fs *flag.FlagSet, args []string, dbPath *string) error {
	title := fs.String("title", "", "Feed title (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: rss-graph add <url>")
	}
	feedURL := fs.Arg(0)

	g, err := ensureDB(*dbPath)
	if err != nil {
		return err
	}
	defer g.Close()

	id, err := g.AddFeed(&graph.FeedNode{
		URL:   feedURL,
		Title: *title,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Added feed %s (id: %d)\n", feedURL, id)
	return nil
}

func cmdScan(fs *flag.FlagSet, args []string, dbPath *string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: rss-graph scan <url>")
	}
	feedURL := fs.Arg(0)

	g, err := ensureDB(*dbPath)
	if err != nil {
		return err
	}
	defer g.Close()

	// Fetch the feed
	f := fetcher.New()
	data, err := f.Fetch(feedURL)
	if err != nil {
		return fmt.Errorf("fetching feed: %w", err)
	}

	// Parse it
	parsed, err := feed.ParseFeed(data)
	if err != nil {
		return fmt.Errorf("parsing feed: %w", err)
	}

	// Add/update source feed
	sourceID, err := g.AddFeed(&graph.FeedNode{
		URL:   feedURL,
		Title: parsed.Title,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Scanning: %s (%d items)\n", parsed.Title, len(parsed.Items))

	// Process each item
	var totalLinks int
	for _, item := range parsed.Items {
		for _, link := range item.ExtractedLinks {
			// Skip links to same domain (internal links)
			if isSameDomain(feedURL, link.URL) {
				continue
			}

			// Try to identify if this is a blog/feed URL
			targetURL := normalizeToFeedURL(link.URL)

			// Add target as a potential feed
			targetID, err := g.AddFeed(&graph.FeedNode{
				URL:   targetURL,
				Title: link.Text,
			})
			if err != nil {
				continue
			}

			// Add the link
			err = g.AddLink(&graph.LinkEdge{
				SourceID:  sourceID,
				TargetID:  targetID,
				Context:   link.Text,
				PostURL:   item.URL,
				PostTitle: item.Title,
			})
			if err == nil {
				totalLinks++
			}
		}
	}

	fmt.Printf("Found %d outbound links to other sites\n", totalLinks)
	return nil
}

func cmdRank(fs *flag.FlagSet, args []string, dbPath *string) error {
	limit := fs.Int("n", 20, "Number of results")
	if err := fs.Parse(args); err != nil {
		return err
	}

	g, err := ensureDB(*dbPath)
	if err != nil {
		return err
	}
	defer g.Close()

	ranked, err := g.GetMostLinked(*limit)
	if err != nil {
		return err
	}

	if len(ranked) == 0 {
		fmt.Println("No feeds with inbound links yet.")
		return nil
	}

	fmt.Println("Feeds ranked by inbound links:")
	for i, r := range ranked {
		title := r.Feed.Title
		if title == "" {
			title = "(untitled)"
		}
		fmt.Printf("%2d. [%d links] %s\n    %s\n", i+1, r.InboundCount, title, r.Feed.URL)
	}
	return nil
}

func cmdLinks(fs *flag.FlagSet, args []string, dbPath *string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: rss-graph links <url>")
	}
	feedURL := fs.Arg(0)

	g, err := ensureDB(*dbPath)
	if err != nil {
		return err
	}
	defer g.Close()

	feedNode, err := g.GetFeedByURL(feedURL)
	if err != nil {
		return err
	}
	if feedNode == nil {
		return fmt.Errorf("feed not found: %s", feedURL)
	}

	inbound, _ := g.GetInboundLinks(feedNode.ID)
	outbound, _ := g.GetOutboundLinks(feedNode.ID)

	fmt.Printf("Feed: %s\n", feedURL)
	fmt.Printf("Inbound links: %d\n", len(inbound))
	fmt.Printf("Outbound links: %d\n", len(outbound))

	return nil
}

// Helper functions

func isSameDomain(url1, url2 string) bool {
	u1, err1 := url.Parse(url1)
	u2, err2 := url.Parse(url2)
	if err1 != nil || err2 != nil {
		return false
	}
	return u1.Host == u2.Host
}

func normalizeToFeedURL(rawURL string) string {
	// Remove fragments and query params for normalization
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Fragment = ""
	u.RawQuery = ""

	// If it's a specific post URL, try to get the root
	// e.g., https://blog.example.com/2024/01/post -> https://blog.example.com/
	path := u.Path
	if strings.Count(path, "/") > 2 {
		u.Path = "/"
	}

	return strings.TrimSuffix(u.String(), "/") + "/"
}

// Ensure extractor is imported (used by feed package)
var _ = extractor.Link{}
