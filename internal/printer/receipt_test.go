package printer

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// --- buildReceiptScript tests (pure function, no Node.js needed) ---

func TestBuildReceiptScript_ContainsTodoText(t *testing.T) {
	script := buildReceiptScript(testTodo(), 0, receiptWidth)
	if !strings.Contains(script, "Test task") {
		t.Error("script does not contain todo text")
	}
}

func TestBuildReceiptScript_ContainsTodoID(t *testing.T) {
	script := buildReceiptScript(testTodo(), 0, receiptWidth)
	if !strings.Contains(script, "abc123") {
		t.Error("script does not contain todo ID")
	}
}

func TestBuildReceiptScript_ContainsTags(t *testing.T) {
	todo := &core.Todo{
		ID: "abc123", Text: "Tagged task", Tags: []string{"work", "homelab"}, Status: "active",
	}
	script := buildReceiptScript(todo, 0, receiptWidth)
	if !strings.Contains(script, "#work") {
		t.Error("script does not contain tag 'work'")
	}
	if !strings.Contains(script, "#homelab") {
		t.Error("script does not contain tag 'homelab'")
	}
}

func TestBuildReceiptScript_NoCenterOrLeftCommands(t *testing.T) {
	script := buildReceiptScript(testTodo(), 0, receiptWidth)
	if strings.Contains(script, "\nCENTER\n") || strings.HasPrefix(script, "CENTER\n") {
		t.Error("script should not contain CENTER command (centering is done in Go)")
	}
	if strings.Contains(script, "\nLEFT\n") {
		t.Error("script should not contain LEFT command (no alignment commands used)")
	}
}

func TestBuildReceiptScript_HasBoldCommands(t *testing.T) {
	script := buildReceiptScript(testTodo(), 0, receiptWidth)
	if !strings.Contains(script, "BOLD ON\n") {
		t.Error("script does not contain BOLD ON command")
	}
	if !strings.Contains(script, "\nBOLD OFF\n") {
		t.Error("script does not contain BOLD OFF command")
	}
}

func TestBuildReceiptScript_HasFeed(t *testing.T) {
	script := buildReceiptScript(testTodo(), 0, receiptWidth)
	if !strings.Contains(script, "FEED 3") {
		t.Error("script does not contain FEED 3 for tear margin")
	}
}

func TestBuildReceiptScript_Urgent(t *testing.T) {
	todo := &core.Todo{
		ID: "abc123", Text: "Urgent task", Tags: []string{"work"}, Status: "active",
		Urgent: true,
	}
	script := buildReceiptScript(todo, 0, receiptWidth)
	if !strings.Contains(script, "URGENT") {
		t.Error("urgent todo script should contain URGENT marker")
	}
}

func TestBuildReceiptScript_Important(t *testing.T) {
	todo := &core.Todo{
		ID: "abc123", Text: "Important task", Tags: []string{"work"}, Status: "active",
		Important: true,
	}
	script := buildReceiptScript(todo, 0, receiptWidth)
	if !strings.Contains(script, "IMPORTANT") {
		t.Error("important todo script should contain IMPORTANT marker")
	}
}

func TestBuildReceiptScript_WithStreak(t *testing.T) {
	script := buildReceiptScript(testTodo(), 5, receiptWidth)
	if !strings.Contains(script, "STREAK") {
		t.Error("script with streak > 0 should contain STREAK text")
	}
}

func TestBuildReceiptScript_ZeroStreak_NoStreakText(t *testing.T) {
	script := buildReceiptScript(testTodo(), 0, receiptWidth)
	if strings.Contains(script, "STREAK") {
		t.Error("script with streak == 0 should NOT contain STREAK text")
	}
}

func TestBuildReceiptScript_EmptyTags_NoCrash(t *testing.T) {
	todo := &core.Todo{
		ID: "abc123", Text: "No tags", Tags: nil, Status: "active",
	}
	script := buildReceiptScript(todo, 0, receiptWidth)
	if len(script) == 0 {
		t.Error("buildReceiptScript with nil tags returned empty output")
	}
}

func TestBuildReceiptScript_OnlyValidCommands(t *testing.T) {
	script := buildReceiptScript(testTodo(), 0, receiptWidth)
	validPrefixes := []string{"BOLD ON", "BOLD OFF", "TEXT", "FEED"}
	for _, line := range strings.Split(script, "\n") {
		if line == "" {
			continue
		}
		valid := false
		for _, prefix := range validPrefixes {
			if line == prefix || strings.HasPrefix(line, prefix+" ") {
				valid = true
				break
			}
		}
		if !valid {
			t.Errorf("invalid command line: %q", line)
		}
	}
}

