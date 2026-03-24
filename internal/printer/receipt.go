package printer

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/anujp/braindump/internal/core"
)

const receiptWidth = 32

// buildReceiptScript generates a line-based command script for the Node.js
// ESC/POS encoder. Pure function -- no side effects, fully testable without Node.js.
//
// Command protocol: CENTER, LEFT, BOLD ON, BOLD OFF, TEXT <content>, FEED <n>
func buildReceiptScript(todo *core.Todo, streak int, width int) string {
	var b strings.Builder
	cmd := func(s string) { b.WriteString(s); b.WriteByte('\n') }
	text := func(s string) { cmd("TEXT " + s) }

	legendary := IsLegendary()

	// --- Header (bold, centered in Go) ---
	cmd("BOLD ON")

	border := RandomBorder(width)
	if legendary {
		border = strings.Repeat("*", width)
	}
	text(border)

	header := RandomHeader()
	if legendary {
		header = "*** LEGENDARY TICKET ***"
	}
	text(centerText(header, width))
	text(border)

	cmd("BOLD OFF")

	// Time greeting + day flavor
	text(centerText(RandomTimeGreeting(), width))
	if flavor := RandomDayFlavor(); flavor != "" {
		text(centerText(flavor, width))
	}
	text(centerText(RandomMessage(), width))
	text(strings.Repeat("-", width))

	// --- Body ---
	cmd("TEXT") // blank line

	// Urgent/Important markers
	if todo.Urgent {
		text("!! URGENT !!")
	}
	if todo.Important {
		text("!!! IMPORTANT !!!")
	}

	// Todo text (word-wrapped)
	for _, line := range wordWrap(todo.Text, width) {
		text(line)
	}
	cmd("TEXT") // blank line

	// Metadata
	text(fmt.Sprintf("ID: %s", todo.ID))
	if len(todo.Tags) > 0 {
		tagStr := ""
		for _, tag := range todo.Tags {
			tagStr += " #" + tag
		}
		text("Tags:" + tagStr)
	}
	text(time.Now().Format("2006-01-02 15:04"))

	// Streak
	if streak > 0 {
		text(fmt.Sprintf("STREAK: %d days", streak))
	}

	cmd("TEXT") // blank line

	// ASCII art
	if legendary {
		text("=== LEGENDARY ===")
	}
	art := RandomArt()
	for _, line := range strings.Split(art, "\n") {
		text(line)
	}

	text(strings.Repeat("-", width))

	// --- Footer (centered in Go) ---
	text(centerText(fmt.Sprintf("* %s *", RandomSignoff()), width))
	text(border)

	// Tear margin
	cmd("FEED 3")

	return b.String()
}

// FormatReceipt formats a todo into ESC/POS receipt bytes by shelling out
// to a Node.js encoder script. Returns the ESC/POS bytes, the human-readable
// command script (for debugging), and any error.
func FormatReceipt(todo *core.Todo, streak int, encoderScript string, width int) ([]byte, string, error) {
	if encoderScript == "" {
		return nil, "", fmt.Errorf("encoder script path is empty")
	}

	script := buildReceiptScript(todo, streak, width)

	cmd := exec.Command("node", encoderScript)
	cmd.Stdin = strings.NewReader(script)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, script, fmt.Errorf("node encoder: %w: %s", err, stderr.String())
	}

	return stdout.Bytes(), script, nil
}

// FormatPlainReceipt formats a todo into plain ASCII bytes with no ESC/POS commands.
// Safe for printers that don't support ESC/POS (e.g., Analog Devices H58).
func FormatPlainReceipt(todo *core.Todo, streak int, width int) []byte {
	var buf []byte
	writeln := func(s string) { buf = append(buf, []byte(s)...); buf = append(buf, '\n') }

	legendary := IsLegendary()

	// --- Header ---
	border := RandomBorder(width)
	if legendary {
		border = strings.Repeat("*", width)
	}
	writeln(border)

	header := RandomHeader()
	if legendary {
		header = "*** LEGENDARY TICKET ***"
	}
	writeln(centerText(header, width))
	writeln(border)

	// Time greeting + day flavor
	writeln(RandomTimeGreeting())
	if flavor := RandomDayFlavor(); flavor != "" {
		writeln(flavor)
	}
	writeln(RandomMessage())
	writeln(strings.Repeat("-", width))

	// --- Body ---
	writeln("")

	if todo.Urgent {
		writeln("!! URGENT !!")
	}
	if todo.Important {
		writeln("!!! IMPORTANT !!!")
	}

	for _, line := range wordWrap(todo.Text, width) {
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

	writeln(strings.Repeat("-", width))

	// --- Footer ---
	writeln(centerText(fmt.Sprintf("* %s *", RandomSignoff()), width))
	writeln(border)

	// Line feeds for tear margin (~1 inch on 58mm thermal at ~8 lines/inch)
	for i := 0; i < 3; i++ {
		buf = append(buf, '\n')
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

// centerText pads a string with spaces on both sides to center it within width.
// Returns a string of exactly width characters (or the original if already >= width).
func centerText(s string, width int) string {
	if len(s) >= width {
		return s
	}
	totalPad := width - len(s)
	leftPad := totalPad / 2
	rightPad := totalPad - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}
