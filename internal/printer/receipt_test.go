package printer

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/anujp/braindump/internal/core"
)

func testTodo() *core.Todo {
	return &core.Todo{
		ID:     "abc123",
		Text:   "Test task",
		Tags:   []string{"work"},
		Status: "active",
	}
}

func TestFormatReceipt_NonEmpty(t *testing.T) {
	data := FormatReceipt(testTodo(), 0)
	if len(data) == 0 {
		t.Error("FormatReceipt returned empty byte slice")
	}
}

func TestFormatReceipt_StartsWithInit(t *testing.T) {
	data := FormatReceipt(testTodo(), 0)
	if len(data) < 2 {
		t.Fatal("output too short to contain CmdInit")
	}
	if data[0] != 0x1b || data[1] != 0x40 {
		t.Errorf("output does not start with CmdInit (0x1b 0x40), got 0x%02X 0x%02X", data[0], data[1])
	}
}

func TestFormatReceipt_ContainsTodoText(t *testing.T) {
	todo := testTodo()
	data := FormatReceipt(todo, 0)
	if !bytes.Contains(data, []byte(todo.Text)) {
		t.Error("output does not contain todo text")
	}
}

func TestFormatReceipt_ContainsTodoID(t *testing.T) {
	todo := testTodo()
	data := FormatReceipt(todo, 0)
	if !bytes.Contains(data, []byte(todo.ID)) {
		t.Error("output does not contain todo ID")
	}
}

func TestFormatReceipt_ContainsTags(t *testing.T) {
	todo := &core.Todo{
		ID: "abc123", Text: "Tagged task", Tags: []string{"work", "homelab"}, Status: "active",
	}
	data := FormatReceipt(todo, 0)
	if !bytes.Contains(data, []byte("work")) {
		t.Error("output does not contain tag 'work'")
	}
	if !bytes.Contains(data, []byte("homelab")) {
		t.Error("output does not contain tag 'homelab'")
	}
}

func TestFormatReceipt_EndsWithLineFeeds(t *testing.T) {
	data := FormatReceipt(testTodo(), 0)
	if len(data) < 8 {
		t.Fatal("output too short")
	}
	// Count trailing 0x0a bytes
	feedCount := 0
	for i := len(data) - 1; i >= 0 && data[i] == 0x0a; i-- {
		feedCount++
	}
	if feedCount < 8 {
		t.Errorf("expected >= 8 trailing line feeds for manual tear, got %d", feedCount)
	}
}

func TestFormatReceipt_LongText_WordWrapped(t *testing.T) {
	todo := &core.Todo{
		ID:     "abc123",
		Text:   "This is a very long todo text that should be word wrapped because it exceeds thirty two characters",
		Tags:   []string{"work"},
		Status: "active",
	}
	data := FormatReceipt(todo, 0)

	// Strip ESC/POS commands and check printable line lengths
	printable := stripEscPos(data)
	lines := strings.Split(string(printable), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		runeCount := utf8.RuneCountInString(trimmed)
		if runeCount > 32 {
			t.Errorf("line exceeds 32 chars (%d): %q", runeCount, trimmed)
		}
	}
}

func TestFormatReceipt_EmptyTags_NoCrash(t *testing.T) {
	todo := &core.Todo{
		ID: "abc123", Text: "No tags", Tags: nil, Status: "active",
	}
	data := FormatReceipt(todo, 0)
	if len(data) == 0 {
		t.Error("FormatReceipt with nil tags returned empty output")
	}
}

func TestFormatReceipt_Urgent(t *testing.T) {
	todo := &core.Todo{
		ID: "abc123", Text: "Urgent task", Tags: []string{"work"}, Status: "active",
		Urgent: true,
	}
	data := FormatReceipt(todo, 0)
	upper := strings.ToUpper(string(data))
	if !strings.Contains(upper, "URGENT") && !bytes.Contains(data, []byte("!!")) {
		t.Error("urgent todo output should contain URGENT or !! marker")
	}
}

func TestFormatReceipt_Important(t *testing.T) {
	todo := &core.Todo{
		ID: "abc123", Text: "Important task", Tags: []string{"work"}, Status: "active",
		Important: true,
	}
	data := FormatReceipt(todo, 0)
	upper := strings.ToUpper(string(data))
	if !strings.Contains(upper, "IMPORTANT") && !bytes.Contains(data, []byte("!!!")) {
		t.Error("important todo output should contain IMPORTANT or !!! marker")
	}
}

func TestFormatReceipt_Randomization(t *testing.T) {
	todo := testTodo()
	// Try multiple times since there's a small chance of collision
	distinct := make(map[string]bool)
	for i := 0; i < 20; i++ {
		data := FormatReceipt(todo, 0)
		distinct[string(data)] = true
	}
	if len(distinct) < 2 {
		t.Errorf("FormatReceipt produced only %d distinct outputs over 20 calls, expected >= 2 (randomization)", len(distinct))
	}
}

func TestFormatReceipt_WithStreak_ContainsStreakText(t *testing.T) {
	data := FormatReceipt(testTodo(), 5)
	upper := strings.ToUpper(string(data))
	if !strings.Contains(upper, "STREAK") {
		t.Error("FormatReceipt with streak > 0 should contain STREAK text")
	}
}

func TestFormatReceipt_ZeroStreak_NoStreakText(t *testing.T) {
	data := FormatReceipt(testTodo(), 0)
	upper := strings.ToUpper(string(data))
	if strings.Contains(upper, "STREAK") {
		t.Error("FormatReceipt with streak == 0 should NOT contain STREAK text")
	}
}

func TestFormatReceipt_NoEmoji(t *testing.T) {
	for i := 0; i < 20; i++ {
		data := FormatReceipt(testTodo(), i%3)
		for j, b := range data {
			if b >= 0xF0 && b <= 0xF4 {
				t.Errorf("FormatReceipt contains emoji byte 0x%02X at position %d", b, j)
				break
			}
		}
	}
}

// stripEscPos removes ESC/POS command sequences from raw bytes, leaving only printable content.
func stripEscPos(data []byte) []byte {
	var result []byte
	for i := 0; i < len(data); i++ {
		if data[i] == 0x1b && i+1 < len(data) {
			// ESC commands: skip ESC + command byte + possible parameter bytes
			i++ // skip command byte
			switch data[i] {
			case 0x40: // Init (no params)
			case 0x45: // Bold on/off (1 param)
				i++
			case 0x61: // Alignment (1 param)
				i++
			default:
				// Unknown ESC command, skip just the two bytes
			}
			continue
		}
		result = append(result, data[i])
	}
	return result
}
