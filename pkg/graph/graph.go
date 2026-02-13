// Package graph provides a SQLite-backed graph for tracking RSS feed relationships.
package graph

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

// Graph represents the RSS feed relationship graph.
type Graph struct {
	db *sql.DB
}

// FeedNode represents a feed in the graph.
type FeedNode struct {
	ID        int64
	URL       string
	Title     string
	CreatedAt time.Time
}

// LinkEdge represents a link from one feed to another.
type LinkEdge struct {
	ID           int64
	SourceID     int64
	TargetID     int64
	Context      string // Snippet of text around the link
	PostURL      string // URL of the post containing the link
	PostTitle    string // Title of the post
	DiscoveredAt time.Time
}

// RankedFeed represents a feed with its link count.
type RankedFeed struct {
	Feed         *FeedNode
	InboundCount int
}

// Mention represents a person/org mentioned in a feed post.
type Mention struct {
	ID           int64
	SourceID     int64  // Feed that contains the mention
	Name         string // Normalized name
	EntityType   string // PERSON, ORG, etc.
	Context      string // Surrounding text
	PostURL      string
	PostTitle    string
	DiscoveredAt time.Time
}

// RankedMention represents a name with mention count.
type RankedMention struct {
	Name         string
	EntityType   string
	MentionCount int
}

// NewGraph creates or opens a graph database.
func NewGraph(dbPath string) (*Graph, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	g := &Graph{db: db}
	if err := g.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return g, nil
}

// Close closes the database connection.
func (g *Graph) Close() error {
	return g.db.Close()
}

func (g *Graph) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS feeds (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT UNIQUE NOT NULL,
			title TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_id INTEGER NOT NULL,
			target_id INTEGER NOT NULL,
			context TEXT,
			post_url TEXT,
			post_title TEXT,
			discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (source_id) REFERENCES feeds(id),
			FOREIGN KEY (target_id) REFERENCES feeds(id),
			UNIQUE(source_id, target_id, post_url)
		);

		CREATE INDEX IF NOT EXISTS idx_links_source ON links(source_id);
		CREATE INDEX IF NOT EXISTS idx_links_target ON links(target_id);

		CREATE TABLE IF NOT EXISTS mentions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			context TEXT,
			post_url TEXT,
			post_title TEXT,
			discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (source_id) REFERENCES feeds(id),
			UNIQUE(source_id, name, post_url)
		);

		CREATE INDEX IF NOT EXISTS idx_mentions_name ON mentions(name);
		CREATE INDEX IF NOT EXISTS idx_mentions_source ON mentions(source_id);
	`
	_, err := g.db.Exec(schema)
	return err
}

// AddFeed adds a feed to the graph, returning its ID.
// If the feed already exists (by URL), returns the existing ID.
func (g *Graph) AddFeed(feed *FeedNode) (int64, error) {
	// Try to get existing
	existing, err := g.GetFeedByURL(feed.URL)
	if err != nil {
		return 0, err
	}
	if existing != nil {
		return existing.ID, nil
	}

	// Insert new
	result, err := g.db.Exec(
		"INSERT INTO feeds (url, title) VALUES (?, ?)",
		feed.URL, feed.Title,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetFeedByURL retrieves a feed by its URL.
func (g *Graph) GetFeedByURL(url string) (*FeedNode, error) {
	row := g.db.QueryRow(
		"SELECT id, url, title, created_at FROM feeds WHERE url = ?",
		url,
	)

	feed := &FeedNode{}
	err := row.Scan(&feed.ID, &feed.URL, &feed.Title, &feed.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return feed, nil
}

// AddLink adds a link between two feeds.
func (g *Graph) AddLink(link *LinkEdge) error {
	_, err := g.db.Exec(
		`INSERT OR IGNORE INTO links (source_id, target_id, context, post_url, post_title)
		 VALUES (?, ?, ?, ?, ?)`,
		link.SourceID, link.TargetID, link.Context, link.PostURL, link.PostTitle,
	)
	return err
}

// GetOutboundLinks gets all links from a feed.
func (g *Graph) GetOutboundLinks(feedID int64) ([]LinkEdge, error) {
	rows, err := g.db.Query(
		`SELECT id, source_id, target_id, context, post_url, post_title, discovered_at
		 FROM links WHERE source_id = ?`,
		feedID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLinks(rows)
}

// GetInboundLinks gets all links to a feed.
func (g *Graph) GetInboundLinks(feedID int64) ([]LinkEdge, error) {
	rows, err := g.db.Query(
		`SELECT id, source_id, target_id, context, post_url, post_title, discovered_at
		 FROM links WHERE target_id = ?`,
		feedID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLinks(rows)
}

// GetMostLinked returns feeds ranked by inbound link count.
func (g *Graph) GetMostLinked(limit int) ([]RankedFeed, error) {
	rows, err := g.db.Query(
		`SELECT f.id, f.url, f.title, f.created_at, COUNT(l.id) as link_count
		 FROM feeds f
		 LEFT JOIN links l ON f.id = l.target_id
		 GROUP BY f.id
		 HAVING link_count > 0
		 ORDER BY link_count DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RankedFeed
	for rows.Next() {
		feed := &FeedNode{}
		var count int
		if err := rows.Scan(&feed.ID, &feed.URL, &feed.Title, &feed.CreatedAt, &count); err != nil {
			return nil, err
		}
		results = append(results, RankedFeed{Feed: feed, InboundCount: count})
	}
	return results, rows.Err()
}

