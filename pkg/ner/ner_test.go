package ner

import (
	"testing"
)

func TestExtractEntities_People(t *testing.T) {
	text := `I've been impressed by Hamel Husain's work on LLM evals.
	Simon Willison built amazing tools. Nathan Lambert wrote the RLHF book.`

	entities := ExtractEntities(text)

	people := filterByLabel(entities, "PERSON")
	if len(people) != 3 {
		t.Errorf("Expected 3 people, got %d: %v", len(people), people)
	}

	names := entityNames(people)
	if !contains(names, "Hamel Husain") {
		t.Error("Expected to find Hamel Husain")
	}
	if !contains(names, "Simon Willison") {
		t.Error("Expected to find Simon Willison")
	}
}

func TestExtractEntities_Organizations(t *testing.T) {
	text := `Anthropic released Claude. OpenAI built GPT. Google has Gemini.`

	entities := ExtractEntities(text)

	orgs := filterByLabel(entities, "GPE") // prose uses GPE for orgs
	if len(orgs) < 2 {
		t.Errorf("Expected at least 2 organizations, got %d", len(orgs))
	}
}

func TestExtractEntities_Empty(t *testing.T) {
	entities := ExtractEntities("")
	if len(entities) != 0 {
		t.Errorf("Expected 0 entities for empty input, got %d", len(entities))
	}
}

func TestExtractEntities_NoEntities(t *testing.T) {
	text := "This is a simple sentence without any named entities."
	entities := ExtractEntities(text)
	// Should return empty or minimal false positives
	people := filterByLabel(entities, "PERSON")
	if len(people) > 0 {
		t.Errorf("Expected no people in generic text, got %d", len(people))
	}
}

func TestExtractPeople(t *testing.T) {
	text := `Hamel Husain and Eugene Yan work on AI evals. 
	The conference was in New York.`

	people := ExtractPeople(text)

	if len(people) != 2 {
		t.Errorf("Expected 2 people, got %d: %v", len(people), people)
	}
}

func TestExtractPeople_Deduplicates(t *testing.T) {
	text := `Simon Willison wrote about LLMs. Later, Simon Willison shared more insights.`

	people := ExtractPeople(text)

	if len(people) != 1 {
		t.Errorf("Expected 1 unique person, got %d: %v", len(people), people)
	}
}

// Helper functions
func filterByLabel(entities []Entity, label string) []Entity {
	var filtered []Entity
	for _, e := range entities {
		if e.Label == label {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func entityNames(entities []Entity) []string {
	names := make([]string, len(entities))
	for i, e := range entities {
		names[i] = e.Text
	}
	return names
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