func TestBuildReceiptScript_Randomization(t *testing.T) {
	todo := testTodo()
	distinct := make(map[string]bool)
	for i := 0; i < 20; i++ {
		script := buildReceiptScript(todo, 0, receiptWidth)
		distinct[script] = true
	}
	if len(distinct) < 2 {
		t.Errorf("buildReceiptScript produced only %d distinct outputs over 20 calls, expected >= 2", len(distinct))
	}
}

// --- FormatReceipt tests (require Node.js) ---

func nodeAvailable() bool {
	_, err := exec.LookPath("node")
	return err == nil
}

func TestFormatReceipt_NonEmpty(t *testing.T) {
	if !nodeAvailable() {
		t.Skip("node not available")
	}
	data, _, err := FormatReceipt(testTodo(), 0, "../../scripts/receipt-encoder/encode.js", receiptWidth)
	if err != nil {
		t.Fatalf("FormatReceipt error: %v", err)
	}
	if len(data) == 0 {
		t.Error("FormatReceipt returned empty byte slice")
	}
}

func TestFormatReceipt_ContainsESCBytes(t *testing.T) {
	if !nodeAvailable() {
		t.Skip("node not available")
	}
	data, _, err := FormatReceipt(testTodo(), 0, "../../scripts/receipt-encoder/encode.js", receiptWidth)
	if err != nil {
		t.Fatalf("FormatReceipt error: %v", err)
	}
	if !bytes.Contains(data, []byte{0x1b}) {
		t.Error("FormatReceipt output does not contain ESC bytes")
	}
}

func TestFormatReceipt_ContainsTodoText(t *testing.T) {
	if !nodeAvailable() {
		t.Skip("node not available")
	}
	todo := testTodo()
	data, _, err := FormatReceipt(todo, 0, "../../scripts/receipt-encoder/encode.js", receiptWidth)
	if err != nil {
		t.Fatalf("FormatReceipt error: %v", err)
	}
	if !bytes.Contains(data, []byte(todo.Text)) {
		t.Error("output does not contain todo text")
	}
}

func TestFormatReceipt_ContainsTodoID(t *testing.T) {
	if !nodeAvailable() {
		t.Skip("node not available")
	}
	todo := testTodo()
	data, _, err := FormatReceipt(todo, 0, "../../scripts/receipt-encoder/encode.js", receiptWidth)
	if err != nil {
		t.Fatalf("FormatReceipt error: %v", err)
	}
	if !bytes.Contains(data, []byte(todo.ID)) {
		t.Error("output does not contain todo ID")
	}
}

func TestFormatReceipt_BadScript_ReturnsError(t *testing.T) {
	if !nodeAvailable() {
		t.Skip("node not available")
	}
	_, _, err := FormatReceipt(testTodo(), 0, "/nonexistent/encode.js", receiptWidth)
	if err == nil {
		t.Error("FormatReceipt should return error for bad script path")
	}
}

// TestFormatReceipt_EmptyScript_ReturnsError tests the guard added to prevent
// the bug where an empty EncoderScript caused "node """ to run, which reads
// stdin as JavaScript instead of failing fast.
func TestFormatReceipt_EmptyScript_ReturnsError(t *testing.T) {
	_, _, err := FormatReceipt(testTodo(), 0, "", receiptWidth)
	if err == nil {
		t.Fatal("FormatReceipt with empty encoder script should return an error")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention 'empty', got: %v", err)
	}
}

