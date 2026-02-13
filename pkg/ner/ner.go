// Package ner provides Named Entity Recognition using prose.
package ner

import (
	"strings"

	"github.com/jdkato/prose/v2"
)

// Entity represents a named entity found in text.
type Entity struct {
	Text  string
	Label string // PERSON, GPE (org/location), etc.
}

// Blocklist of terms that are commonly misidentified as people
var notPeople = map[string]bool{
	// AI Products/Models
	"claude":          true,
	"claude code":     true,
	"claude opus":     true,
	"claude sonnet":   true,
	"chatgpt":         true,
	"gpt":             true,
	"gpt-4":           true,
	"gpt-4o":          true,
	"gpt-3":           true,
	"gemini":          true,
	"llama":           true,
	"mistral":         true,
	"qwen":            true,
	"kimi":            true,
	"kimi k2":         true,
	"deepseek":        true,
	"copilot":         true,

	// Companies (sometimes detected as people)
	"openai":    true,
	"anthropic": true,
	"google":    true,
	"meta":      true,
	"microsoft": true,
	"nvidia":    true,
	"huggingface": true,
	"arcee":     true,

	// Technical terms
	"llm":       true,
	"llms":      true,
	"ai":        true,
	"ml":        true,
	"rlhf":      true,
	"python":    true,
	"json":      true,
	"api":       true,
	"bert":      true,
	"transformer": true,
	"rag":       true,

	// UI/Web false positives
	"share":     true,
	"below":     true,
	"above":     true,
	"click":     true,
	"tags":      true,
	"via":       true,
	"via hacker news tags": true,
	"read more": true,
	"subscribe": true,
	"comments":  true,
}

// ExtractEntities extracts all named entities from text.
func ExtractEntities(text string) []Entity {
	if text == "" {
		return []Entity{}
	}

	// Strip HTML tags for cleaner NER
	text = stripHTML(text)

	doc, err := prose.NewDocument(text)
	if err != nil {
		return []Entity{}
	}

	var entities []Entity
	for _, ent := range doc.Entities() {
		entities = append(entities, Entity{
			Text:  ent.Text,
			Label: ent.Label,
		})
	}

	return entities
}

// ExtractPeople extracts unique person names from text.
// Filters out known products, companies, and false positives.
func ExtractPeople(text string) []string {
	entities := ExtractEntities(text)

	seen := make(map[string]bool)
	var people []string

	for _, ent := range entities {
		if ent.Label == "PERSON" {
			name := normalizeName(ent.Text)
			nameLower := strings.ToLower(name)

			// Skip blocklisted terms
			if notPeople[nameLower] {
				continue
			}

			// Skip single words (usually false positives)
			if !strings.Contains(name, " ") && len(name) < 10 {
				continue
			}

			// Skip if it looks like a title/header (all caps or weird patterns)
			if name == strings.ToUpper(name) && len(name) > 3 {
				continue
			}

			if !seen[nameLower] {
				seen[nameLower] = true
				people = append(people, name)
			}
		}
	}

	return people
}

// ExtractOrganizations extracts unique organization names from text.
func ExtractOrganizations(text string) []string {
	entities := ExtractEntities(text)

	seen := make(map[string]bool)
	var orgs []string

	for _, ent := range entities {
		// prose uses GPE for geopolitical entities (orgs, places)
		if ent.Label == "GPE" || ent.Label == "ORG" {
			name := strings.TrimSpace(ent.Text)
			if !seen[name] {
				seen[name] = true
				orgs = append(orgs, name)
			}
		}
	}

	return orgs
}

// stripHTML removes HTML tags from text.
func stripHTML(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			result.WriteRune(' ')
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// normalizeName cleans up a person's name.
func normalizeName(name string) string {
	// Remove possessives
	name = strings.TrimSuffix(name, "'s")
	name = strings.TrimSuffix(name, "'s")
	return strings.TrimSpace(name)
}
