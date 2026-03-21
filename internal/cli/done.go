package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/anujp/braindump/internal/core"
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
	todo.Done = true

	if err := store.UpdateTodo(todo); err != nil {
		return err
	}

	if err := logger.LogStatusChange(date, todo.ID, oldStatus, "done"); err != nil {
		return err
	}

	fmt.Printf("Done: [%s] %s\n", todo.ID, todo.Text)
	printInfoLine()
	return nil
}

func markDoneInteractive() error {
	today := time.Now().Format("2006-01-02")
	todos, err := store.ReadDay(today)
	if err != nil {
		return err
	}

	// Filter to non-done items.
	var open []*openItem
	for _, t := range todos {
		if !t.Done {
			open = append(open, &openItem{todo: t, date: today})
		}
	}

	if len(open) == 0 {
		fmt.Println("No open items today.")
		printInfoLine()
		return nil
	}

	// Present numbered list.
	for i, item := range open {
		fmt.Printf("  %d) [%s] %s\n", i+1, item.todo.ID, item.todo.Text)
	}

	fmt.Print("\nSelect number: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil
	}

	input := strings.TrimSpace(scanner.Text())
	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(open) {
		return fmt.Errorf("invalid selection: %s", input)
	}

	selected := open[num-1]
	oldStatus := selected.todo.Status
	selected.todo.Status = "done"
	selected.todo.Done = true

	if err := store.UpdateTodo(selected.todo); err != nil {
		return err
	}

	if err := logger.LogStatusChange(selected.date, selected.todo.ID, oldStatus, "done"); err != nil {
		return err
	}

	fmt.Printf("Done: [%s] %s\n", selected.todo.ID, selected.todo.Text)
	printInfoLine()
	return nil
}

type openItem struct {
	todo *core.Todo
	date string
}
