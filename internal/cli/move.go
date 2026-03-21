package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var validStatuses = map[string]bool{
	"unprocessed": true,
	"inbox":       true,
	"today":       true,
	"waiting":     true,
	"done":        true,
	"stale":       true,
}

var moveCmd = &cobra.Command{
	Use:   "move <id> <status>",
	Short: "Move a todo to a different status",
	Long: fmt.Sprintf("Move a todo to a new status. Valid statuses: %s",
		strings.Join([]string{"unprocessed", "inbox", "today", "waiting", "done", "stale"}, ", ")),
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		newStatus := args[1]

		if !validStatuses[newStatus] {
			return fmt.Errorf("invalid status %q; valid statuses: unprocessed, inbox, today, waiting, done, stale", newStatus)
		}

		todo, date, err := store.FindTodoByID(id)
		if err != nil {
			return err
		}

		oldStatus := todo.Status
		todo.Status = newStatus

		if newStatus == "done" {
			todo.Done = true
		}

		if err := logger.LogStatusChange(date, id, oldStatus, newStatus); err != nil {
			return err
		}

		if err := store.UpdateTodo(todo); err != nil {
			return err
		}

		fmt.Printf("moved %s: %s -> %s\n", id, oldStatus, newStatus)
		printInfoLine()
		return nil
	},
}
