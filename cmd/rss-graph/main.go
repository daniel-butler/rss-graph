package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	case "snapshot":
		return cmdSnapshot(fs, args[1:], dbPath)
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
                  --new         Show recently added feeds (last 30 days)
                  --filter      Filter out common domains
  links <url>   Show links to/from a feed
  import        Import feeds from Miniflux
  crawl         Import and scan all feeds from Miniflux
                  --snapshot    Take a snapshot after crawling
  mentions      Show most-mentioned people/orgs
                  --rising      Sort by velocity (growth rate)
  snapshot      Manage velocity snapshots
                  --list        Show available snapshots
                  --prune       Remove old snapshots (>90 days)
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

// Common domains to filter out when showing rankings
var commonDomains = []string{
	"github.com",
	"twitter.com",
	"x.com",
	"youtube.com",
	"linkedin.com",
	"huggingface.co",
	"news.ycombinator.com",
	"arxiv.org",
	"nytimes.com",
	"openai.com",
	"anthropic.com",
	"google.com",
	"medium.com",
	"substack.com",
	"podcasts.apple.com",
	"scholar.google.com",
	"en.wikipedia.org",
	"reddit.com",
	"facebook.com",
}

func isCommonDomain(feedURL string) bool {
	for _, domain := range commonDomains {
		if strings.Contains(feedURL, domain) {
			return true
		}
	}
	return false
}

