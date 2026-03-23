package cli

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/anujp/braindump/internal/printer"
	"github.com/spf13/cobra"
)

var doneCmd = &cobra.Command{
	Use:   "done [id]",
	Short: "Mark a todo as done",
	Long: `Mark a todo as complete. Pass an ID directly, or omit it to pick
from today's open items interactively.`,
	Example: `  todo done a1b2c3              # mark by ID
  todo done                     # interactive picker from today`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDone,
}

func runDone(cmd *cobra.Command, args []string) error {
	if len(args) == 1 {
		return markDoneByID(args[0])
	}
	return markDoneInteractive()
}

func markDoneByID(id string) error {
	todo, date, err := store.FindTodoByID(id)
	if err != nil {
		return err
	}

	oldStatus := todo.Status
	todo.Status = "done"
	todo.StatusChanged = time.Now()
	todo.Done = true

	if err := store.UpdateTodo(todo); err != nil {
		return err
	}

	if err := logger.LogStatusChange(date, todo.ID, oldStatus, "done"); err != nil {
		return err
	}

	// Celebration for active items
	if oldStatus == "active" {
		fmt.Println(printer.RandomCelebration(todo))
	}

	fmt.Printf("Done: [%s] %s\n", todo.ID, todo.Text)
	printInfoLine()
	return nil
}

func markDoneInteractive() error {
	items, err := collectByStatus("active")
	if err != nil {
		return err
	}

	if len(items) == 0 {
		fmt.Println("No active items.")
		printInfoLine()
		return nil
	}

	scanner := bufio.NewScanner(os.Stdin)
	selected, err := pickFromList(items, "\nSelect number: ", scanner)
	if err != nil {
		return err
	}
	if selected == nil {
		return nil
	}

	selected.todo.Status = "done"
	selected.todo.StatusChanged = time.Now()
	selected.todo.Done = true

	if err := store.UpdateTodo(selected.todo); err != nil {
		return err
	}

	if err := logger.LogStatusChange(selected.date, selected.todo.ID, "active", "done"); err != nil {
		return err
	}

	fmt.Println(printer.RandomCelebration(selected.todo))
	fmt.Printf("Done: [%s] %s\n", selected.todo.ID, selected.todo.Text)
	printInfoLine()
	return nil
}
