package core

import (
	"fmt"
	"strings"
)

// InfoLine holds the counts displayed in the CLI info bar.
type InfoLine struct {
	Unprocessed int
	Looping     int
	Active      int
}

// GetInfoLine computes the info line data from the store.
// Unprocessed = count of todos with status "unprocessed"
// Looping = count of todos with stale_count >= 2
// Active = count of todos with status "active"
func GetInfoLine(store *Store) (InfoLine, error) {
	all, err := allTodos(store)
	if err != nil {
		return InfoLine{}, err
	}

	var info InfoLine
	for _, todo := range all {
		if todo.Status == "unprocessed" {
			info.Unprocessed++
		}
		if todo.Status == "active" {
			info.Active++
		}
		if todo.StaleCount >= 2 {
			info.Looping++
		}
	}
	return info, nil
}

// FormatInfoLine returns the formatted info line string for CLI display.
// If all counts are 0, returns an empty string.
func FormatInfoLine(info InfoLine) string {
	if info.Unprocessed == 0 && info.Looping == 0 && info.Active == 0 {
		return ""
	}
	var parts []string
	if info.Unprocessed > 0 {
		parts = append(parts, fmt.Sprintf("Unprocessed: %d", info.Unprocessed))
	}
	if info.Active > 0 {
		parts = append(parts, fmt.Sprintf("Active: %d", info.Active))
	}
	if info.Looping > 0 {
		parts = append(parts, fmt.Sprintf("Looping: %d", info.Looping))
	}
	return fmt.Sprintf("-- %s --", strings.Join(parts, " | "))
}
