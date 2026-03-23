package printer

import (
	"fmt"
	"strings"
	"time"

	"github.com/anujp/braindump/internal/core"
)

const receiptWidth = 32

// FormatReceipt formats a todo into ESC/POS receipt bytes.
// streak is the number of consecutive days the user has completed todos.
func FormatReceipt(todo *core.Todo, streak int) []byte {
	var buf []byte

	// Helper to append bytes
	write := func(b []byte) { buf = append(buf, b...) }
	writeln := func(s string) { buf = append(buf, []byte(s)...); buf = append(buf, 0x0a) }

	// Init printer
	write(CmdInit)

	// Check legendary
	legendary := IsLegendary()

	// --- Header section (centered, bold) ---
	write(CmdCenter)
	write(CmdBoldOn)

	border := RandomBorder(receiptWidth)
	if legendary {
		border = strings.Repeat("*", receiptWidth)
	}
	writeln(border)

	header := RandomHeader()
	if legendary {
		header = "*** LEGENDARY TICKET ***"
	}
	writeln(centerText(header, receiptWidth))
	writeln(border)

	write(CmdBoldOff)

	// Time greeting + day flavor
	writeln(RandomTimeGreeting())
	if flavor := RandomDayFlavor(); flavor != "" {
		writeln(flavor)
	}
	writeln(RandomMessage())
	writeln(strings.Repeat("-", receiptWidth))

	// --- Body section (left-aligned) ---
	write(CmdLeft)
	writeln("")

	// Urgent/Important markers
	if todo.Urgent {
		writeln("!! URGENT !!")
	}
	if todo.Important {
		writeln("!!! IMPORTANT !!!")
	}

	// Todo text (word-wrapped)
	for _, line := range wordWrap(todo.Text, receiptWidth) {
		writeln(line)
	}
	writeln("")

	// Metadata
	writeln(fmt.Sprintf("ID: %s", todo.ID))
	if len(todo.Tags) > 0 {
		tagStr := ""
		for _, tag := range todo.Tags {
			tagStr += " #" + tag
		}
		writeln("Tags:" + tagStr)
	}
	writeln(time.Now().Format("2006-01-02 15:04"))

	// Streak
	if streak > 0 {
		writeln(fmt.Sprintf("STREAK: %d days", streak))
	}

	writeln("")

	// ASCII art
	if legendary {
		writeln("=== LEGENDARY ===")
	}
	art := RandomArt()
	for _, line := range strings.Split(art, "\n") {
		writeln(line)
	}

	writeln(strings.Repeat("-", receiptWidth))

	// --- Footer (centered) ---
	write(CmdCenter)
	writeln(fmt.Sprintf("* %s *", RandomSignoff()))
	writeln(border)

	// Line feeds for tear (>= 8)
	for i := 0; i < 10; i++ {
		write(CmdFeed)
	}

	return buf
}

// wordWrap splits text into lines no wider than maxWidth characters.
// Words longer than maxWidth are split at the character boundary.
func wordWrap(text string, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}

	// splitLong breaks a single word into chunks of at most maxWidth chars.
	splitLong := func(word string) []string {
		if len(word) <= maxWidth {
			return []string{word}
		}
		var chunks []string
		for len(word) > maxWidth {
			chunks = append(chunks, word[:maxWidth])
			word = word[maxWidth:]
		}
		if len(word) > 0 {
			chunks = append(chunks, word)
		}
		return chunks
	}

	var lines []string
	current := ""

	for _, word := range words {
		parts := splitLong(word)
		for _, part := range parts {
			if current == "" {
				current = part
			} else if len(current)+1+len(part) <= maxWidth {
				current += " " + part
			} else {
				lines = append(lines, current)
				current = part
			}
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// centerText pads a string to center it within width.
func centerText(s string, width int) string {
	if len(s) >= width {
		return s
	}
	pad := (width - len(s)) / 2
	return strings.Repeat(" ", pad) + s
}
