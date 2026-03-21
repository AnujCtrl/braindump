package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/anujp/braindump/internal/core"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List todos",
	RunE:  runList,
}

func init() {
	lsCmd.Flags().BoolP("all", "a", false, "show all days")
	lsCmd.Flags().StringP("tag", "t", "", "filter by tag")
	lsCmd.Flags().StringP("date", "d", "", "show specific date (YYYY-MM-DD)")
	lsCmd.Flags().BoolP("looping", "l", false, "show looping items (stale_count >= 2)")
}

func runList(cmd *cobra.Command, args []string) error {
	showAll, _ := cmd.Flags().GetBool("all")
	tagFilter, _ := cmd.Flags().GetString("tag")
	dateFilter, _ := cmd.Flags().GetString("date")
	showLooping, _ := cmd.Flags().GetBool("looping")

	if showLooping {
		return listLooping()
	}

	// Collect the date->todos map to display.
	days := make(map[string][]*core.Todo)

	if showAll {
		allDays, err := store.ReadAllDays()
		if err != nil {
			return err
		}
		days = allDays
	} else {
		date := dateFilter
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		todos, err := store.ReadDay(date)
		if err != nil {
			return err
		}
		days[date] = todos
	}

	// Sort dates for consistent output.
	dates := make([]string, 0, len(days))
	for d := range days {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	multiDay := len(dates) > 1

	for _, date := range dates {
		todos := days[date]

		// Apply tag filter if set.
		if tagFilter != "" {
			todos = filterByTag(todos, tagFilter)
		}

		if len(todos) == 0 {
			continue
		}

		if multiDay {
			fmt.Printf("── %s ──\n", date)
		}

		for _, t := range todos {
			fmt.Println(formatTodoLine(t))
		}

		if multiDay {
			fmt.Println()
		}
	}

	printInfoLine()
	return nil
}

func listLooping() error {
	items, err := core.FindLoopingItems(store)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		fmt.Println("No looping items.")
	} else {
		fmt.Println("── Looping items ──")
		for _, t := range items {
			fmt.Println(formatTodoLine(t))
		}
	}

	printInfoLine()
	return nil
}

func formatTodoLine(t *core.Todo) string {
	var b strings.Builder

	if t.Important {
		b.WriteString("!!! ")
	} else if t.Urgent {
		b.WriteString("!! ")
	}

	b.WriteString("[")
	b.WriteString(t.ID)
	b.WriteString("] ")

	if t.Done {
		b.WriteString("[x] ")
	} else {
		b.WriteString("[ ] ")
	}

	b.WriteString(t.Text)

	for _, tag := range t.Tags {
		b.WriteString(" #")
		b.WriteString(tag)
	}

	return b.String()
}

func filterByTag(todos []*core.Todo, tag string) []*core.Todo {
	var filtered []*core.Todo
	lower := strings.ToLower(tag)
	for _, t := range todos {
		for _, tt := range t.Tags {
			if strings.ToLower(tt) == lower {
				filtered = append(filtered, t)
				break
			}
		}
	}
	return filtered
}
