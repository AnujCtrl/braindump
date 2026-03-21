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

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Brain dump: capture multiple todos quickly",
	RunE:  runDump,
}

func init() {
	dumpCmd.Flags().StringP("tag", "t", "", "batch tag applied to all items")
}

func runDump(cmd *cobra.Command, args []string) error {
	defaultTag, _ := cmd.Flags().GetString("tag")

	fmt.Println("Brain dump mode. Enter one item per line. Empty line to finish.")

	scanner := bufio.NewScanner(os.Stdin)
	var created int
	tagCounts := make(map[string]int)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			break
		}

		todo, err := parseDumpLine(line, defaultTag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing line: %v\n", err)
			continue
		}

		if err := store.AddTodo(todo); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving todo: %v\n", err)
			continue
		}

		date := todo.Created.Format("2006-01-02")
		if err := logger.LogCreated(date, todo); err != nil {
			fmt.Fprintf(os.Stderr, "Error logging: %v\n", err)
		}

		created++
		for _, tag := range todo.Tags {
			tagCounts[tag]++
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	if created == 0 {
		fmt.Println("Nothing captured.")
	} else {
		var breakdown []string
		for tag, count := range tagCounts {
			breakdown = append(breakdown, fmt.Sprintf("#%s: %d", tag, count))
		}
		summary := strings.Join(breakdown, ", ")
		if summary == "" {
			summary = "(no tags)"
		}
		fmt.Printf("Created %d todos. (%s)\n", created, summary)
	}

	printInfoLine()
	return nil
}

// parseDumpLine parses a single brain dump line into a Todo.
// Extracts #tags, @source, !!, !!! from the text.
func parseDumpLine(line string, defaultTag string) (*core.Todo, error) {
	todo := &core.Todo{
		Status:  "unprocessed",
		Source:  "cli",
		Created: time.Now(),
	}

	// Normalize shell-escaped exclamation marks: \! → !
	line = strings.ReplaceAll(line, `\!`, "!")

	// Extract !!! (important) and !! (urgent) — check !!! first.
	if strings.Contains(line, "!!!") {
		todo.Important = true
		line = strings.Replace(line, "!!!", "", 1)
	} else if strings.Contains(line, "!!") {
		todo.Urgent = true
		line = strings.Replace(line, "!!", "", 1)
	}

	// Extract @source.
	words := strings.Fields(line)
	var remaining []string
	for _, w := range words {
		if strings.HasPrefix(w, "@") && len(w) > 1 {
			todo.Source = w[1:]
		} else if strings.HasPrefix(w, "#") && len(w) > 1 {
			todo.Tags = append(todo.Tags, w[1:])
		} else {
			remaining = append(remaining, w)
		}
	}

	todo.Text = strings.TrimSpace(strings.Join(remaining, " "))
	if todo.Text == "" {
		return nil, fmt.Errorf("empty todo text")
	}

	// Apply default tag if no tags found.
	if len(todo.Tags) == 0 {
		if defaultTag != "" {
			todo.Tags = []string{defaultTag}
		} else {
			todo.Tags = []string{"braindump"}
		}
	}

	// Generate unique ID.
	id, err := core.GenerateID()
	if err != nil {
		return nil, err
	}
	todo.ID = id

	return todo, nil
}
