package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anujp/braindump/internal/core"
	"github.com/spf13/cobra"
)

var (
	store    *core.Store
	logger   *core.Logger
	tagStore *core.TagStore
	dataDir  string
)

// Reserved subcommand names that cannot be used as capture text.
var reservedNames = map[string]bool{
	"ls": true, "done": true, "edit": true, "delete": true,
	"move": true, "dump": true, "tag": true, "stale": true, "looping": true,
}

var RootCmd = &cobra.Command{
	Use:                "todo",
	Short:              "Fast brain dump capture system",
	Long:               "A capture-first todo system designed for low friction brain dumps.",
	Args:             cobra.ArbitraryArgs,
	TraverseChildren: true,
	FParseErrWhitelist: cobra.FParseErrWhitelist{
		UnknownFlags: true,
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		dataDir = os.Getenv("TODO_DATA_DIR")
		if dataDir == "" {
			dataDir = "./data"
		}

		store = core.NewStore(dataDir)
		logger = core.NewLogger(dataDir)

		tagsPath := filepath.Join(dataDir, "tags.yaml")
		var err error
		tagStore, err = core.NewTagStore(tagsPath)
		if err != nil {
			return fmt.Errorf("loading tags: %w", err)
		}

		if err := store.BackfillGaps(); err != nil {
			return fmt.Errorf("backfilling gaps: %w", err)
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Usage()
		}

		// Special handling for # and @
		if args[0] == "#" {
			grouped := tagStore.GroupedTags()
			for group, tags := range grouped {
				fmt.Printf("%s:\n", group)
				for _, t := range tags {
					fmt.Printf("  #%s\n", t)
				}
			}
			return nil
		}

		if args[0] == "@" {
			fmt.Println("cli")
			fmt.Println("api")
			fmt.Println("minecraft")
			return nil
		}

		return runCapture(cmd, args)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func printInfoLine() {
	info, err := core.GetInfoLine(store)
	if err != nil {
		return
	}
	line := core.FormatInfoLine(info)
	if line != "" {
		fmt.Println(line)
	}
}

func init() {
	RootCmd.Flags().StringP("note", "", "", "attach a note to the captured todo")

	RootCmd.AddCommand(lsCmd)
	RootCmd.AddCommand(doneCmd)
	RootCmd.AddCommand(editCmd)
	RootCmd.AddCommand(deleteCmd)
	RootCmd.AddCommand(moveCmd)
	RootCmd.AddCommand(dumpCmd)
	RootCmd.AddCommand(tagCmd)
	RootCmd.AddCommand(staleCmd)
	RootCmd.AddCommand(loopingCmd)
}

