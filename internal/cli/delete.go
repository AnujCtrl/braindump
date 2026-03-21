package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a todo",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		todo, date, err := store.FindTodoByID(id)
		if err != nil {
			return err
		}

		if err := store.DeleteTodo(id); err != nil {
			return err
		}

		if err := logger.LogDelete(date, id); err != nil {
			return err
		}

		fmt.Printf("deleted %s: %s\n", id, todo.Text)
		printInfoLine()
		return nil
	},
}
