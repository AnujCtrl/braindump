package core

import (
	"fmt"
	"strings"
	"time"
)

const timeFormat = "2006-01-02T15:04:05"

// Todo represents a single captured item with metadata.
type Todo struct {
	ID         string
	Text       string
	Source     string
	Status     string // unprocessed, inbox, today, waiting, done, stale
	Created    time.Time
	Urgent     bool
	Important  bool
	StaleCount int
	Tags       []string
	Notes      []string // lines from > prefixed notes
	Subtasks   []string // lines from indented - [ ] subtasks
	Done       bool     // checkbox state
}

// ParseTodoLine parses a single markdown todo line into a Todo struct.
// It does not handle notes or subtasks (use ParseTodoBlock for that).
func ParseTodoLine(line string) (*Todo, error) {
	line = strings.TrimSpace(line)

	// Parse checkbox
	var done bool
	var rest string
	switch {
	case strings.HasPrefix(line, "- [x] "):
		done = true
		rest = line[6:]
	case strings.HasPrefix(line, "- [ ] "):
		done = false
		rest = line[6:]
	default:
		return nil, fmt.Errorf("invalid todo line: missing checkbox prefix")
	}

	todo := &Todo{
		Done:   done,
		Status: "unprocessed",
	}

	// Extract tags (#word) from the end
	var tags []string
	for {
		rest = strings.TrimRight(rest, " ")
		idx := strings.LastIndex(rest, " #")
		if idx == -1 {
			break
		}
		candidate := rest[idx+2:]
		// Tags shouldn't contain spaces or braces
		if strings.ContainsAny(candidate, " {}") {
			break
		}
		tags = append([]string{candidate}, tags...)
		rest = rest[:idx]
	}
	todo.Tags = tags

	// Extract {key:value} metadata fields from the end
	for {
		rest = strings.TrimRight(rest, " ")
		if !strings.HasSuffix(rest, "}") {
			break
		}
		openIdx := strings.LastIndex(rest, "{")
		if openIdx == -1 {
			break
		}
		meta := rest[openIdx+1 : len(rest)-1]
		rest = rest[:openIdx]

		colonIdx := strings.Index(meta, ":")
		if colonIdx == -1 {
			continue
		}
		key := meta[:colonIdx]
		value := meta[colonIdx+1:]

		switch key {
		case "id":
			todo.ID = value
		case "source":
			todo.Source = value
		case "status":
			todo.Status = value
		case "created":
			t, err := time.Parse(timeFormat, value)
			if err != nil {
				return nil, fmt.Errorf("parsing created time %q: %w", value, err)
			}
			todo.Created = t
		case "urgent":
			todo.Urgent = value == "yes"
		case "important":
			todo.Important = value == "yes"
		case "stale_count":
			n := 0
			for _, c := range value {
				if c < '0' || c > '9' {
					return nil, fmt.Errorf("invalid stale_count: %q", value)
				}
				n = n*10 + int(c-'0')
			}
			todo.StaleCount = n
		}
	}

	todo.Text = strings.TrimSpace(rest)

	return todo, nil
}

// ToMarkdown serializes a Todo back to its markdown line representation,
// including any notes and subtasks on subsequent lines.
func (t *Todo) ToMarkdown() string {
	var b strings.Builder

	// Checkbox
	if t.Done {
		b.WriteString("- [x] ")
	} else {
		b.WriteString("- [ ] ")
	}

	// Text
	b.WriteString(t.Text)

	// Metadata fields
	b.WriteString(" {id:")
	b.WriteString(t.ID)
	b.WriteString("}")

	b.WriteString(" {source:")
	b.WriteString(t.Source)
	b.WriteString("}")

	b.WriteString(" {status:")
	b.WriteString(t.Status)
	b.WriteString("}")

	b.WriteString(" {created:")
	b.WriteString(t.Created.Format(timeFormat))
	b.WriteString("}")

	b.WriteString(" {urgent:")
	b.WriteString(boolToYesNo(t.Urgent))
	b.WriteString("}")

	b.WriteString(" {important:")
	b.WriteString(boolToYesNo(t.Important))
	b.WriteString("}")

	b.WriteString(" {stale_count:")
	b.WriteString(fmt.Sprintf("%d", t.StaleCount))
	b.WriteString("}")

	// Tags
	for _, tag := range t.Tags {
		b.WriteString(" #")
		b.WriteString(tag)
	}

	// Notes
	for _, note := range t.Notes {
		b.WriteString("\n  > ")
		b.WriteString(note)
	}

	// Subtasks
	for _, sub := range t.Subtasks {
		b.WriteString("\n  - [ ] ")
		b.WriteString(sub)
	}

	return b.String()
}

// ParseTodoBlock parses a todo line plus its subsequent note and subtask lines.
// The first element of lines must be the todo line; following lines are notes/subtasks.
func ParseTodoBlock(lines []string) (*Todo, error) {
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty block")
	}

	todo, err := ParseTodoLine(lines[0])
	if err != nil {
		return nil, err
	}

	for _, line := range lines[1:] {
		trimmed := strings.TrimRight(line, " \t")
		switch {
		case strings.HasPrefix(trimmed, "  > "):
			todo.Notes = append(todo.Notes, trimmed[4:])
		case strings.HasPrefix(trimmed, "  - [ ] "):
			todo.Subtasks = append(todo.Subtasks, trimmed[8:])
		case strings.HasPrefix(trimmed, "  - [x] "):
			todo.Subtasks = append(todo.Subtasks, trimmed[8:])
		}
	}

	return todo, nil
}

// ParseDayFile parses an entire day file's content into a slice of Todos.
// It skips the `# date` header line at the top.
func ParseDayFile(content string) ([]*Todo, error) {
	lines := strings.Split(content, "\n")

	var todos []*Todo
	var blockLines []string

	flushBlock := func() error {
		if len(blockLines) == 0 {
			return nil
		}
		todo, err := ParseTodoBlock(blockLines)
		if err != nil {
			return err
		}
		todos = append(todos, todo)
		blockLines = nil
		return nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip header and blank lines
		if strings.HasPrefix(trimmed, "# ") || trimmed == "" {
			continue
		}

		// Start of a new todo line
		if strings.HasPrefix(trimmed, "- [") {
			if err := flushBlock(); err != nil {
				return nil, err
			}
			blockLines = []string{line}
			continue
		}

		// Note or subtask continuation
		if len(blockLines) > 0 && (strings.HasPrefix(line, "  >") || strings.HasPrefix(line, "  -")) {
			blockLines = append(blockLines, line)
			continue
		}
	}

	if err := flushBlock(); err != nil {
		return nil, err
	}

	return todos, nil
}

// SerializeDayFile serializes a date header and a slice of Todos into a full day file string.
func SerializeDayFile(date string, todos []*Todo) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(date)
	b.WriteString("\n")

	for _, t := range todos {
		b.WriteString("\n")
		b.WriteString(t.ToMarkdown())
		b.WriteString("\n")
	}

	return b.String()
}

func boolToYesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
