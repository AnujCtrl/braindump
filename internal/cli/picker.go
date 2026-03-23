package cli

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/anujp/braindump/internal/core"
)

// pickItem represents a todo with its associated date for the interactive picker.
type pickItem struct {
	todo *core.Todo
	date string
}

// collectByStatus collects all todos with the given status across all date files.
func collectByStatus(status string) ([]*pickItem, error) {
	allDays, err := store.ReadAllDays()
	if err != nil {
		return nil, err
	}

	var items []*pickItem
	for date, todos := range allDays {
		for _, todo := range todos {
			if todo.Status == status {
				items = append(items, &pickItem{todo: todo, date: date})
			}
		}
	}
	return items, nil
}

// pickFromList presents a numbered list and reads user selection from scanner.
// Returns nil, nil on empty input/EOF or empty items list.
func pickFromList(items []*pickItem, prompt string, scanner *bufio.Scanner) (*pickItem, error) {
	if len(items) == 0 {
		return nil, nil
	}

	for i, item := range items {
		fmt.Printf("  %d) [%s] %s\n", i+1, item.todo.ID, item.todo.Text)
	}

	fmt.Print(prompt)
	if !scanner.Scan() {
		return nil, nil
	}

	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return nil, nil
	}

	num, err := strconv.Atoi(input)
	if err != nil {
		return nil, fmt.Errorf("invalid selection: %s", input)
	}
	if num < 1 || num > len(items) {
		return nil, fmt.Errorf("selection out of range: %d", num)
	}

	return items[num-1], nil
}
