# RSS Graph

Discover RSS feed relationships by analyzing who links to whom.

## What It Does

RSS Graph builds a social graph of RSS feeds by:
1. Fetching RSS/Atom feeds
2. Extracting outbound links from posts
3. Storing relationships in a SQLite database
4. Ranking feeds by how often they're cited

This helps discover new blogs and feeds by following the link trail from people you already follow.

## Installation

### From Releases

Download the latest binary from [Releases](https://github.com/daniel-butler/rss-graph/releases).

### From Source

```bash
git clone https://github.com/daniel-butler/rss-graph
cd rss-graph
go build -o rss-graph ./cmd/rss-graph
```

## Usage

### Add a Feed

```bash
rss-graph add https://simonwillison.net/atom/everything/
```

### Scan a Feed for Links

Fetch a feed and extract all outbound links to other sites:

```bash
rss-graph scan https://simonwillison.net/atom/everything/
```

### See Most-Linked Feeds

Show feeds ranked by how many other feeds link to them:

```bash
rss-graph rank
```

### Check Link Stats

```bash
rss-graph links https://example.com/
```

## How It Works

1. **Parsing**: Supports RSS 2.0 and Atom feeds
2. **Link Extraction**: Finds all `<a href>` links in post content
3. **Filtering**: Skips internal links (same domain), anchors, javascript:, mailto:
4. **Normalization**: Converts post URLs to root domain for better deduplication
5. **Storage**: SQLite database tracks feeds (nodes) and links (edges)

## Database

By default, the database is stored at `~/.rss-graph/graph.db`. Override with:

```bash
rss-graph -db /path/to/custom.db scan https://example.com/feed.xml
```

## Project Structure

```
rss-graph/
├── cmd/rss-graph/       # CLI entrypoint
├── pkg/
│   ├── extractor/       # HTML link extraction
│   ├── feed/            # RSS/Atom parsing
│   ├── fetcher/         # HTTP client
│   └── graph/           # SQLite graph storage
└── go.mod
```

## Dependencies

- `modernc.org/sqlite` - Pure Go SQLite (no CGO required)

## License

MIT

## Miniflux Integration

If you use [Miniflux](https://miniflux.app/) as your RSS reader, you can import your subscriptions directly:

```bash
export MINIFLUX_URL=https://your-miniflux-instance.com
export MINIFLUX_API_KEY=your-api-key

# Import all feeds
rss-graph import

# Import AND crawl entries for links (builds the graph)
rss-graph crawl
```

Get your API key from Miniflux: Settings → API Keys → Create API Key

### Running Miniflux with Docker

If you don't have Miniflux yet:

```bash
docker run -d \
  --name miniflux \
  -p 8080:8080 \
  -e DATABASE_URL="user=miniflux password=secret dbname=miniflux sslmode=disable" \
  -e RUN_MIGRATIONS=1 \
  -e CREATE_ADMIN=1 \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD=admin123 \
  miniflux/miniflux:latest
```

## Ideas for Future

- [x] Miniflux integration (import existing subscriptions)
- [ ] OPML import/export
- [ ] Web UI for exploring the graph
- [ ] Feed health checks (detect stale feeds)
- [ ] Auto-discovery of RSS URLs from blog homepages
