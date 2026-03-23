package printer

import (
	"math/rand/v2"
	"strings"
	"time"
)

var headers = []string{
	"QUEST ACTIVE",
	"WORK ORDER",
	"MISSION BRIEF",
	"TODAY'S BOSS FIGHT",
	"TICKET TO RIDE",
	"BRAIN ACTIVATED",
	"FOCUS LOCKED",
	"OBJECTIVE SET",
	"NEW OBJECTIVE",
	"TARGET ACQUIRED",
}

var messages = []string{
	"you got this.",
	"ship it.",
	"one thing at a time.",
	"future you says thanks.",
	"momentum is everything.",
	"done > perfect.",
	"begin.",
	"lock in.",
	"trust the process.",
	"just start.",
	"keep it moving.",
	"do the next thing.",
	"one bite at a time.",
	"you've done harder.",
	"action beats anxiety.",
	"good enough is good.",
	"focus is a superpower.",
	"now or never.",
	"tick tock.",
	"make it happen.",
	"no zero days.",
	"prove them wrong.",
	"be relentless.",
	"grind time.",
	"execute.",
}

var signoffs = []string{
	"GO FORTH",
	"LOCK IN",
	"LET'S GO",
	"BEGIN",
	"EXECUTE",
	"DO THE THING",
	"COMMENCE",
	"ENGAGE",
	"INITIATE",
	"LAUNCH",
}

var borderChars = []string{
	"=", "~", "#", "*", "+", "-", ".", ">",
}

var receiptArt = []string{
	// Cat
	"  /\\_/\\\n (o o )\n  > ^ <",
	// Cow
	"   __\n  (oo)\n  /--\\\n / |  |",
	// Shield
	"  /----\\\n |      |\n |  ()  |\n  \\----/",
	// Mountain
	"    /\\\n   /  \\\n  / /\\ \\\n /______\\",
	// Checkbox
	" +------+\n |      |\n |  OK  |\n +------+",
	// Diamond
	"    *\n   * *\n  *   *\n   * *\n    *",
	// Rocket
	"    /\\\n   /  \\\n  | () |\n  |    |\n  /----\\",
	// Bunny
	" (\\  /)\n ( >.< )\n (\")(\")",
	// Flag
	" +----\n |####\n |####\n |\n |",
	// Star
	"    *\n  * * *\n *******\n  * * *\n    *",
	// Trophy
	"  _____\n |     |\n |     |\n  \\___/\n   | |\n  _|_|_",
	// Sword
	"   /\\\n  /  \\\n |    |\n  \\  /\n   \\/\n    |",
	// Potion
	"   _\n  | |\n / ~ \\\n|~~~~~|\n \\_O_/",
	// Smiley
	"  -----\n (o   o)\n  \\ ^ /\n   ---",
	// Brick wall
	" +--+--+--+\n |  |  |  |\n +--+--+--+\n |  |  |  |\n +--+--+--+",
	// Zen
	"  _   _\n (.) (.)\n   \\_/\n   |||",
	// Chalice
	"  /---\\\n /     \\\n|       |\n \\     /\n  |   |\n  |___|",
}

// RandomMessage returns a random motivational message for receipts.
func RandomMessage() string {
	return messages[rand.IntN(len(messages))]
}

// RandomBorder returns a decorative border string of the given width.
func RandomBorder(width int) string {
	ch := borderChars[rand.IntN(len(borderChars))]
	return strings.Repeat(ch, width)
}

// RandomArt returns random ASCII art that fits on a receipt (<=32 chars wide).
func RandomArt() string {
	return receiptArt[rand.IntN(len(receiptArt))]
}

// RandomHeader returns a random header string for receipts.
func RandomHeader() string {
	return headers[rand.IntN(len(headers))]
}

// RandomSignoff returns a random sign-off message.
func RandomSignoff() string {
	return signoffs[rand.IntN(len(signoffs))]
}

// RandomTimeGreeting returns a greeting based on the current time of day.
func RandomTimeGreeting() string {
	hour := time.Now().Hour()
	switch {
	case hour < 6:
		return "NIGHT OWL MODE"
	case hour < 12:
		return "EARLY BIRD MODE"
	case hour < 17:
		return "AFTERNOON GRIND"
	default:
		return "EVENING SESSION"
	}
}

// RandomDayFlavor returns a flavor string based on the current day of week.
func RandomDayFlavor() string {
	switch time.Now().Weekday() {
	case time.Monday:
		return "FRESH START"
	case time.Friday:
		return "FINISH STRONG"
	case time.Saturday, time.Sunday:
		return "WEEKEND WARRIOR"
	default:
		return ""
	}
}

// IsLegendary returns true with approximately 5% probability.
func IsLegendary() bool {
	return rand.IntN(20) == 0
}
