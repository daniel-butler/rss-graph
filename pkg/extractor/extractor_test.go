package extractor

import (
	"testing"
)

func TestExtractLinks_Basic(t *testing.T) {
	html := `<p>Check out <a href="https://simonwillison.net/">Simon's blog</a> for great content.</p>`

	links := ExtractLinks(html)

	if len(links) != 1 {
		t.Fatalf("Expected 1 link, got %d", len(links))
	}
	if links[0].URL != "https://simonwillison.net/" {
		t.Errorf("Expected URL https://simonwillison.net/, got %s", links[0].URL)
	}
	if links[0].Text != "Simon's blog" {
		t.Errorf("Expected text 'Simon's blog', got %s", links[0].Text)
	}
}

func TestExtractLinks_MultipleLinks(t *testing.T) {
	html := `
		<p>I recommend <a href="https://example.com/alice">Alice's blog</a> and 
		<a href="https://example.com/bob">Bob's site</a>.</p>
	`

	links := ExtractLinks(html)

	if len(links) != 2 {
		t.Fatalf("Expected 2 links, got %d", len(links))
	}
}

func TestExtractLinks_NoLinks(t *testing.T) {
	html := `<p>Just plain text with no links.</p>`

	links := ExtractLinks(html)

	if len(links) != 0 {
		t.Errorf("Expected 0 links, got %d", len(links))
	}
}

func TestExtractLinks_EmptyInput(t *testing.T) {
	links := ExtractLinks("")

	if len(links) != 0 {
		t.Errorf("Expected 0 links for empty input, got %d", len(links))
	}
}

func TestExtractLinks_IgnoresInternalAnchors(t *testing.T) {
	html := `<a href="#section1">Jump to section</a>`

	links := ExtractLinks(html)

	if len(links) != 0 {
		t.Errorf("Expected 0 links (anchors ignored), got %d", len(links))
	}
}

func TestExtractLinks_IgnoresJavascript(t *testing.T) {
	html := `<a href="javascript:void(0)">Click me</a>`

	links := ExtractLinks(html)

	if len(links) != 0 {
		t.Errorf("Expected 0 links (javascript ignored), got %d", len(links))
	}
}

func TestExtractLinks_IgnoresMailto(t *testing.T) {
	html := `<a href="mailto:test@example.com">Email me</a>`

	links := ExtractLinks(html)

	if len(links) != 0 {
		t.Errorf("Expected 0 links (mailto ignored), got %d", len(links))
	}
}

func TestExtractLinks_HandlesRelativeURLs(t *testing.T) {
	html := `<a href="/about">About page</a>`

	links := ExtractLinks(html)

	// Relative URLs should be extracted (caller resolves them)
	if len(links) != 1 {
		t.Fatalf("Expected 1 link, got %d", len(links))
	}
	if links[0].URL != "/about" {
		t.Errorf("Expected /about, got %s", links[0].URL)
	}
}

func TestExtractLinks_DeduplicatesURLs(t *testing.T) {
	html := `
		<a href="https://example.com">First</a>
		<a href="https://example.com">Second</a>
		<a href="https://example.com/">Third with slash</a>
	`

	links := ExtractLinks(html)

	// Should dedupe same URLs (with/without trailing slash normalized)
	if len(links) > 2 {
		t.Errorf("Expected deduplication, got %d links", len(links))
	}
}

func TestExtractLinks_ExtractsFromNestedHTML(t *testing.T) {
	html := `
		<div class="content">
			<article>
				<p>Read <a href="https://deep.example.com">this</a></p>
			</article>
		</div>
	`

	links := ExtractLinks(html)

	if len(links) != 1 {
		t.Fatalf("Expected 1 link from nested HTML, got %d", len(links))
	}
	if links[0].URL != "https://deep.example.com" {
		t.Errorf("Wrong URL extracted")
	}
}
