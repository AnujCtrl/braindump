package printer

import (
	"github.com/anujp/braindump/internal/core"
)

// FormatReceipt formats a todo into ESC/POS receipt bytes.
// streak is the number of consecutive days the user has completed todos.
func FormatReceipt(todo *core.Todo, streak int) []byte {
	return nil
}
