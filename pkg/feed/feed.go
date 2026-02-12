// Package feed provides RSS and Atom feed parsing.
package feed

import (
	"encoding/xml"
	"errors"
	"html"
	"strings"

	"github.com/daniel-butler/rss-graph/pkg/extractor"
)

// Feed represents a parsed RSS or Atom feed.
type Feed struct {
	Title string
	URL   string
	Items []Item
}

// Item represents a single entry in a feed.
type Item struct {
	Title          string
	URL            string
	Description    string
	Content        string
	ExtractedLinks []extractor.Link
}

// RSS 2.0 structures
type rss2Feed struct {
	XMLName xml.Name    `xml:"rss"`
	Channel rss2Channel `xml:"channel"`
}

type rss2Channel struct {
	Title       string     `xml:"title"`
	Link        string     `xml:"link"`
	Description string     `xml:"description"`
	Items       []rss2Item `xml:"item"`
}

type rss2Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Content     string `xml:"encoded"`
}

// Atom structures
type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Title   string      `xml:"title"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type atomEntry struct {
	Title   string     `xml:"title"`
	Links   []atomLink `xml:"link"`
	Content string     `xml:"content"`
	Summary string     `xml:"summary"`
}

// ParseFeed parses RSS 2.0 or Atom feed data.
func ParseFeed(data []byte) (*Feed, error) {
	if len(data) == 0 {
		return nil, errors.New("empty feed data")
	}

	// Try RSS 2.0 first
	var rss rss2Feed
	if err := xml.Unmarshal(data, &rss); err == nil && rss.Channel.Title != "" {
		return parseRSS2(&rss), nil
	}

	// Try Atom
	var atom atomFeed
	if err := xml.Unmarshal(data, &atom); err == nil && atom.Title != "" {
		return parseAtom(&atom), nil
	}

	return nil, errors.New("unable to parse feed as RSS or Atom")
}

func parseRSS2(rss *rss2Feed) *Feed {
	feed := &Feed{
		Title: rss.Channel.Title,
		URL:   rss.Channel.Link,
		Items: make([]Item, 0, len(rss.Channel.Items)),
	}

	for _, item := range rss.Channel.Items {
		content := item.Content
		if content == "" {
			content = item.Description
		}

		// Decode HTML entities in content
		decodedContent := html.UnescapeString(content)

		feedItem := Item{
			Title:          item.Title,
			URL:            item.Link,
			Description:    item.Description,
			Content:        content,
			ExtractedLinks: extractor.ExtractLinks(decodedContent),
		}
		feed.Items = append(feed.Items, feedItem)
	}

	return feed
}

func parseAtom(atom *atomFeed) *Feed {
	// Find the main link (prefer alternate, fallback to first)
	var feedURL string
	for _, link := range atom.Links {
		if link.Rel == "alternate" || link.Rel == "" {
			feedURL = link.Href
			break
		}
	}
	if feedURL == "" && len(atom.Links) > 0 {
		feedURL = atom.Links[0].Href
	}

	feed := &Feed{
		Title: atom.Title,
		URL:   strings.TrimSuffix(feedURL, "/"),
		Items: make([]Item, 0, len(atom.Entries)),
	}

	for _, entry := range atom.Entries {
		// Find entry link
		var entryURL string
		for _, link := range entry.Links {
			if link.Rel == "alternate" || link.Rel == "" {
				entryURL = link.Href
				break
			}
		}
		if entryURL == "" && len(entry.Links) > 0 {
			entryURL = entry.Links[0].Href
		}

		content := entry.Content
		if content == "" {
			content = entry.Summary
		}

		// Decode HTML entities in content
		decodedContent := html.UnescapeString(content)

		feedItem := Item{
			Title:          entry.Title,
			URL:            entryURL,
			Description:    entry.Summary,
			Content:        content,
			ExtractedLinks: extractor.ExtractLinks(decodedContent),
		}
		feed.Items = append(feed.Items, feedItem)
	}

	return feed
}
