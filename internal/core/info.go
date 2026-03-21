package core

import "fmt"

// InfoLine holds the counts displayed in the CLI info bar.
type InfoLine struct {
	Unprocessed int
	Looping     int
}

// GetInfoLine computes the info line data from the store.
// Unprocessed = count of todos with status "unprocessed"
// Looping = count of todos with stale_count >= 2
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
		if todo.StaleCount >= 2 {
			info.Looping++
		}
	}
	return info, nil
}

// FormatInfoLine returns the formatted info line string for CLI display.
// If both counts are 0, returns an empty string.
func FormatInfoLine(info InfoLine) string {
	if info.Unprocessed == 0 && info.Looping == 0 {
		return ""
	}
	return fmt.Sprintf("── 📬 Unprocessed: %d │ 🔁 Looping: %d ──",
		info.Unprocessed, info.Looping)
}
