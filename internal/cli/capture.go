package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anujp/braindump/internal/core"
	"github.com/spf13/cobra"
)

// runCapture parses the CLI arguments into a Todo and persists it.
func runCapture(cmd *cobra.Command, args []string) error {
	raw := strings.Join(args, " ")

	var (
		textWords []string
		tags      []string
		source    string
		urgent    bool
		important bool
		note      string
	)

	// Get note from flag if present.
	note, _ = cmd.Flags().GetString("note")

	// Split on "--" separator: everything after is plain text.
	plainSuffix := ""
	if idx := strings.Index(raw, " -- "); idx != -1 {
		plainSuffix = raw[idx+4:]
		raw = raw[:idx]
	} else if strings.HasSuffix(raw, " --") {
		raw = raw[:len(raw)-3]
	}

	// Tokenize remaining input.
	tokens := tokenize(raw)

	for _, tok := range tokens {
		// Normalize shell-escaped exclamation marks: \! → !
		normTok := strings.ReplaceAll(tok, `\!`, "!")
		switch {
		case normTok == "!!!":
			important = true
		case normTok == "!!":
			urgent = true
		case strings.HasPrefix(tok, "#"):
			tagName := tok[1:]
			if tagName != "" {
				tags = append(tags, tagName)
			}
		case strings.HasPrefix(tok, `\#`):
			// Escaped hash — literal # in text.
			textWords = append(textWords, "#"+tok[2:])
		case strings.HasPrefix(tok, "@"):
			srcName := tok[1:]
			if srcName != "" {
				source = srcName
			}
		default:
			textWords = append(textWords, tok)
		}
	}

	// Append plain suffix text (from after --).
	if plainSuffix != "" {
		textWords = append(textWords, plainSuffix)
	}

	// Validate and resolve tags.
	resolvedTags, err := resolveTags(tags)
	if err != nil {
		return err
	}
	tags = resolvedTags

	// Defaults.
	if len(tags) == 0 {
		tags = []string{"braindump"}
	}
	if source == "" {
		source = "cli"
	}

	// If source matches a tag name, auto-add that tag.
	if tagStore.IsValid(source) {
		found := false
		for _, t := range tags {
			if strings.EqualFold(t, source) {
				found = true
				break
			}
		}
		if !found {
			tags = append(tags, source)
		}
	}

	text := strings.TrimSpace(strings.Join(textWords, " "))
	if text == "" {
		return fmt.Errorf("no text provided for todo")
	}

	// Generate a unique ID.
	existingIDs, err := store.CollectAllIDs()
	if err != nil {
		return fmt.Errorf("collecting IDs: %w", err)
	}

	var id string
	for {
		id, err = core.GenerateID()
		if err != nil {
			return fmt.Errorf("generating ID: %w", err)
		}
		if core.IsUniqueID(id, existingIDs) {
			break
		}
	}

	now := time.Now()

	todo := &core.Todo{
		ID:            id,
		Text:          text,
		Source:        source,
		Status:        "inbox",
		Created:       now,
		StatusChanged: now,
		Urgent:        urgent,
		Important:     important,
		Tags:          tags,
		Done:          false,
	}

	if note != "" {
		todo.Notes = []string{note}
	}

	if err := store.AddTodo(todo); err != nil {
		return fmt.Errorf("saving todo: %w", err)
	}

	date := now.Format("2006-01-02")
	if err := logger.LogCreated(date, todo); err != nil {
		return fmt.Errorf("logging creation: %w", err)
	}

	fmt.Printf("✓ Captured (%s)\n", id)
	printInfoLine()

	return nil
}

// extractNote finds and removes --note "value" or --note value from the input.
// Returns the remaining string and the extracted note.
func extractNote(s string) (string, string) {
	const flag = "--note"

	idx := strings.Index(s, flag)
	if idx == -1 {
		return s, ""
	}

	before := s[:idx]
	after := strings.TrimSpace(s[idx+len(flag):])

	if after == "" {
		return strings.TrimSpace(before), ""
	}

	var note string
	var rest string

	if after[0] == '"' {
		// Quoted note value — find closing quote.
		closeIdx := strings.Index(after[1:], "\"")
		if closeIdx == -1 {
			// No closing quote; take the rest.
			note = after[1:]
			rest = ""
		} else {
			note = after[1 : closeIdx+1]
			rest = strings.TrimSpace(after[closeIdx+2:])
		}
	} else {
		// Unquoted note value — take the next token.
		parts := strings.SplitN(after, " ", 2)
		note = parts[0]
		if len(parts) > 1 {
			rest = parts[1]
		}
	}

	remaining := strings.TrimSpace(before + " " + rest)
	return remaining, note
}

// tokenize splits a string by whitespace, respecting quoted segments.
func tokenize(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	var tokens []string
	var current strings.Builder
	inQuote := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '"':
			inQuote = !inQuote
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// resolveTags validates each tag against the tag store, prompting the user
// for fuzzy matches or new tag creation when a tag is not found.
func resolveTags(tags []string) ([]string, error) {
	var resolved []string
	reader := bufio.NewReader(os.Stdin)

	for _, tag := range tags {
		if tagStore.IsValid(tag) {
			resolved = append(resolved, tag)
			continue
		}

		// Tag not found — try fuzzy match.
		suggestion := tagStore.FuzzyMatch(tag)

		if suggestion != "" {
			fmt.Printf("\"#%s\" not found. Did you mean #%s? [Y/n/add as new] ", tag, suggestion)
			input, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("reading input: %w", err)
			}
			input = strings.TrimSpace(input)

			switch strings.ToLower(input) {
			case "", "y":
				resolved = append(resolved, suggestion)
			case "n":
				// Skip this tag.
			case "add":
				if err := tagStore.AddTag(tag, ""); err != nil {
					return nil, fmt.Errorf("adding tag %q: %w", tag, err)
				}
				resolved = append(resolved, tag)
			default:
				// Treat unknown input as skip.
			}
		} else {
			fmt.Printf("\"#%s\" not found. Add as new tag? [y/N] ", tag)
			input, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("reading input: %w", err)
			}
			input = strings.TrimSpace(input)

			switch strings.ToLower(input) {
			case "y":
				if err := tagStore.AddTag(tag, ""); err != nil {
					return nil, fmt.Errorf("adding tag %q: %w", tag, err)
				}
				resolved = append(resolved, tag)
			default:
				// Skip by default.
			}
		}
	}

	return resolved, nil
}
