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
	"github.com/daniel-butler/rss-graph/pkg/miniflux"
	"github.com/daniel-butler/rss-graph/pkg/ner"
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
	case "import":
		return cmdImport(fs, args[1:], dbPath)
	case "crawl":
		return cmdCrawl(fs, args[1:], dbPath)
	case "mentions":
		return cmdMentions(fs, args[1:], dbPath)
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
  import        Import feeds from Miniflux
  crawl         Import and scan all feeds from Miniflux
  mentions      Show most-mentioned people/orgs
  version       Show version
  help          Show this help

Options:
  -db <path>    SQLite database path (default: ~/.rss-graph/graph.db)

Environment:
  MINIFLUX_URL      Miniflux server URL
  MINIFLUX_API_KEY  Miniflux API key`)
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

func cmdImport(fs *flag.FlagSet, args []string, dbPath *string) error {
	minifluxURL := fs.String("url", os.Getenv("MINIFLUX_URL"), "Miniflux server URL")
	apiKey := fs.String("api-key", os.Getenv("MINIFLUX_API_KEY"), "Miniflux API key")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *minifluxURL == "" || *apiKey == "" {
		return fmt.Errorf("MINIFLUX_URL and MINIFLUX_API_KEY required (env or flags)")
	}

	g, err := ensureDB(*dbPath)
	if err != nil {
		return err
	}
	defer g.Close()

	client := miniflux.NewClient(*minifluxURL, *apiKey)
	feeds, err := client.GetFeeds()
	if err != nil {
		return fmt.Errorf("fetching feeds from Miniflux: %w", err)
	}

	fmt.Printf("Importing %d feeds from Miniflux...\n", len(feeds))
	for _, f := range feeds {
		_, err := g.AddFeed(&graph.FeedNode{
			URL:   f.FeedURL,
			Title: f.Title,
		})
		if err != nil {
			fmt.Printf("  Warning: failed to add %s: %v\n", f.FeedURL, err)
			continue
		}
		fmt.Printf("  + %s\n", f.Title)
	}

	fmt.Printf("Imported %d feeds.\n", len(feeds))
	return nil
}

func cmdCrawl(fs *flag.FlagSet, args []string, dbPath *string) error {
	minifluxURL := fs.String("url", os.Getenv("MINIFLUX_URL"), "Miniflux server URL")
	apiKey := fs.String("api-key", os.Getenv("MINIFLUX_API_KEY"), "Miniflux API key")
	entriesPerFeed := fs.Int("entries", 50, "Entries to scan per feed")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *minifluxURL == "" || *apiKey == "" {
		return fmt.Errorf("MINIFLUX_URL and MINIFLUX_API_KEY required (env or flags)")
	}

	g, err := ensureDB(*dbPath)
	if err != nil {
		return err
	}
	defer g.Close()

	client := miniflux.NewClient(*minifluxURL, *apiKey)
	feeds, err := client.GetFeeds()
	if err != nil {
		return fmt.Errorf("fetching feeds from Miniflux: %w", err)
	}

	fmt.Printf("Crawling %d feeds from Miniflux...\n\n", len(feeds))

	var totalLinks, totalMentions int
	for _, mf := range feeds {
		// Add source feed
		sourceID, err := g.AddFeed(&graph.FeedNode{
			URL:   mf.FeedURL,
			Title: mf.Title,
		})
		if err != nil {
			continue
		}

		// Get entries from Miniflux (already fetched, no need to re-fetch)
		entries, err := client.GetEntries(mf.ID, *entriesPerFeed)
		if err != nil {
			fmt.Printf("  Warning: failed to get entries for %s: %v\n", mf.Title, err)
			continue
		}

		feedLinks := 0
		feedMentions := 0
		for _, entry := range entries {
			// Extract links from entry content
			links := extractor.ExtractLinks(entry.Content)
			for _, link := range links {
				if isSameDomain(mf.SiteURL, link.URL) {
					continue
				}

				targetURL := normalizeToFeedURL(link.URL)
				targetID, err := g.AddFeed(&graph.FeedNode{
					URL:   targetURL,
					Title: link.Text,
				})
				if err != nil {
					continue
				}

				err = g.AddLink(&graph.LinkEdge{
					SourceID:  sourceID,
					TargetID:  targetID,
					Context:   link.Text,
					PostURL:   entry.URL,
					PostTitle: entry.Title,
				})
				if err == nil {
					feedLinks++
				}
			}

			// Extract people mentions using NER
			people := ner.ExtractPeople(entry.Content)
			for _, name := range people {
				err := g.AddMention(&graph.Mention{
					SourceID:   sourceID,
					Name:       name,
					EntityType: "PERSON",
					PostURL:    entry.URL,
					PostTitle:  entry.Title,
				})
				if err == nil {
					feedMentions++
				}
			}
		}
		totalLinks += feedLinks
		totalMentions += feedMentions
		fmt.Printf("  %s: %d entries, %d links, %d mentions\n", mf.Title, len(entries), feedLinks, feedMentions)
	}

	fmt.Printf("\nTotal: %d feeds crawled, %d outbound links, %d people mentions\n", len(feeds), totalLinks, totalMentions)
	return nil
}

func cmdMentions(fs *flag.FlagSet, args []string, dbPath *string) error {
	limit := fs.Int("n", 30, "Number of results")
	entityType := fs.String("type", "PERSON", "Entity type (PERSON, ORG)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	g, err := ensureDB(*dbPath)
	if err != nil {
		return err
	}
	defer g.Close()

	mentions, err := g.GetMostMentioned(*entityType, *limit)
	if err != nil {
		return err
	}

	if len(mentions) == 0 {
		fmt.Println("No mentions found. Run 'crawl' first to extract mentions.")
		return nil
	}

	fmt.Printf("Most mentioned %ss:\n", strings.ToLower(*entityType))
	for i, m := range mentions {
		fmt.Printf("%2d. [%d mentions] %s\n", i+1, m.MentionCount, m.Name)
	}
	return nil
}
