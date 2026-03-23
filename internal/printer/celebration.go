package printer

import (
	"fmt"
	"math/rand/v2"

	"github.com/anujp/braindump/internal/core"
)

// Short punchy celebrations (60% chance)
var shortCelebrations = []string{
	">>> DONE. %s -- gone.",
	"*** CRUSHED IT *** %s",
	"[x] Another one bites the dust: %s",
	"VICTORY. %s is HISTORY.",
	"=== COMPLETE === %s",
	">> SHIPPED << %s",
	"BOOM. %s -- destroyed.",
	"++ DONE ++ %s",
	"NAILED IT. %s -- checked off.",
	"--- FINISHED --- %s",
	"DONE AND DUSTED: %s",
	"CONQUERED: %s",
}

// Big ASCII art celebrations (30% chance)
var bigCelebrations = []string{
	"     *  .  *\n   .  *  .  *  .\n     * DONE! *\n   .  *  .  *  .\n     *  .  *\n\n  [x] %s\n  -- QUEST COMPLETE --",

	"  ___________\n |           |\n |   DONE!   |\n |    [x]    |\n |___________|\n    |     |\n  %s",

	"    \\o/\n     |\n    / \\\n  VICTORY!\n  %s",

	"  =========\n  | DONE! |\n  =========\n  %s",

	"    ***\n   *   *\n  * [x] *\n   *   *\n    ***\n  %s",
}

// Legendary celebrations (10% chance)
var legendaryCelebrations = []string{
	"=============================\n=                           =\n=   *** LEGENDARY DONE ***  =\n=                           =\n=  %s\n=                           =\n=============================",

	"  +-+-+-+-+-+-+-+-+-+-+-+-+-+\n  |L|E|G|E|N|D|A|R|Y|!|!|!|\n  +-+-+-+-+-+-+-+-+-+-+-+-+-+\n  %s",

	"  ###########################\n  #   LEGENDARY COMPLETE!   #\n  ###########################\n  #  %s\n  ###########################",
}

// RandomCelebration returns a random celebration message for completing a todo.
func RandomCelebration(todo *core.Todo) string {
	roll := rand.IntN(10)
	switch {
	case roll == 0: // 10%
		tmpl := legendaryCelebrations[rand.IntN(len(legendaryCelebrations))]
		return fmt.Sprintf(tmpl, todo.Text)
	case roll < 4: // 30%
		tmpl := bigCelebrations[rand.IntN(len(bigCelebrations))]
		return fmt.Sprintf(tmpl, todo.Text)
	default: // 60%
		tmpl := shortCelebrations[rand.IntN(len(shortCelebrations))]
		return fmt.Sprintf(tmpl, todo.Text)
	}
}