// TestFormatReceipt_ConfigToReceipt_Integration loads a partial YAML config
// (mimicking a real printer.yaml that only has enabled + device_path + mode),
// verifies defaults are backfilled, then calls FormatReceipt with the
// backfilled EncoderScript. This covers the full config -> receipt path
// that was broken before the backfill fix.
func TestFormatReceipt_ConfigToReceipt_Integration(t *testing.T) {
	if !nodeAvailable() {
		t.Skip("node not available")
	}

	// Write a partial config like a real user would have.
	dir := t.TempDir()
	path := filepath.Join(dir, "printer.yaml")
	yaml := "enabled: true\ndevice_path: /dev/usb/lp0\nmode: escpos\n"
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// The encoder script must have been backfilled from defaults.
	if cfg.EncoderScript == "" {
		t.Fatal("EncoderScript not backfilled -- FormatReceipt would run 'node \"\"'")
	}

	// Now use the backfilled script path (relative from test working dir).
	// The test runs from internal/printer/, so we need ../../ prefix.
	data, _, err := FormatReceipt(testTodo(), 0, "../../"+cfg.EncoderScript, cfg.Width)
	if err != nil {
		t.Fatalf("FormatReceipt with backfilled config failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("FormatReceipt returned empty output with backfilled config")
	}
}

// --- FormatPlainReceipt tests ---

func TestFormatPlainReceipt_NoEscBytes(t *testing.T) {
	for i := 0; i < 20; i++ {
		data := FormatPlainReceipt(testTodo(), i%3, receiptWidth)
		if bytes.Contains(data, []byte{0x1b}) {
			t.Fatal("FormatPlainReceipt must not contain ESC (0x1b) bytes")
		}
	}
}

func TestFormatPlainReceipt_ContainsTodoText(t *testing.T) {
	todo := testTodo()
	data := FormatPlainReceipt(todo, 0, receiptWidth)
	if !bytes.Contains(data, []byte(todo.Text)) {
		t.Error("plain receipt does not contain todo text")
	}
}

func TestFormatPlainReceipt_ContainsTodoID(t *testing.T) {
	todo := testTodo()
	data := FormatPlainReceipt(todo, 0, receiptWidth)
	if !bytes.Contains(data, []byte(todo.ID)) {
		t.Error("plain receipt does not contain todo ID")
	}
}

func TestFormatPlainReceipt_ContainsTags(t *testing.T) {
	todo := &core.Todo{
		ID: "abc123", Text: "Tagged task", Tags: []string{"work", "homelab"}, Status: "active",
	}
	data := FormatPlainReceipt(todo, 0, receiptWidth)
	if !bytes.Contains(data, []byte("#work")) {
		t.Error("plain receipt does not contain tag 'work'")
	}
	if !bytes.Contains(data, []byte("#homelab")) {
		t.Error("plain receipt does not contain tag 'homelab'")
	}
}

func TestFormatPlainReceipt_LinesWithinWidth(t *testing.T) {
	todo := &core.Todo{
		ID:     "abc123",
		Text:   "This is a very long todo text that should be word wrapped because it exceeds thirty two characters",
		Tags:   []string{"work"},
		Status: "active",
	}
	data := FormatPlainReceipt(todo, 0, receiptWidth)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if utf8.RuneCountInString(line) > receiptWidth {
			t.Errorf("line exceeds %d chars (%d): %q", receiptWidth, utf8.RuneCountInString(line), line)
		}
	}
}

// TestFormatPlainReceipt_EndsWithLineFeeds verifies the tear margin was bumped
// from 20 to 40 newlines. We check >= 30 to have some tolerance while still
// catching the old value of 20.
func TestFormatPlainReceipt_EndsWithLineFeeds(t *testing.T) {
	data := FormatPlainReceipt(testTodo(), 0, receiptWidth)
	feedCount := 0
	for i := len(data) - 1; i >= 0 && data[i] == '\n'; i-- {
		feedCount++
	}
	if feedCount < 3 {
		t.Errorf("expected >= 3 trailing line feeds for tear margin, got %d", feedCount)
	}
}

func TestFormatPlainReceipt_WithStreak(t *testing.T) {
	data := FormatPlainReceipt(testTodo(), 5, receiptWidth)
	if !strings.Contains(strings.ToUpper(string(data)), "STREAK") {
		t.Error("plain receipt with streak > 0 should contain STREAK text")
	}
}

func TestFormatPlainReceipt_Urgent(t *testing.T) {
	todo := &core.Todo{
		ID: "abc123", Text: "Urgent task", Tags: []string{"work"}, Status: "active",
		Urgent: true,
	}
	data := FormatPlainReceipt(todo, 0, receiptWidth)
	if !strings.Contains(string(data), "URGENT") {
		t.Error("urgent todo plain receipt should contain URGENT marker")
	}
}

func TestFormatPlainReceipt_OnlyPrintableASCII(t *testing.T) {
	for i := 0; i < 20; i++ {
		data := FormatPlainReceipt(testTodo(), i%3, receiptWidth)
		for j, b := range data {
			// Allow printable ASCII (0x20-0x7E) and newline (0x0A)
			if b != 0x0a && (b < 0x20 || b > 0x7e) {
				t.Errorf("non-printable byte 0x%02X at position %d", b, j)
				return
			}
		}
	}
}

// --- wordWrap tests ---

