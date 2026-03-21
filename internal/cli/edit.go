package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit <id> <new text> [#tags] [!! !!!]",
	Short: "Edit a todo",
	Long:  `Edit a todo's text, tags, or priority flags.`,
	Example: `  todo edit a1b2c3 fix the AE2 autocrafter #minecraft #deep-focus
  todo edit a1b2c3 renew SSL cert #homelab !!
  todo edit a1b2c3 weekly grocery run #errands`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		todo, date, err := store.FindTodoByID(id)
		if err != nil {
			return err
		}

		// Parse remaining args: extract text, tags, urgent, important
		var textParts []string
		var tags []string
		urgent := todo.Urgent
		important := todo.Important

		for _, arg := range args[1:] {
			switch {
			case arg == "!!!":
				important = true
			case arg == "!!":
				urgent = true
			case strings.HasPrefix(arg, "#"):
				tag := strings.TrimPrefix(arg, "#")
				if tag != "" {
					tags = append(tags, tag)
				}
			default:
				textParts = append(textParts, arg)
			}
		}

		newText := strings.Join(textParts, " ")

		// Log edits for changed fields
		if newText != todo.Text {
			if err := logger.LogEdit(date, id, "text", todo.Text, newText); err != nil {
				return err
			}
			todo.Text = newText
		}

		if len(tags) > 0 {
			oldTags := strings.Join(todo.Tags, ",")
			newTags := strings.Join(tags, ",")
			if oldTags != newTags {
				if err := logger.LogEdit(date, id, "tags", oldTags, newTags); err != nil {
					return err
				}
				todo.Tags = tags
			}
		}

		if urgent != todo.Urgent {
			old := boolStr(todo.Urgent)
			new := boolStr(urgent)
			if err := logger.LogEdit(date, id, "urgent", old, new); err != nil {
				return err
			}
			todo.Urgent = urgent
		}

		if important != todo.Important {
			old := boolStr(todo.Important)
			new := boolStr(important)
			if err := logger.LogEdit(date, id, "important", old, new); err != nil {
				return err
			}
			todo.Important = important
		}

		if err := store.UpdateTodo(todo); err != nil {
			return err
		}

		fmt.Printf("edited %s: %s\n", id, todo.Text)
		printInfoLine()
		return nil
	},
}

func boolStr(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
