package feed

import (
	"testing"
)

func TestParseFeed_RSS2(t *testing.T) {
	rss := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Simon Willison's Blog</title>
    <link>https://simonwillison.net/</link>
    <description>A blog about Python, Django, and more</description>
    <item>
      <title>First Post</title>
      <link>https://simonwillison.net/2024/Jan/1/first/</link>
      <description>&lt;p&gt;Check out &lt;a href="https://example.com"&gt;this link&lt;/a&gt;&lt;/p&gt;</description>
    </item>
    <item>
      <title>Second Post</title>
      <link>https://simonwillison.net/2024/Jan/2/second/</link>
      <description>Plain text description</description>
    </item>
  </channel>
</rss>`

	feed, err := ParseFeed([]byte(rss))
	if err != nil {
		t.Fatalf("ParseFeed error: %v", err)
	}

	if feed.Title != "Simon Willison's Blog" {
		t.Errorf("Expected title 'Simon Willison's Blog', got '%s'", feed.Title)
	}
	if feed.URL != "https://simonwillison.net/" {
		t.Errorf("Expected URL 'https://simonwillison.net/', got '%s'", feed.URL)
	}
	if len(feed.Items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(feed.Items))
	}
	if feed.Items[0].Title != "First Post" {
		t.Errorf("Expected first item title 'First Post', got '%s'", feed.Items[0].Title)
	}
}

func TestParseFeed_Atom(t *testing.T) {
	atom := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Hamel's Blog</title>
  <link href="https://hamel.dev/"/>
  <entry>
    <title>LLM Engineering</title>
    <link href="https://hamel.dev/blog/llm-engineering"/>
    <content type="html">&lt;p&gt;Some content with &lt;a href="https://anthropic.com"&gt;a link&lt;/a&gt;&lt;/p&gt;</content>
  </entry>
</feed>`

	feed, err := ParseFeed([]byte(atom))
	if err != nil {
		t.Fatalf("ParseFeed error: %v", err)
	}

	if feed.Title != "Hamel's Blog" {
		t.Errorf("Expected title 'Hamel's Blog', got '%s'", feed.Title)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(feed.Items))
	}
}

func TestParseFeed_Empty(t *testing.T) {
	_, err := ParseFeed([]byte(""))
	if err == nil {
		t.Error("Expected error for empty input")
	}
}

func TestParseFeed_Invalid(t *testing.T) {
	_, err := ParseFeed([]byte("not xml at all"))
	if err == nil {
		t.Error("Expected error for invalid XML")
	}
}

func TestParseFeed_ExtractLinksFromContent(t *testing.T) {
	rss := `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Test</title>
    <link>https://test.com/</link>
    <item>
      <title>Post</title>
      <link>https://test.com/post</link>
      <description>&lt;p&gt;Links: &lt;a href="https://a.com"&gt;A&lt;/a&gt; and &lt;a href="https://b.com"&gt;B&lt;/a&gt;&lt;/p&gt;</description>
    </item>
  </channel>
</rss>`

	feed, err := ParseFeed([]byte(rss))
	if err != nil {
		t.Fatalf("ParseFeed error: %v", err)
	}

	if len(feed.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(feed.Items))
	}

	links := feed.Items[0].ExtractedLinks
	if len(links) != 2 {
		t.Errorf("Expected 2 extracted links, got %d", len(links))
	}
}

func TestFeedItem_ContentOrDescription(t *testing.T) {
	// Test that we prefer content over description when both exist
	rss := `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Test</title>
    <link>https://test.com/</link>
    <item>
      <title>Post</title>
      <link>https://test.com/post</link>
      <description>Short desc</description>
      <content:encoded xmlns:content="http://purl.org/rss/1.0/modules/content/">Full content here</content:encoded>
    </item>
  </channel>
</rss>`

	feed, err := ParseFeed([]byte(rss))
	if err != nil {
		t.Fatalf("ParseFeed error: %v", err)
	}

	// Should have captured the content
	if feed.Items[0].Content == "" {
		t.Error("Expected content to be extracted")
	}
}
