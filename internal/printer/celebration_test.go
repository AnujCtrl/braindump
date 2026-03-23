package printer

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/anujp/braindump/internal/core"
)

func celebrationTodo() *core.Todo {
	return &core.Todo{
		ID:   "abc123",
		Text: "Fix server",
	}
}

func TestRandomCelebration_NonEmpty(t *testing.T) {
	result := RandomCelebration(celebrationTodo())
	if result == "" {
		t.Error("RandomCelebration returned empty string")
	}
}

func TestRandomCelebration_ContainsTodoText(t *testing.T) {
	todo := celebrationTodo()
	result := RandomCelebration(todo)
	if !strings.Contains(result, todo.Text) {
		t.Errorf("celebration should contain todo text %q, got: %q", todo.Text, result)
	}
}

func TestRandomCelebration_Variety(t *testing.T) {
	todo := celebrationTodo()
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		seen[RandomCelebration(todo)] = true
	}
	if len(seen) < 5 {
		t.Errorf("RandomCelebration produced only %d distinct values over 50 calls, want >= 5", len(seen))
	}
}

func TestRandomCelebration_LineLengths(t *testing.T) {
	todo := celebrationTodo()
	for i := 0; i < 30; i++ {
		result := RandomCelebration(todo)
		for lineNum, line := range strings.Split(result, "\n") {
			runeCount := utf8.RuneCountInString(line)
			if runeCount > 80 {
				t.Errorf("celebration line %d is %d chars (> 80): %q", lineNum, runeCount, line)
			}
		}
	}
}

func TestRandomCelebration_NoEmoji(t *testing.T) {
	todo := celebrationTodo()
	for i := 0; i < 30; i++ {
		result := RandomCelebration(todo)
		for j, b := range []byte(result) {
			if b >= 0xF0 && b <= 0xF4 {
				t.Errorf("celebration contains emoji byte 0x%02X at pos %d: %q", b, j, result)
				break
			}
		}
	}
}

func TestRandomCelebration_NoSprintfArtifacts(t *testing.T) {
	todo := celebrationTodo()
	for i := 0; i < 50; i++ {
		result := RandomCelebration(todo)
		if strings.Contains(result, "%!") {
			t.Errorf("celebration contains Sprintf artifact '%%!': %q", result)
		}
	}
}
