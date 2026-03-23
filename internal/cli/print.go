package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anujp/braindump/internal/core"
	"github.com/anujp/braindump/internal/printer"
	"github.com/spf13/cobra"
)

const activeItemCap = 5

var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Pick a todo from inbox, print it, mark active",
	Long: `Select a todo from your inbox to work on. Prints a physical
receipt and marks the item as active. Active items go stale after 24 hours.`,
	RunE: runPrint,
}

func runPrint(cmd *cobra.Command, args []string) error {
	// Check soft cap
	info, err := core.GetInfoLine(store)
	if err != nil {
		return err
	}
	if info.Active >= activeItemCap {
		fmt.Printf("!! You have %d active items (cap: %d). Finish something first? [y/N] ", info.Active, activeItemCap)
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return nil
		}
		if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
			return nil
		}
	}

	// Collect inbox items
	items, err := collectByStatus("inbox")
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println("No inbox items to print.")
		printInfoLine()
		return nil
	}

	// Pick one
	scanner := bufio.NewScanner(os.Stdin)
	selected, err := pickFromList(items, "Select todo to print: ", scanner)
	if err != nil {
		return err
	}
	if selected == nil {
		return nil
	}

	// Mark active
	selected.todo.Status = "active"
	selected.todo.StatusChanged = time.Now()
	if err := store.UpdateTodo(selected.todo); err != nil {
		return err
	}
	if err := logger.LogStatusChange(selected.date, selected.todo.ID, "inbox", "active"); err != nil {
		return err
	}

	// Try to print receipt
	cfg, cfgErr := printer.LoadConfig(filepath.Join(dataDir, "printer.yaml"))
	if cfgErr != nil {
		fmt.Printf("!! Warning: could not load printer config: %v (using defaults)\n", cfgErr)
		cfg = printer.DefaultConfig()
	}
	var p printer.Printer = &printer.ESCPOSPrinter{DevicePath: cfg.DevicePath}

	if cfg.Enabled && p.Available() {
		receipt := printer.FormatReceipt(selected.todo, 0)
		if err := p.Print(receipt); err != nil {
			fmt.Printf("!! Print failed: %v (marked active without printing)\n", err)
		} else {
			fmt.Printf(">>> Printed receipt for [%s] %s\n", selected.todo.ID, selected.todo.Text)
		}
	} else {
		fmt.Printf("[*] Activated [%s] %s (printer offline)\n", selected.todo.ID, selected.todo.Text)
	}

	printInfoLine()
	return nil
}