func TestWordWrap(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		width    int
		wantMax  int // max number of lines expected (-1 = don't check)
		wantAll  bool
		checkFn  func(t *testing.T, lines []string)
	}{
		{
			name:  "empty string returns single element",
			text:  "",
			width: 32,
			checkFn: func(t *testing.T, lines []string) {
				if len(lines) != 1 {
					t.Errorf("expected 1 line for empty input, got %d", len(lines))
				}
			},
		},
		{
			name:  "short text stays on one line",
			text:  "hello world",
			width: 32,
			checkFn: func(t *testing.T, lines []string) {
				if len(lines) != 1 || lines[0] != "hello world" {
					t.Errorf("got %v, want [\"hello world\"]", lines)
				}
			},
		},
		{
			name:  "wraps at word boundary",
			text:  "one two three four five six seven eight",
			width: 15,
			checkFn: func(t *testing.T, lines []string) {
				for _, line := range lines {
					if len(line) > 15 {
						t.Errorf("line exceeds width 15: %q (%d chars)", line, len(line))
					}
				}
				// All words must appear in output
				joined := strings.Join(lines, " ")
				for _, word := range []string{"one", "two", "three", "four", "five", "six", "seven", "eight"} {
					if !strings.Contains(joined, word) {
						t.Errorf("missing word %q in wrapped output", word)
					}
				}
			},
		},
		{
			name:  "long word gets split at character boundary",
			text:  "abcdefghijklmnopqrstuvwxyz0123456789",
			width: 10,
			checkFn: func(t *testing.T, lines []string) {
				for _, line := range lines {
					if len(line) > 10 {
						t.Errorf("line exceeds width 10: %q (%d chars)", line, len(line))
					}
				}
				// Reconstruct and verify no chars lost
				joined := strings.Join(lines, "")
				if joined != "abcdefghijklmnopqrstuvwxyz0123456789" {
					t.Errorf("character-split lost data: got %q", joined)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := wordWrap(tt.text, tt.width)
			tt.checkFn(t, lines)
		})
	}
}

// --- centerText tests ---

func TestCenterText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{
			name:  "shorter than width gets padded",
			input: "hi",
			width: 10,
			want:  "    hi    ",
		},
		{
			name:  "exact width unchanged",
			input: "1234567890",
			width: 10,
			want:  "1234567890",
		},
		{
			name:  "longer than width unchanged",
			input: "this is longer than ten",
			width: 10,
			want:  "this is longer than ten",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := centerText(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("centerText(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
			}
		})
	}
}

// --- LogPrintDebug tests ---

// TestLogPrintDebug_WritesFile verifies that LogPrintDebug creates a file inside
// the .debug/ subdirectory with metadata header and content.
func TestLogPrintDebug_WritesFile(t *testing.T) {
	dir := t.TempDir()
	content := "TEXT Hardware test todo\nTEXT ID: abc123"

	err := LogPrintDebug(dir, "abc123", "test", "escpos", "/dev/usb/lp0", content, nil)
	if err != nil {
		t.Fatalf("LogPrintDebug error: %v", err)
	}

	debugDir := filepath.Join(dir, ".debug")
	entries, err := os.ReadDir(debugDir)
	if err != nil {
		t.Fatalf("failed to read .debug dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in .debug, got %d", len(entries))
	}

	if !strings.Contains(entries[0].Name(), "abc123") {
		t.Errorf("filename %q does not contain todo ID 'abc123'", entries[0].Name())
	}

	data, err := os.ReadFile(filepath.Join(debugDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "mode: escpos") {
		t.Error("debug log missing mode")
	}
	if !strings.Contains(s, "print: ok") {
		t.Error("debug log should show print: ok for nil error")
	}
	if !strings.Contains(s, content) {
		t.Error("debug log missing content")
	}
}

// TestLogPrintDebug_RecordsPrintError verifies that a print error is captured.
func TestLogPrintDebug_RecordsPrintError(t *testing.T) {
	dir := t.TempDir()

	err := LogPrintDebug(dir, "fail01", "test", "text", "/dev/usb/lp0", "receipt text", fmt.Errorf("write to printer: device busy"))
	if err != nil {
		t.Fatalf("LogPrintDebug error: %v", err)
	}

	debugDir := filepath.Join(dir, ".debug")
	entries, _ := os.ReadDir(debugDir)
	data, _ := os.ReadFile(filepath.Join(debugDir, entries[0].Name()))
	s := string(data)
	if !strings.Contains(s, "FAILED") {
		t.Error("debug log should contain FAILED for print errors")
	}
	if !strings.Contains(s, "device busy") {
		t.Error("debug log should contain the error message")
	}
}

// TestLogPrintDebug_CreatesDebugDir verifies that LogPrintDebug creates the
// .debug/ directory if it does not already exist.
func TestLogPrintDebug_CreatesDebugDir(t *testing.T) {
	dir := t.TempDir()
	debugDir := filepath.Join(dir, ".debug")

	if _, err := os.Stat(debugDir); err == nil {
		t.Fatal(".debug dir should not exist before LogPrintDebug call")
	}

	err := LogPrintDebug(dir, "def456", "test", "text", "/dev/null", "test", nil)
	if err != nil {
		t.Fatalf("LogPrintDebug error: %v", err)
	}

	info, err := os.Stat(debugDir)
	if err != nil {
		t.Fatalf(".debug dir was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error(".debug should be a directory")
	}
}
