package printer

import (
	"testing"
	"unicode/utf8"
)

// hasEmojiBytes checks if a string contains emoji leading bytes (0xF0-0xF4).
func hasEmojiBytes(s string) bool {
	for _, b := range []byte(s) {
		if b >= 0xF0 && b <= 0xF4 {
			return true
		}
	}
	return false
}

func TestRandomMessage_NonEmpty(t *testing.T) {
	msg := RandomMessage()
	if msg == "" {
		t.Error("RandomMessage() returned empty string")
	}
}

func TestRandomMessage_Variety(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		seen[RandomMessage()] = true
	}
	if len(seen) < 3 {
		t.Errorf("RandomMessage() produced only %d distinct values over 50 calls, want >= 3", len(seen))
	}
}

func TestRandomMessage_MaxLength(t *testing.T) {
	for i := 0; i < 100; i++ {
		msg := RandomMessage()
		if len(msg) > 30 {
			t.Errorf("RandomMessage() returned string of len %d (> 30): %q", len(msg), msg)
		}
	}
}

func TestRandomBorder_ExactWidth(t *testing.T) {
	border := RandomBorder(32)
	runeCount := utf8.RuneCountInString(border)
	if runeCount != 32 {
		t.Errorf("RandomBorder(32) has %d visible chars, want 32: %q", runeCount, border)
	}
}

func TestRandomBorder_Width1(t *testing.T) {
	border := RandomBorder(1)
	runeCount := utf8.RuneCountInString(border)
	if runeCount != 1 {
		t.Errorf("RandomBorder(1) has %d visible chars, want 1: %q", runeCount, border)
	}
}

func TestRandomBorder_Variety(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		seen[RandomBorder(32)] = true
	}
	if len(seen) < 2 {
		t.Errorf("RandomBorder produced only %d distinct values over 20 calls, want >= 2", len(seen))
	}
}

func TestRandomArt_NonEmpty(t *testing.T) {
	art := RandomArt()
	if art == "" {
		t.Error("RandomArt() returned empty string")
	}
}

func TestRandomArt_LineLengths(t *testing.T) {
	for i := 0; i < 20; i++ {
		art := RandomArt()
		for lineNum, line := range splitLines(art) {
			runeCount := utf8.RuneCountInString(line)
			if runeCount > 32 {
				t.Errorf("RandomArt() line %d is %d chars (> 32): %q", lineNum, runeCount, line)
			}
		}
	}
}

func TestRandomHeader_NonEmpty(t *testing.T) {
	header := RandomHeader()
	if header == "" {
		t.Error("RandomHeader() returned empty string")
	}
}

func TestRandomHeader_Variety(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 30; i++ {
		seen[RandomHeader()] = true
	}
	if len(seen) < 3 {
		t.Errorf("RandomHeader() produced only %d distinct values over 30 calls, want >= 3", len(seen))
	}
}

func TestRandomSignoff_NonEmpty(t *testing.T) {
	signoff := RandomSignoff()
	if signoff == "" {
		t.Error("RandomSignoff() returned empty string")
	}
}

func TestRandomTimeGreeting_NonEmpty(t *testing.T) {
	greeting := RandomTimeGreeting()
	if greeting == "" {
		t.Error("RandomTimeGreeting() returned empty string")
	}
}

func TestRandomDayFlavor_DoesNotPanic(t *testing.T) {
	// RandomDayFlavor depends on the current day, so we just verify it doesn't panic
	// and returns a string (may be empty on weekdays).
	_ = RandomDayFlavor()
}

func TestIsLegendary_Probability(t *testing.T) {
	count := 0
	runs := 1000
	for i := 0; i < runs; i++ {
		if IsLegendary() {
			count++
		}
	}
	// Expected: ~2-10% = 20-100 hits out of 1000
	if count < 20 || count > 100 {
		t.Errorf("IsLegendary() returned true %d/%d times, expected 20-100 (2-10%%)", count, runs)
	}
}

func TestRandomFunctions_NoEmoji(t *testing.T) {
	// Test all Random* functions for emoji bytes
	funcs := map[string]func() string{
		"RandomMessage":      RandomMessage,
		"RandomArt":          RandomArt,
		"RandomHeader":       RandomHeader,
		"RandomSignoff":      RandomSignoff,
		"RandomTimeGreeting": RandomTimeGreeting,
		"RandomDayFlavor":    RandomDayFlavor,
	}
	for name, fn := range funcs {
		for i := 0; i < 20; i++ {
			result := fn()
			if hasEmojiBytes(result) {
				t.Errorf("%s() contains emoji bytes: %q", name, result)
			}
		}
	}
	// Also test RandomBorder
	for i := 0; i < 20; i++ {
		border := RandomBorder(32)
		if hasEmojiBytes(border) {
			t.Errorf("RandomBorder() contains emoji bytes: %q", border)
		}
	}
}

// splitLines splits a string into lines, handling both \n and \r\n.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