func cmdRank(fs *flag.FlagSet, args []string, dbPath *string) error {
	limit := fs.Int("n", 20, "Number of results")
	filterCommon := fs.Bool("filter", false, "Filter out common domains (github, twitter, etc)")
	showNew := fs.Bool("new", false, "Show recently added feeds (last 30 days)")
	newDays := fs.Int("days", 30, "Days to consider 'new' (use with --new)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	g, err := ensureDB(*dbPath)
	if err != nil {
		return err
	}
	defer g.Close()

	// Show new feeds mode
	if *showNew {
		newFeeds, err := g.GetNewFeeds(*newDays, *limit)
		if err != nil {
			return err
		}

		if len(newFeeds) == 0 {
			fmt.Printf("No feeds added in the last %d days.\n", *newDays)
			return nil
		}

		fmt.Printf("🆕 Recently added feeds (last %d days):\n\n", *newDays)
		for i, r := range newFeeds {
			title := r.Feed.Title
			if title == "" {
				title = "(untitled)"
			}
			daysAgo := int(time.Since(r.Feed.CreatedAt).Hours() / 24)
			fmt.Printf("%2d. [%d links] %s\n    %s\n    Added: %d days ago\n\n", 
				i+1, r.InboundCount, title, r.Feed.URL, daysAgo)
		}
		return nil
	}

	// Fetch more results if filtering
	fetchLimit := *limit
	if *filterCommon {
		fetchLimit = *limit * 5
	}

	ranked, err := g.GetMostLinked(fetchLimit)
	if err != nil {
		return err
	}

	if len(ranked) == 0 {
		fmt.Println("No feeds with inbound links yet.")
		return nil
	}

	fmt.Println("Feeds ranked by inbound links:")
	shown := 0
	for _, r := range ranked {
		if shown >= *limit {
			break
		}

		// Skip common domains if filtering
		if *filterCommon && isCommonDomain(r.Feed.URL) {
			continue
		}

		title := r.Feed.Title
		if title == "" {
			title = "(untitled)"
		}
		shown++
		fmt.Printf("%2d. [%d links] %s\n    %s\n", shown, r.InboundCount, title, r.Feed.URL)
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
	takeSnapshot := fs.Bool("snapshot", false, "Take a snapshot after crawling (for velocity tracking)")
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

	// Take snapshot if requested
	if *takeSnapshot {
		today := time.Now().Format("2006-01-02")
		n, err := g.TakeSnapshot(today)
		if err != nil {
			fmt.Printf("Warning: failed to take snapshot: %v\n", err)
		} else {
			fmt.Printf("Snapshot saved: %s (%d entries)\n", today, n)
		}
	}

	return nil
}

func cmdMentions(fs *flag.FlagSet, args []string, dbPath *string) error {
	limit := fs.Int("n", 30, "Number of results")
	entityType := fs.String("type", "PERSON", "Entity type (PERSON, ORG)")
	rising := fs.Bool("rising", false, "Sort by velocity (growth rate)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	g, err := ensureDB(*dbPath)
	if err != nil {
		return err
	}
	defer g.Close()

	if *rising {
		// Get available snapshots
		dates, err := g.GetSnapshotDates()
		if err != nil {
			return err
		}
		
		if len(dates) < 2 {
			fmt.Println("Need at least 2 snapshots for velocity calculation.")
			fmt.Println("Run 'rss-graph snapshot' after each crawl to build history.")
			fmt.Println("\nFalling back to standard ranking...")
			*rising = false
		} else {
			currentDate := dates[0]
			previousDate := dates[1]
			
			risingMentions, err := g.GetRisingMentions(*entityType, currentDate, previousDate, *limit)
			if err != nil {
				return err
			}

			if len(risingMentions) == 0 {
				fmt.Println("No rising mentions found.")
				return nil
			}

			fmt.Printf("Rising stars (%ss gaining momentum):\n", strings.ToLower(*entityType))
			fmt.Printf("Comparing %s vs %s\n\n", currentDate, previousDate)

			// Group by status
			var hot, rising, new_ []graph.RisingMention
			for _, m := range risingMentions {
				switch m.Status {
				case "hot":
					hot = append(hot, m)
				case "rising":
					rising = append(rising, m)
				case "new":
					new_ = append(new_, m)
				}
			}

			if len(hot) > 0 {
				fmt.Println("🔥 HOT")
				for i, m := range hot {
					fmt.Printf("%2d. [+%.0f%%] %s (%d → %d mentions)\n", 
						i+1, m.Velocity*100, m.Name, m.PreviousCount, m.CurrentCount)
				}
				fmt.Println()
			}

			if len(rising) > 0 {
				fmt.Println("📈 RISING")
				for i, m := range rising {
					fmt.Printf("%2d. [+%.0f%%] %s (%d → %d mentions)\n",
						i+1, m.Velocity*100, m.Name, m.PreviousCount, m.CurrentCount)
				}
				fmt.Println()
			}

			if len(new_) > 0 {
				fmt.Println("🆕 NEW (first seen this period)")
				for i, m := range new_ {
					fmt.Printf("%2d. %s (%d mentions)\n", i+1, m.Name, m.CurrentCount)
				}
			}

			return nil
		}
	}

	// Standard ranking
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

func cmdSnapshot(fs *flag.FlagSet, args []string, dbPath *string) error {
	list := fs.Bool("list", false, "Show available snapshots")
	prune := fs.Bool("prune", false, "Remove snapshots older than 90 days")
	if err := fs.Parse(args); err != nil {
		return err
	}

	g, err := ensureDB(*dbPath)
	if err != nil {
		return err
	}
	defer g.Close()

	if *list {
		dates, err := g.GetSnapshotDates()
		if err != nil {
			return err
		}
		if len(dates) == 0 {
			fmt.Println("No snapshots yet. Run 'rss-graph snapshot' to create one.")
			return nil
		}
		fmt.Println("Available snapshots:")
		for _, d := range dates {
			fmt.Printf("  %s\n", d)
		}
		return nil
	}

	if *prune {
		// 90 days ago
		cutoff := time.Now().AddDate(0, 0, -90).Format("2006-01-02")
		n, err := g.PruneSnapshots(cutoff)
		if err != nil {
			return err
		}
		fmt.Printf("Removed %d old snapshot entries (before %s)\n", n, cutoff)
		return nil
	}

	// Take a snapshot
	today := time.Now().Format("2006-01-02")
	n, err := g.TakeSnapshot(today)
	if err != nil {
		return err
	}
	fmt.Printf("Snapshot saved: %s (%d entries)\n", today, n)
	return nil
}
