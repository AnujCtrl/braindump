package cli

import (
	"fmt"

	"github.com/anujp/braindump/internal/core"
	"github.com/spf13/cobra"
)

var staleCmd = &cobra.Command{
	Use:   "stale [revive <id>]",
	Short: "Show stale todos or revive one",
	Long: `Show inbox items untouched for 7+ days, or revive one back to inbox.
Reviving increments stale_count — items that go stale twice are "looping".`,
	Example: `  todo stale                    # list stale items
  todo stale revive a1b2c3      # move back to inbox`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle "stale revive <id>"
		if len(args) >= 1 && args[0] == "revive" {
			if len(args) < 2 {
				return fmt.Errorf("revive requires a todo ID")
			}
			return runRevive(args[1])
		}

		// No args: list stale items
		items, err := core.FindStaleItems(store)
		if err != nil {
			return err
		}

		if len(items) == 0 {
			fmt.Println("no stale items")
		} else {
			for _, todo := range items {
				printTodoLine(todo)
			}
		}

		printInfoLine()
		return nil
	},
}

func runRevive(id string) error {
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		return err
	}

	if err := core.ReviveTodo(store, logger, todo); err != nil {
		return err
	}

	fmt.Printf("revived %s: %s (stale_count: %d)\n", id, todo.Text, todo.StaleCount)
	printInfoLine()
	return nil
}

var loopingCmd = &cobra.Command{
	Use:   "looping",
	Short: "Show looping todos (stale_count >= 2)",
	Long: `Show todos stuck in stale loops (gone stale 2+ times).
These need a decision: do it, break it down, or drop it.`,
	Example: `  todo looping`,
	RunE: func(cmd *cobra.Command, args []string) error {
		items, err := core.FindLoopingItems(store)
		if err != nil {
			return err
		}

		if len(items) == 0 {
			fmt.Println("no looping items")
		} else {
			for _, todo := range items {
				fmt.Printf("[%s] %s (stale_count: %d)\n", todo.ID, todo.Text, todo.StaleCount)
			}
		}

		printInfoLine()
		return nil
	},
}

// printTodoLine prints a single todo in list format.
func printTodoLine(todo *core.Todo) {
	checkbox := "[ ]"
	if todo.Done {
		checkbox = "[x]"
	}

	tags := ""
	for _, t := range todo.Tags {
		tags += " #" + t
	}

	urgency := ""
	if todo.Urgent {
		urgency += " !!"
	}
	if todo.Important {
		urgency += " !!!"
	}

	fmt.Printf("%s %s %s%s%s\n", todo.ID, checkbox, todo.Text, tags, urgency)
}
