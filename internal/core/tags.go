package core

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type TagsFile struct {
	Categories []string `yaml:"categories"`
	Energy     []string `yaml:"energy"`
}

type TagStore struct {
	FilePath string
	Tags     TagsFile
}

// NewTagStore loads and parses the tags.yaml file at filePath.
func NewTagStore(filePath string) (*TagStore, error) {
	ts := &TagStore{FilePath: filePath}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &ts.Tags); err != nil {
		return nil, err
	}

	return ts, nil
}

// AllTags returns all tags from all groups flattened into a single slice.
func (ts *TagStore) AllTags() []string {
	all := make([]string, 0, len(ts.Tags.Categories)+len(ts.Tags.Energy))
	all = append(all, ts.Tags.Categories...)
	all = append(all, ts.Tags.Energy...)
	return all
}

// IsValid checks if a tag exists (case-insensitive).
func (ts *TagStore) IsValid(tag string) bool {
	lower := strings.ToLower(tag)
	for _, t := range ts.AllTags() {
		if strings.ToLower(t) == lower {
			return true
		}
	}
	return false
}

// FuzzyMatch finds the closest matching tag using Levenshtein distance.
// Returns empty string if no match is within distance 3.
func (ts *TagStore) FuzzyMatch(input string) string {
	lower := strings.ToLower(input)
	bestMatch := ""
	bestDist := 4 // threshold: must be <= 3

	for _, tag := range ts.AllTags() {
		d := levenshtein(lower, strings.ToLower(tag))
		if d < bestDist {
			bestDist = d
			bestMatch = tag
		}
	}

	return bestMatch
}

// AddTag adds a new tag to the specified group and saves to disk.
// Group defaults to "categories" if empty.
func (ts *TagStore) AddTag(tag string, group string) error {
	tag = strings.ToLower(tag)
	if group == "" {
		group = "categories"
	}

	switch group {
	case "categories":
		ts.Tags.Categories = append(ts.Tags.Categories, tag)
	case "energy":
		ts.Tags.Energy = append(ts.Tags.Energy, tag)
	}

	return ts.Save()
}

// Save writes the current tags back to tags.yaml.
func (ts *TagStore) Save() error {
	data, err := yaml.Marshal(&ts.Tags)
	if err != nil {
		return err
	}
	return os.WriteFile(ts.FilePath, data, 0644)
}

// GroupedTags returns tags organized by group name.
func (ts *TagStore) GroupedTags() map[string][]string {
	return map[string][]string{
		"categories": ts.Tags.Categories,
		"energy":     ts.Tags.Energy,
	}
}

// levenshtein computes the Levenshtein distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Use a single row for space efficiency.
	prev := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr := make([]int, lb+1)
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(
				prev[j]+1,      // deletion
				curr[j-1]+1,    // insertion
				prev[j-1]+cost, // substitution
			)
		}
		prev = curr
	}

	return prev[lb]
}

func min(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}