func scanLinks(rows *sql.Rows) ([]LinkEdge, error) {
	var links []LinkEdge
	for rows.Next() {
		var link LinkEdge
		var postURL, postTitle, context sql.NullString
		if err := rows.Scan(&link.ID, &link.SourceID, &link.TargetID, &context, &postURL, &postTitle, &link.DiscoveredAt); err != nil {
			return nil, err
		}
		link.Context = context.String
		link.PostURL = postURL.String
		link.PostTitle = postTitle.String
		links = append(links, link)
	}
	return links, rows.Err()
}

// AddMention adds a mention to the graph.
func (g *Graph) AddMention(mention *Mention) error {
	_, err := g.db.Exec(
		`INSERT OR IGNORE INTO mentions (source_id, name, entity_type, context, post_url, post_title)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		mention.SourceID, mention.Name, mention.EntityType, mention.Context, mention.PostURL, mention.PostTitle,
	)
	return err
}

// GetMostMentioned returns names ranked by mention count.
func (g *Graph) GetMostMentioned(entityType string, limit int) ([]RankedMention, error) {
	query := `SELECT name, entity_type, COUNT(*) as mention_count
		 FROM mentions
		 WHERE entity_type = ?
		 GROUP BY name
		 ORDER BY mention_count DESC
		 LIMIT ?`

	rows, err := g.db.Query(query, entityType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RankedMention
	for rows.Next() {
		var r RankedMention
		if err := rows.Scan(&r.Name, &r.EntityType, &r.MentionCount); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetMentionsByFeed returns all mentions from a specific feed.
func (g *Graph) GetMentionsByFeed(feedID int64) ([]Mention, error) {
	rows, err := g.db.Query(
		`SELECT id, source_id, name, entity_type, context, post_url, post_title, discovered_at
		 FROM mentions WHERE source_id = ?`,
		feedID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mentions []Mention
	for rows.Next() {
		var m Mention
		var context, postURL, postTitle sql.NullString
		if err := rows.Scan(&m.ID, &m.SourceID, &m.Name, &m.EntityType, &context, &postURL, &postTitle, &m.DiscoveredAt); err != nil {
			return nil, err
		}
		m.Context = context.String
		m.PostURL = postURL.String
		m.PostTitle = postTitle.String
		mentions = append(mentions, m)
	}
	return mentions, rows.Err()
}
