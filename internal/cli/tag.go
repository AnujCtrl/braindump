package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage tags",
	Long:  `Manage your tag taxonomy. Tags are defined in tags.yaml.`,
	Example: `  todo tag list                 # show all tags by group
  todo tag add cooking          # add a new tag`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	},
}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tags grouped by category",
	RunE: func(cmd *cobra.Command, args []string) error {
		grouped := tagStore.GroupedTags()
		for group, tags := range grouped {
			fmt.Printf("%s:\n", group)
			for _, t := range tags {
				fmt.Printf("  #%s\n", t)
			}
		}
		return nil
	},
}

var tagAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if err := tagStore.AddTag(name, ""); err != nil {
			return err
		}

		fmt.Printf("added tag: #%s\n", name)
		return nil
	},
}

func init() {
	tagCmd.AddCommand(tagListCmd)
	tagCmd.AddCommand(tagAddCmd)
}
