package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// id.go
// ---------------------------------------------------------------------------

func TestGenerateID_Format(t *testing.T) {
	id, err := GenerateID()
	if err != nil {
		t.Fatalf("GenerateID() error: %v", err)
	}
	if len(id) != 8 {
		t.Errorf("expected 8-char ID, got %q (len %d)", id, len(id))
	}
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex character %q in ID %q", string(c), id)
		}
	}
}

func TestGenerateID_Unique(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		id, err := GenerateID()
		if err != nil {
			t.Fatalf("GenerateID() error on iteration %d: %v", i, err)
		}
		if seen[id] {
			t.Fatalf("duplicate ID %q at iteration %d", id, i)
		}
		seen[id] = true
	}
}

func TestIsUniqueID(t *testing.T) {
	existing := map[string]bool{"abc123": true, "def456": true}

	if !IsUniqueID("newone", existing) {
		t.Error("expected true for new ID")
	}
	if IsUniqueID("abc123", existing) {
		t.Error("expected false for existing ID")
	}
}

// ---------------------------------------------------------------------------
// todo.go
// ---------------------------------------------------------------------------

func TestParseTodoLine_Complete(t *testing.T) {
	line := `- [ ] Buy groceries {id:a1b2c3} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:yes} {important:no} {stale_count:1} #health #errands`

	todo, err := ParseTodoLine(line)
	if err != nil {
		t.Fatalf("ParseTodoLine error: %v", err)
	}

	if todo.Text != "Buy groceries" {
		t.Errorf("Text = %q, want %q", todo.Text, "Buy groceries")
	}
	if todo.ID != "a1b2c3" {
		t.Errorf("ID = %q, want %q", todo.ID, "a1b2c3")
	}
	if todo.Source != "cli" {
		t.Errorf("Source = %q, want %q", todo.Source, "cli")
	}
	if todo.Status != "inbox" {
		t.Errorf("Status = %q, want %q", todo.Status, "inbox")
	}
	if todo.Created.Format("2006-01-02") != "2026-03-10" {
		t.Errorf("Created = %v, want 2026-03-10", todo.Created)
	}
	if !todo.Urgent {
		t.Error("Urgent = false, want true")
	}
	if todo.Important {
		t.Error("Important = true, want false")
	}
	if todo.StaleCount != 1 {
		t.Errorf("StaleCount = %d, want 1", todo.StaleCount)
	}
	if todo.Done {
		t.Error("Done = true, want false")
	}
	if len(todo.Tags) != 2 || todo.Tags[0] != "health" || todo.Tags[1] != "errands" {
		t.Errorf("Tags = %v, want [health errands]", todo.Tags)
	}
}

func TestParseTodoLine_Done(t *testing.T) {
	line := `- [x] Finish report {id:ff0011} {source:cli} {status:done} {created:2026-03-01T10:00:00} {urgent:no} {important:yes} {stale_count:0}`

	todo, err := ParseTodoLine(line)
	if err != nil {
		t.Fatalf("ParseTodoLine error: %v", err)
	}
	if !todo.Done {
		t.Error("Done = false, want true")
	}
	if todo.Status != "done" {
		t.Errorf("Status = %q, want %q", todo.Status, "done")
	}
	if !todo.Important {
		t.Error("Important = false, want true")
	}
}

func TestParseTodoLine_InvalidPrefix(t *testing.T) {
	invalids := []string{
		"Buy groceries",
		"* [ ] not a dash",
		"",
	}
	for _, line := range invalids {
		_, err := ParseTodoLine(line)
		if err == nil {
			t.Errorf("expected error for line %q, got nil", line)
		}
	}
}

func TestToMarkdown_RoundTrip(t *testing.T) {
	created := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	todo := &Todo{
		ID:         "abcdef",
		Text:       "Test round-trip",
		Source:     "cli",
		Status:     "inbox",
		Created:    created,
		Urgent:     false,
		Important:  true,
		StaleCount: 0,
		Tags:       []string{"work", "urgent"},
		Notes:      []string{"some note here"},
		Subtasks:   []string{"first subtask"},
		Done:       false,
	}

	md := todo.ToMarkdown()
	// Parse it back (only the first line for ParseTodoLine; use ParseTodoBlock for full)
	lines := strings.Split(md, "\n")
	parsed, err := ParseTodoBlock(lines)
	if err != nil {
		t.Fatalf("ParseTodoBlock error: %v", err)
	}

	if parsed.ID != todo.ID {
		t.Errorf("ID = %q, want %q", parsed.ID, todo.ID)
	}
	if parsed.Text != todo.Text {
		t.Errorf("Text = %q, want %q", parsed.Text, todo.Text)
	}
	if parsed.Source != todo.Source {
		t.Errorf("Source = %q, want %q", parsed.Source, todo.Source)
	}
	if parsed.Status != todo.Status {
		t.Errorf("Status = %q, want %q", parsed.Status, todo.Status)
	}
	if !parsed.Created.Equal(todo.Created) {
		t.Errorf("Created = %v, want %v", parsed.Created, todo.Created)
	}
	if parsed.Urgent != todo.Urgent {
		t.Errorf("Urgent = %v, want %v", parsed.Urgent, todo.Urgent)
	}
	if parsed.Important != todo.Important {
		t.Errorf("Important = %v, want %v", parsed.Important, todo.Important)
	}
	if parsed.StaleCount != todo.StaleCount {
		t.Errorf("StaleCount = %d, want %d", parsed.StaleCount, todo.StaleCount)
	}
	if parsed.Done != todo.Done {
		t.Errorf("Done = %v, want %v", parsed.Done, todo.Done)
	}
	if len(parsed.Tags) != len(todo.Tags) {
		t.Errorf("Tags = %v, want %v", parsed.Tags, todo.Tags)
	}
	if len(parsed.Notes) != 1 || parsed.Notes[0] != "some note here" {
		t.Errorf("Notes = %v, want [some note here]", parsed.Notes)
	}
	if len(parsed.Subtasks) != 1 || parsed.Subtasks[0] != "first subtask" {
		t.Errorf("Subtasks = %v, want [first subtask]", parsed.Subtasks)
	}
}

func TestParseTodoBlock_NotesAndSubtasks(t *testing.T) {
	lines := []string{
		`- [ ] Plan trip {id:aaa111} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0}`,
		`  > Remember to book flights`,
		`  > Check hotel prices`,
		`  - [ ] Buy tickets`,
		`  - [ ] Pack bags`,
	}

	todo, err := ParseTodoBlock(lines)
	if err != nil {
		t.Fatalf("ParseTodoBlock error: %v", err)
	}

	if len(todo.Notes) != 2 {
		t.Fatalf("Notes count = %d, want 2", len(todo.Notes))
	}
	if todo.Notes[0] != "Remember to book flights" {
		t.Errorf("Notes[0] = %q", todo.Notes[0])
	}
	if todo.Notes[1] != "Check hotel prices" {
		t.Errorf("Notes[1] = %q", todo.Notes[1])
	}

	if len(todo.Subtasks) != 2 {
		t.Fatalf("Subtasks count = %d, want 2", len(todo.Subtasks))
	}
	if todo.Subtasks[0] != "Buy tickets" {
		t.Errorf("Subtasks[0] = %q", todo.Subtasks[0])
	}
	if todo.Subtasks[1] != "Pack bags" {
		t.Errorf("Subtasks[1] = %q", todo.Subtasks[1])
	}
}

func TestParseDayFile_Full(t *testing.T) {
	content := `# 2026-03-10

- [ ] First task {id:aaa111} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0} #work

- [x] Second task {id:bbb222} {source:cli} {status:done} {created:2026-03-10T10:00:00} {urgent:no} {important:no} {stale_count:0}
`

	todos, err := ParseDayFile(content)
	if err != nil {
		t.Fatalf("ParseDayFile error: %v", err)
	}

	if len(todos) != 2 {
		t.Fatalf("got %d todos, want 2", len(todos))
	}
	if todos[0].ID != "aaa111" {
		t.Errorf("todos[0].ID = %q, want aaa111", todos[0].ID)
	}
	if todos[1].Done != true {
		t.Error("todos[1].Done = false, want true")
	}
}

func TestParseDayFile_Empty(t *testing.T) {
	content := "# 2026-03-10\n"
	todos, err := ParseDayFile(content)
	if err != nil {
		t.Fatalf("ParseDayFile error: %v", err)
	}
	if len(todos) != 0 {
		t.Errorf("got %d todos, want 0", len(todos))
	}
}

func TestSerializeDayFile(t *testing.T) {
	created := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	todos := []*Todo{
		{
			ID: "abc123", Text: "Test item", Source: "cli", Status: "inbox",
			Created: created, Tags: []string{"work"},
		},
	}

	out := SerializeDayFile("2026-03-10", todos)
	if !strings.HasPrefix(out, "# 2026-03-10\n") {
		t.Errorf("missing header; got:\n%s", out)
	}
	if !strings.Contains(out, "- [ ] Test item") {
		t.Error("missing todo line in serialized output")
	}
}

// ---------------------------------------------------------------------------
// store.go
// ---------------------------------------------------------------------------

func newTestTodo(id, text, status string, created time.Time) *Todo {
	return &Todo{
		ID: id, Text: text, Source: "cli", Status: status,
		Created: created,
	}
}

func TestStore_AddAndReadDay(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	created := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	todo := newTestTodo("aaa111", "Buy milk", "inbox", created)

	if err := store.AddTodo(todo); err != nil {
		t.Fatalf("AddTodo error: %v", err)
	}

	todos, err := store.ReadDay("2026-03-10")
	if err != nil {
		t.Fatalf("ReadDay error: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("got %d todos, want 1", len(todos))
	}
	if todos[0].ID != "aaa111" {
		t.Errorf("ID = %q, want aaa111", todos[0].ID)
	}
	if todos[0].Text != "Buy milk" {
		t.Errorf("Text = %q, want Buy milk", todos[0].Text)
	}
}

func TestStore_FindTodoByID_AcrossDays(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	t1 := newTestTodo("aaa111", "Task A", "inbox", time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC))
	t2 := newTestTodo("bbb222", "Task B", "inbox", time.Date(2026, 3, 11, 9, 0, 0, 0, time.UTC))

	if err := store.AddTodo(t1); err != nil {
		t.Fatal(err)
	}
	if err := store.AddTodo(t2); err != nil {
		t.Fatal(err)
	}

	found, date, err := store.FindTodoByID("bbb222")
	if err != nil {
		t.Fatalf("FindTodoByID error: %v", err)
	}
	if found.Text != "Task B" {
		t.Errorf("Text = %q, want Task B", found.Text)
	}
	if date != "2026-03-11" {
		t.Errorf("date = %q, want 2026-03-11", date)
	}
}

func TestStore_FindTodoByID_Missing(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_, _, err := store.FindTodoByID("nonexistent")
	if err == nil {
		t.Error("expected error for missing ID, got nil")
	}
}

func TestStore_UpdateTodo(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	created := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	todo := newTestTodo("aaa111", "Original text", "inbox", created)

	if err := store.AddTodo(todo); err != nil {
		t.Fatal(err)
	}

	todo.Text = "Updated text"
	todo.Status = "today"
	if err := store.UpdateTodo(todo); err != nil {
		t.Fatalf("UpdateTodo error: %v", err)
	}

	found, _, err := store.FindTodoByID("aaa111")
	if err != nil {
		t.Fatal(err)
	}
	if found.Text != "Updated text" {
		t.Errorf("Text = %q, want Updated text", found.Text)
	}
	if found.Status != "today" {
		t.Errorf("Status = %q, want today", found.Status)
	}
}

func TestStore_DeleteTodo(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	created := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

	t1 := newTestTodo("aaa111", "Keep this", "inbox", created)
	t2 := newTestTodo("bbb222", "Delete this", "inbox", created)

	if err := store.AddTodo(t1); err != nil {
		t.Fatal(err)
	}
	if err := store.AddTodo(t2); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteTodo("bbb222"); err != nil {
		t.Fatalf("DeleteTodo error: %v", err)
	}

	todos, err := store.ReadDay("2026-03-10")
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 {
		t.Fatalf("got %d todos, want 1", len(todos))
	}
	if todos[0].ID != "aaa111" {
		t.Errorf("remaining todo ID = %q, want aaa111", todos[0].ID)
	}
}

func TestStore_EnsureDateFile(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	if err := store.EnsureDateFile("2026-03-15"); err != nil {
		t.Fatalf("EnsureDateFile error: %v", err)
	}

	path := filepath.Join(dir, "2026-03-15.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if !strings.HasPrefix(string(data), "# 2026-03-15") {
		t.Errorf("file content = %q, want header", string(data))
	}

	// Calling again should not error (idempotent).
	if err := store.EnsureDateFile("2026-03-15"); err != nil {
		t.Fatalf("second EnsureDateFile error: %v", err)
	}
}

func TestStore_BackfillGaps(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Create files for 2026-03-10 and 2026-03-13 (gap of 11, 12).
	if err := store.EnsureDateFile("2026-03-10"); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureDateFile("2026-03-13"); err != nil {
		t.Fatal(err)
	}

	if err := store.BackfillGaps(); err != nil {
		t.Fatalf("BackfillGaps error: %v", err)
	}

	// Check that 2026-03-11 and 2026-03-12 now exist.
	for _, date := range []string{"2026-03-11", "2026-03-12"} {
		path := filepath.Join(dir, date+".md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after backfill", date)
		}
	}
}

func TestStore_CollectAllIDs(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	t1 := newTestTodo("aaa111", "A", "inbox", time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC))
	t2 := newTestTodo("bbb222", "B", "inbox", time.Date(2026, 3, 11, 9, 0, 0, 0, time.UTC))

	if err := store.AddTodo(t1); err != nil {
		t.Fatal(err)
	}
	if err := store.AddTodo(t2); err != nil {
		t.Fatal(err)
	}

	ids, err := store.CollectAllIDs()
	if err != nil {
		t.Fatalf("CollectAllIDs error: %v", err)
	}

	if !ids["aaa111"] || !ids["bbb222"] {
		t.Errorf("ids = %v, expected both aaa111 and bbb222", ids)
	}
	if len(ids) != 2 {
		t.Errorf("len(ids) = %d, want 2", len(ids))
	}
}

// ---------------------------------------------------------------------------
// tags.go
// ---------------------------------------------------------------------------

func writeTestTags(t *testing.T, dir string) string {
	t.Helper()
	content := `categories:
  - work
  - personal
  - errands
energy:
  - health
  - fitness
`
	path := filepath.Join(dir, "tags.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewTagStore(t *testing.T) {
	dir := t.TempDir()
	path := writeTestTags(t, dir)

	ts, err := NewTagStore(path)
	if err != nil {
		t.Fatalf("NewTagStore error: %v", err)
	}

	if len(ts.Tags.Categories) != 3 {
		t.Errorf("Categories count = %d, want 3", len(ts.Tags.Categories))
	}
	if len(ts.Tags.Energy) != 2 {
		t.Errorf("Energy count = %d, want 2", len(ts.Tags.Energy))
	}
}

func TestAllTags(t *testing.T) {
	dir := t.TempDir()
	path := writeTestTags(t, dir)
	ts, _ := NewTagStore(path)

	all := ts.AllTags()
	if len(all) != 5 {
		t.Errorf("AllTags() len = %d, want 5", len(all))
	}
}

func TestIsValid(t *testing.T) {
	dir := t.TempDir()
	path := writeTestTags(t, dir)
	ts, _ := NewTagStore(path)

	tests := []struct {
		tag  string
		want bool
	}{
		{"work", true},
		{"Work", true},   // case-insensitive
		{"HEALTH", true}, // case-insensitive
		{"nonexistent", false},
	}

	for _, tt := range tests {
		got := ts.IsValid(tt.tag)
		if got != tt.want {
			t.Errorf("IsValid(%q) = %v, want %v", tt.tag, got, tt.want)
		}
	}
}

func TestFuzzyMatch(t *testing.T) {
	dir := t.TempDir()
	path := writeTestTags(t, dir)
	ts, _ := NewTagStore(path)

	// Close typo: "heatlh" -> "health" (distance 2, transposition)
	match := ts.FuzzyMatch("heatlh")
	if match != "health" {
		t.Errorf("FuzzyMatch(heatlh) = %q, want health", match)
	}

	// Distant string should return empty
	match = ts.FuzzyMatch("zzzzzzzzz")
	if match != "" {
		t.Errorf("FuzzyMatch(zzzzzzzzz) = %q, want empty", match)
	}
}

func TestAddTag(t *testing.T) {
	dir := t.TempDir()
	path := writeTestTags(t, dir)
	ts, _ := NewTagStore(path)

	if err := ts.AddTag("Finance", "categories"); err != nil {
		t.Fatalf("AddTag error: %v", err)
	}

	if !ts.IsValid("finance") {
		t.Error("finance should be valid after AddTag")
	}

	// Reload from disk to verify persistence.
	ts2, err := NewTagStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if !ts2.IsValid("finance") {
		t.Error("finance should be valid after reload")
	}
}

// ---------------------------------------------------------------------------
// logger.go
// ---------------------------------------------------------------------------

func TestLogCreated(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)
	todo := newTestTodo("aaa111", "Log test", "inbox", time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC))
	todo.Tags = []string{"work"}

	if err := logger.LogCreated("2026-03-10", todo); err != nil {
		t.Fatalf("LogCreated error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "2026-03-10.log"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "aaa111") {
		t.Error("log missing todo ID")
	}
	if !strings.Contains(content, "created") {
		t.Error("log missing 'created' keyword")
	}
	if !strings.Contains(content, "source=cli") {
		t.Error("log missing source")
	}
	if !strings.Contains(content, "tags=work") {
		t.Error("log missing tags")
	}
}

func TestLogStatusChange(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)

	if err := logger.LogStatusChange("2026-03-10", "aaa111", "inbox", "today"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "2026-03-10.log"))
	content := string(data)
	if !strings.Contains(content, "status") {
		t.Error("log missing 'status' keyword")
	}
	if !strings.Contains(content, "inbox->today") {
		t.Error("log missing status transition")
	}
}

func TestLogEdit(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)

	if err := logger.LogEdit("2026-03-10", "aaa111", "text", "old value", "new value"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "2026-03-10.log"))
	content := string(data)
	if !strings.Contains(content, "edited") {
		t.Error("log missing 'edited' keyword")
	}
	if !strings.Contains(content, "field=text") {
		t.Error("log missing field name")
	}
}

func TestLogDelete(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)

	if err := logger.LogDelete("2026-03-10", "aaa111"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "2026-03-10.log"))
	content := string(data)
	if !strings.Contains(content, "aaa111") {
		t.Error("log missing todo ID")
	}
	if !strings.Contains(content, "deleted") {
		t.Error("log missing 'deleted' keyword")
	}
}

func TestLogger_MultipleAppend(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)

	logger.LogDelete("2026-03-10", "aaa111")
	logger.LogDelete("2026-03-10", "bbb222")
	logger.LogStatusChange("2026-03-10", "ccc333", "inbox", "done")

	data, _ := os.ReadFile(filepath.Join(dir, "2026-03-10.log"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 log lines, got %d", len(lines))
	}
}

// ---------------------------------------------------------------------------
// stale.go
// ---------------------------------------------------------------------------

func TestFindStaleItems(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	oldDate := time.Now().AddDate(0, 0, -10)
	recentDate := time.Now().AddDate(0, 0, -1)

	// Old inbox item -> should be stale
	staleItem := newTestTodo("aaa111", "Old inbox", "inbox", oldDate)
	// Recent inbox item -> not stale
	recentItem := newTestTodo("bbb222", "Recent inbox", "inbox", recentDate)
	// Old non-inbox item -> should NOT be returned
	todayItem := newTestTodo("ccc333", "Old today", "today", oldDate)

	for _, todo := range []*Todo{staleItem, recentItem, todayItem} {
		if err := store.AddTodo(todo); err != nil {
			t.Fatal(err)
		}
	}

	stales, err := FindStaleItems(store)
	if err != nil {
		t.Fatalf("FindStaleItems error: %v", err)
	}

	if len(stales) != 1 {
		t.Fatalf("got %d stale items, want 1", len(stales))
	}
	if stales[0].ID != "aaa111" {
		t.Errorf("stale ID = %q, want aaa111", stales[0].ID)
	}
}

func TestMarkStale(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	logger := NewLogger(dir)

	created := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	todo := newTestTodo("aaa111", "Mark me stale", "inbox", created)
	if err := store.AddTodo(todo); err != nil {
		t.Fatal(err)
	}

	if err := MarkStale(store, logger, todo); err != nil {
		t.Fatalf("MarkStale error: %v", err)
	}

	found, _, err := store.FindTodoByID("aaa111")
	if err != nil {
		t.Fatal(err)
	}
	if found.Status != "stale" {
		t.Errorf("Status = %q, want stale", found.Status)
	}
}

func TestReviveTodo(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	logger := NewLogger(dir)

	created := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	todo := newTestTodo("aaa111", "Revive me", "stale", created)
	todo.StaleCount = 1
	if err := store.AddTodo(todo); err != nil {
		t.Fatal(err)
	}

	if err := ReviveTodo(store, logger, todo); err != nil {
		t.Fatalf("ReviveTodo error: %v", err)
	}

	found, _, err := store.FindTodoByID("aaa111")
	if err != nil {
		t.Fatal(err)
	}
	if found.Status != "inbox" {
		t.Errorf("Status = %q, want inbox", found.Status)
	}
	if found.StaleCount != 2 {
		t.Errorf("StaleCount = %d, want 2", found.StaleCount)
	}
}

func TestFindLoopingItems(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	created := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

	looper := newTestTodo("aaa111", "Looping item", "inbox", created)
	looper.StaleCount = 2

	normal := newTestTodo("bbb222", "Normal item", "inbox", created)
	normal.StaleCount = 0

	for _, todo := range []*Todo{looper, normal} {
		if err := store.AddTodo(todo); err != nil {
			t.Fatal(err)
		}
	}

	looping, err := FindLoopingItems(store)
	if err != nil {
		t.Fatal(err)
	}
	if len(looping) != 1 {
		t.Fatalf("got %d looping, want 1", len(looping))
	}
	if looping[0].ID != "aaa111" {
		t.Errorf("looping ID = %q, want aaa111", looping[0].ID)
	}
}

func TestRunStaleCheck(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	logger := NewLogger(dir)

	oldDate := time.Now().AddDate(0, 0, -10)
	old1 := newTestTodo("aaa111", "Stale 1", "inbox", oldDate)
	old2 := newTestTodo("bbb222", "Stale 2", "inbox", oldDate)
	recent := newTestTodo("ccc333", "Not stale", "inbox", time.Now())

	for _, todo := range []*Todo{old1, old2, recent} {
		if err := store.AddTodo(todo); err != nil {
			t.Fatal(err)
		}
	}

	count, err := RunStaleCheck(store, logger)
	if err != nil {
		t.Fatalf("RunStaleCheck error: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	// Verify both are now stale.
	found1, _, _ := store.FindTodoByID("aaa111")
	found2, _, _ := store.FindTodoByID("bbb222")
	if found1.Status != "stale" || found2.Status != "stale" {
		t.Error("not all items marked stale")
	}
}

// ---------------------------------------------------------------------------
// info.go
// ---------------------------------------------------------------------------

func TestGetInfoLine(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	created := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

	unproc := newTestTodo("aaa111", "Unprocessed", "unprocessed", created)
	inbox := newTestTodo("bbb222", "Inbox", "inbox", created)
	looper := newTestTodo("ccc333", "Looper", "inbox", created)
	looper.StaleCount = 3

	for _, todo := range []*Todo{unproc, inbox, looper} {
		if err := store.AddTodo(todo); err != nil {
			t.Fatal(err)
		}
	}

	info, err := GetInfoLine(store)
	if err != nil {
		t.Fatalf("GetInfoLine error: %v", err)
	}
	if info.Unprocessed != 1 {
		t.Errorf("Unprocessed = %d, want 1", info.Unprocessed)
	}
	if info.Looping != 1 {
		t.Errorf("Looping = %d, want 1", info.Looping)
	}
}

func TestFormatInfoLine(t *testing.T) {
	tests := []struct {
		name string
		info InfoLine
		want string
	}{
		{
			name: "both zero",
			info: InfoLine{Unprocessed: 0, Looping: 0},
			want: "",
		},
		{
			name: "has counts",
			info: InfoLine{Unprocessed: 3, Looping: 1},
			want: "── 📬 Unprocessed: 3 │ 🔁 Looping: 1 ──",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatInfoLine(tt.info)
			if got != tt.want {
				t.Errorf("FormatInfoLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NEW TESTS: Additional coverage for real user behavior
// ---------------------------------------------------------------------------

// ParseTodoLine with multiple tags must preserve their order.
func TestParseTodoLine_MultipleTagsPreservesOrder(t *testing.T) {
	line := `- [ ] Deploy to prod {id:abc123} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0} #homelab #work #health`
	todo, err := ParseTodoLine(line)
	if err != nil {
		t.Fatalf("ParseTodoLine error: %v", err)
	}
	if len(todo.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(todo.Tags), todo.Tags)
	}
	if todo.Tags[0] != "homelab" || todo.Tags[1] != "work" || todo.Tags[2] != "health" {
		t.Errorf("tags not in expected order: got %v, want [homelab work health]", todo.Tags)
	}
}

// ParseTodoLine with no tags returns an empty (or nil) slice.
func TestParseTodoLine_NoTags(t *testing.T) {
	line := `- [ ] Simple task {id:aaa111} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0}`
	todo, err := ParseTodoLine(line)
	if err != nil {
		t.Fatalf("ParseTodoLine error: %v", err)
	}
	if len(todo.Tags) != 0 {
		t.Errorf("expected 0 tags, got %d: %v", len(todo.Tags), todo.Tags)
	}
}

// Full round-trip: SerializeDayFile -> ParseDayFile preserves all fields for all todos.
// NOTE: Subtasks are excluded from this test because ParseDayFile has a bug where
// indented subtask lines (  - [ ] ...) are incorrectly parsed as new top-level todos.
// See TestParseDayFile_SubtasksBug for the failing test that documents this.
func TestSerializeDayFile_ParseDayFile_RoundTrip(t *testing.T) {
	created1 := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	created2 := time.Date(2026, 3, 10, 14, 30, 0, 0, time.UTC)

	original := []*Todo{
		{
			ID: "aaa111", Text: "First task with special chars: foo & bar",
			Source: "cli", Status: "inbox", Created: created1,
			Urgent: true, Important: false, StaleCount: 0,
			Tags: []string{"homelab", "work"}, Notes: []string{"note one", "note two"},
			Subtasks: nil, Done: false,
		},
		{
			ID: "bbb222", Text: "Done task",
			Source: "api", Status: "done", Created: created2,
			Urgent: false, Important: true, StaleCount: 3,
			Tags: []string{"health"}, Notes: nil, Subtasks: nil, Done: true,
		},
	}

	serialized := SerializeDayFile("2026-03-10", original)
	parsed, err := ParseDayFile(serialized)
	if err != nil {
		t.Fatalf("ParseDayFile error: %v", err)
	}

	if len(parsed) != len(original) {
		t.Fatalf("expected %d todos, got %d", len(original), len(parsed))
	}

	for i, orig := range original {
		got := parsed[i]
		if got.ID != orig.ID {
			t.Errorf("todo[%d] ID: got %q, want %q", i, got.ID, orig.ID)
		}
		if got.Text != orig.Text {
			t.Errorf("todo[%d] Text: got %q, want %q", i, got.Text, orig.Text)
		}
		if got.Source != orig.Source {
			t.Errorf("todo[%d] Source: got %q, want %q", i, got.Source, orig.Source)
		}
		if got.Status != orig.Status {
			t.Errorf("todo[%d] Status: got %q, want %q", i, got.Status, orig.Status)
		}
		if !got.Created.Equal(orig.Created) {
			t.Errorf("todo[%d] Created: got %v, want %v", i, got.Created, orig.Created)
		}
		if got.Urgent != orig.Urgent {
			t.Errorf("todo[%d] Urgent: got %v, want %v", i, got.Urgent, orig.Urgent)
		}
		if got.Important != orig.Important {
			t.Errorf("todo[%d] Important: got %v, want %v", i, got.Important, orig.Important)
		}
		if got.StaleCount != orig.StaleCount {
			t.Errorf("todo[%d] StaleCount: got %d, want %d", i, got.StaleCount, orig.StaleCount)
		}
		if got.Done != orig.Done {
			t.Errorf("todo[%d] Done: got %v, want %v", i, got.Done, orig.Done)
		}
		if len(got.Tags) != len(orig.Tags) {
			t.Errorf("todo[%d] Tags: got %v, want %v", i, got.Tags, orig.Tags)
		} else {
			for j := range orig.Tags {
				if got.Tags[j] != orig.Tags[j] {
					t.Errorf("todo[%d] Tags[%d]: got %q, want %q", i, j, got.Tags[j], orig.Tags[j])
				}
			}
		}
		if len(got.Notes) != len(orig.Notes) {
			t.Errorf("todo[%d] Notes: got %v, want %v", i, got.Notes, orig.Notes)
		} else {
			for j := range orig.Notes {
				if got.Notes[j] != orig.Notes[j] {
					t.Errorf("todo[%d] Notes[%d]: got %q, want %q", i, j, got.Notes[j], orig.Notes[j])
				}
			}
		}
		if len(got.Subtasks) != len(orig.Subtasks) {
			t.Errorf("todo[%d] Subtasks: got %v, want %v", i, got.Subtasks, orig.Subtasks)
		} else {
			for j := range orig.Subtasks {
				if got.Subtasks[j] != orig.Subtasks[j] {
					t.Errorf("todo[%d] Subtasks[%d]: got %q, want %q", i, j, got.Subtasks[j], orig.Subtasks[j])
				}
			}
		}
	}
}

// BUG: ParseDayFile incorrectly treats indented subtask lines as new top-level todos.
// The line "  - [ ] subtask" gets TrimSpace'd to "- [ ] subtask", which matches the
// "- [" prefix check and starts a new block instead of being appended to the parent.
// This test documents the bug: a todo with subtasks should round-trip through
// SerializeDayFile -> ParseDayFile as a single todo, not multiple.
func TestParseDayFile_SubtasksBug(t *testing.T) {
	created := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	original := []*Todo{
		{
			ID: "aaa111", Text: "Parent task",
			Source: "cli", Status: "inbox", Created: created,
			Tags: []string{"work"}, Subtasks: []string{"step one", "step two"},
		},
	}

	serialized := SerializeDayFile("2026-03-10", original)
	parsed, err := ParseDayFile(serialized)
	if err != nil {
		t.Fatalf("ParseDayFile error: %v", err)
	}

	// BUG: This currently returns 3 (parent + 2 subtasks parsed as separate todos).
	// Expected behavior: 1 todo with 2 subtasks.
	if len(parsed) != 1 {
		t.Errorf("BUG: expected 1 todo (parent with subtasks), got %d — subtasks are incorrectly parsed as separate todos", len(parsed))
	}
}

// Store.FindTodoByID after UpdateTodo returns the updated version.
func TestStore_FindTodoByID_AfterUpdate(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	created := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	todo := newTestTodo("aaa111", "Original", "inbox", created)
	todo.Tags = []string{"work"}

	if err := store.AddTodo(todo); err != nil {
		t.Fatal(err)
	}

	// Mutate and update
	todo.Text = "Changed"
	todo.Status = "today"
	todo.Tags = []string{"homelab", "work"}
	todo.Urgent = true
	if err := store.UpdateTodo(todo); err != nil {
		t.Fatal(err)
	}

	found, _, err := store.FindTodoByID("aaa111")
	if err != nil {
		t.Fatal(err)
	}
	if found.Text != "Changed" {
		t.Errorf("Text = %q, want Changed", found.Text)
	}
	if found.Status != "today" {
		t.Errorf("Status = %q, want today", found.Status)
	}
	if !found.Urgent {
		t.Error("Urgent = false, want true")
	}
	if len(found.Tags) != 2 || found.Tags[0] != "homelab" || found.Tags[1] != "work" {
		t.Errorf("Tags = %v, want [homelab work]", found.Tags)
	}
}

// Store.DeleteTodo then FindTodoByID returns error.
func TestStore_DeleteTodo_ThenFindReturnsError(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	created := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	todo := newTestTodo("aaa111", "Delete me", "inbox", created)

	if err := store.AddTodo(todo); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteTodo("aaa111"); err != nil {
		t.Fatal(err)
	}

	_, _, err := store.FindTodoByID("aaa111")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

// BackfillGaps with no existing files just creates today.
func TestStore_BackfillGaps_NoExistingFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	if err := store.BackfillGaps(); err != nil {
		t.Fatalf("BackfillGaps error: %v", err)
	}

	today := time.Now().Format("2006-01-02")
	path := filepath.Join(dir, today+".md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected today's file %s to exist after BackfillGaps with no prior files", today)
	}
}

// DeleteTodo on a non-existent ID returns an error.
func TestStore_DeleteTodo_NonExistent(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	err := store.DeleteTodo("nonexistent")
	if err == nil {
		t.Error("expected error deleting non-existent todo, got nil")
	}
}

// Verify that writing a day file and reading it back via the store
// produces identical results (file-level round-trip through actual disk).
func TestStore_AddTodo_ReadDay_FileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	created := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)

	// NOTE: Subtasks are excluded due to ParseDayFile bug (see TestParseDayFile_SubtasksBug).
	todo := &Todo{
		ID: "abc123", Text: "Round trip test",
		Source: "api", Status: "today", Created: created,
		Urgent: true, Important: true, StaleCount: 5,
		Tags:     []string{"homelab", "minecraft"},
		Notes:    []string{"important note"},
		Subtasks: nil,
		Done:     false,
	}

	if err := store.AddTodo(todo); err != nil {
		t.Fatalf("AddTodo error: %v", err)
	}

	todos, err := store.ReadDay("2026-03-15")
	if err != nil {
		t.Fatalf("ReadDay error: %v", err)
	}

	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	got := todos[0]
	if got.ID != "abc123" {
		t.Errorf("ID = %q, want abc123", got.ID)
	}
	if got.Text != "Round trip test" {
		t.Errorf("Text = %q", got.Text)
	}
	if got.Source != "api" {
		t.Errorf("Source = %q", got.Source)
	}
	if got.Status != "today" {
		t.Errorf("Status = %q", got.Status)
	}
	if !got.Urgent || !got.Important {
		t.Errorf("Urgent=%v Important=%v", got.Urgent, got.Important)
	}
	if got.StaleCount != 5 {
		t.Errorf("StaleCount = %d", got.StaleCount)
	}
	if len(got.Tags) != 2 {
		t.Errorf("Tags = %v", got.Tags)
	}
	if len(got.Notes) != 1 || got.Notes[0] != "important note" {
		t.Errorf("Notes = %v", got.Notes)
	}
}

// ===========================================================================
// NEW TESTS: Breaking tests for edge cases, bugs, and scaling
// ===========================================================================

// --- CRITICAL BUG: Braces in text causes DATA LOSS ---

// Text containing {curly} braces in the MIDDLE of text survives because the
// metadata parser only strips trailing {} blocks. This test documents that
// the middle-of-text case is safe.
func TestParseTodoLine_BracesInMiddleOfText_Survives(t *testing.T) {
	line := `- [ ] deploy {release} v1.2 {id:abc123} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0}`
	todo, err := ParseTodoLine(line)
	if err != nil {
		t.Fatalf("ParseTodoLine error: %v", err)
	}
	// Middle braces survive because after stripping real metadata from the end,
	// the remaining text "deploy {release} v1.2" doesn't end with }
	if todo.Text != "deploy {release} v1.2" {
		t.Errorf("Text with braces in middle corrupted: got %q, want %q", todo.Text, "deploy {release} v1.2")
	}
}

// BUG: Text ending with {something} is eaten as metadata even when it's user text.
func TestParseTodoLine_TextEndingWithBraces_DataLoss(t *testing.T) {
	line := `- [ ] check the {config} {id:abc123} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0}`
	todo, err := ParseTodoLine(line)
	if err != nil {
		t.Fatalf("ParseTodoLine error: %v", err)
	}
	// BUG: {config} is consumed as metadata (no colon, so it's skipped but rest is truncated)
	if todo.Text != "check the {config}" {
		t.Errorf("BUG: Text ending with braces corrupted: got %q, want %q", todo.Text, "check the {config}")
	}
}

// Round-trip of text with braces in the middle survives (braces not at end of text).
func TestRoundTrip_BracesInMiddleOfText_Survives(t *testing.T) {
	created := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	todo := &Todo{
		ID: "abcdef", Text: "deploy {release} v1.2",
		Source: "cli", Status: "inbox", Created: created,
		Tags: []string{"work"},
	}

	md := todo.ToMarkdown()
	lines := strings.Split(md, "\n")
	parsed, err := ParseTodoBlock(lines)
	if err != nil {
		t.Fatalf("ParseTodoBlock error: %v", err)
	}

	// Middle braces survive round-trip
	if parsed.Text != todo.Text {
		t.Errorf("Round-trip text with middle braces: got %q, want %q", parsed.Text, todo.Text)
	}
}

// BUG: Round-trip of text with braces at the END loses data.
func TestRoundTrip_BracesAtEndOfText_DataLoss(t *testing.T) {
	created := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	todo := &Todo{
		ID: "abcdef", Text: "check the {config}",
		Source: "cli", Status: "inbox", Created: created,
		Tags: []string{"work"},
	}

	md := todo.ToMarkdown()
	lines := strings.Split(md, "\n")
	parsed, err := ParseTodoBlock(lines)
	if err != nil {
		t.Fatalf("ParseTodoBlock error: %v", err)
	}

	// BUG: {config} at end of text is stripped during parse
	if parsed.Text != todo.Text {
		t.Errorf("BUG: Round-trip data loss — trailing braces stripped: got %q, want %q", parsed.Text, todo.Text)
	}
}

// BUG: When braces appear at the END of text (right before real metadata),
// the parser strips them one by one from right to left.
// "use {foo} and {bar}" — after stripping real metadata, rest ends with },
// so {bar} is consumed. Then rest ends with }, so {foo} is consumed too.
func TestParseTodoLine_MultipleBracePairs_TrailingStripped(t *testing.T) {
	line := `- [ ] use {foo} and {bar} {id:abc123} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0}`
	todo, err := ParseTodoLine(line)
	if err != nil {
		t.Fatalf("ParseTodoLine error: %v", err)
	}
	// BUG: Both {foo} and {bar} are stripped because they're both trailing
	if todo.Text != "use {foo} and {bar}" {
		t.Errorf("BUG: trailing braces stripped from text: got %q, want %q", todo.Text, "use {foo} and {bar}")
	}
}

// When trailing braces contain a colon, they're silently consumed as metadata
// with incorrect key:value, potentially overwriting real metadata.
func TestParseTodoLine_BracesWithColon_OverwritesMetadata(t *testing.T) {
	// If user text is "check {status:broken}" — the parser will interpret this
	// as real status metadata and overwrite the actual status
	line := `- [ ] check {status:broken} {id:abc123} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0}`
	todo, err := ParseTodoLine(line)
	if err != nil {
		t.Fatalf("ParseTodoLine error: %v", err)
	}
	// BUG: The user's {status:broken} is parsed as metadata, potentially
	// overwriting the real status value. The parser processes right-to-left,
	// so the real {status:inbox} is processed first, then {status:broken}
	// overwrites it.
	if todo.Status == "broken" {
		t.Errorf("BUG: User text {status:broken} overwrote real status metadata — got status %q", todo.Status)
	}
}

// --- Parsing edge cases ---

// Text containing #hash mid-word like issue#42 should NOT be parsed as tag.
func TestParseTodoLine_HashMidWord_NotParsedAsTag(t *testing.T) {
	line := `- [ ] fix issue#42 in repo {id:abc123} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0} #work`
	todo, err := ParseTodoLine(line)
	if err != nil {
		t.Fatalf("ParseTodoLine error: %v", err)
	}
	// The parser uses LastIndex(" #") so "issue#42" should not match
	// because there's no space before the #
	if len(todo.Tags) != 1 || todo.Tags[0] != "work" {
		t.Errorf("Tags = %v, want [work] — issue#42 should not be parsed as tag", todo.Tags)
	}
	if !strings.Contains(todo.Text, "issue#42") {
		t.Errorf("Text should contain 'issue#42', got %q", todo.Text)
	}
}

// Metadata with colons in value like {source:cli:extra} — what happens?
func TestParseTodoLine_MetadataWithColonInValue(t *testing.T) {
	line := `- [ ] test item {id:abc123} {source:cli:extra} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0}`
	todo, err := ParseTodoLine(line)
	if err != nil {
		t.Fatalf("ParseTodoLine error: %v", err)
	}
	// NOTE: strings.Index(meta, ":") finds the FIRST colon, so key="source", value="cli:extra"
	// This means source gets set to "cli:extra" which is surprising but consistent.
	if todo.Source != "cli:extra" {
		t.Errorf("NOTE: Source with colon in value: got %q, want %q", todo.Source, "cli:extra")
	}
}

// ParseTodoLine with empty text (only metadata) should work but have empty text.
func TestParseTodoLine_OnlyMetadata_EmptyText(t *testing.T) {
	line := `- [ ] {id:abc123} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0}`
	todo, err := ParseTodoLine(line)
	if err != nil {
		t.Fatalf("ParseTodoLine error: %v", err)
	}
	if todo.Text != "" {
		t.Errorf("expected empty text, got %q", todo.Text)
	}
	if todo.ID != "abc123" {
		t.Errorf("ID = %q, want abc123", todo.ID)
	}
}

// ParseDayFile with 3 subtasks should produce 1 todo, not 4.
func TestParseDayFile_ThreeSubtasks_SingleTodo(t *testing.T) {
	content := `# 2026-03-10

- [ ] Parent task {id:aaa111} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0} #work
  - [ ] Subtask one
  - [ ] Subtask two
  - [ ] Subtask three
`
	todos, err := ParseDayFile(content)
	if err != nil {
		t.Fatalf("ParseDayFile error: %v", err)
	}
	// BUG: Returns 4 todos (parent + 3 subtasks parsed as separate todos)
	// Expected: 1 todo with 3 subtasks
	if len(todos) != 1 {
		t.Errorf("BUG: expected 1 todo with 3 subtasks, got %d separate todos — subtask lines are parsed as top-level todos", len(todos))
	}
}

// ParseDayFile with notes AND subtasks should all attach to parent.
func TestParseDayFile_NotesAndSubtasks_AttachToParent(t *testing.T) {
	content := `# 2026-03-10

- [ ] Complex task {id:aaa111} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0}
  > Important note
  > Another note
  - [ ] First step
  - [ ] Second step
`
	todos, err := ParseDayFile(content)
	if err != nil {
		t.Fatalf("ParseDayFile error: %v", err)
	}
	// BUG: subtask lines are parsed as separate todos
	if len(todos) != 1 {
		t.Errorf("BUG: expected 1 todo with notes+subtasks, got %d todos", len(todos))
		return
	}
	if len(todos[0].Notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(todos[0].Notes))
	}
	if len(todos[0].Subtasks) != 2 {
		t.Errorf("expected 2 subtasks, got %d", len(todos[0].Subtasks))
	}
}

// ParseDayFile with malformed lines mixed in should skip gracefully.
func TestParseDayFile_MalformedLines_SkipsGracefully(t *testing.T) {
	content := `# 2026-03-10

some random text that isn't a todo
another garbage line
- [ ] Valid task {id:aaa111} {source:cli} {status:inbox} {created:2026-03-10T09:00:00} {urgent:no} {important:no} {stale_count:0}

more garbage
`
	todos, err := ParseDayFile(content)
	if err != nil {
		t.Fatalf("ParseDayFile should not error on malformed lines: %v", err)
	}
	if len(todos) != 1 {
		t.Errorf("expected 1 valid todo (skipping garbage lines), got %d", len(todos))
	}
	if len(todos) > 0 && todos[0].ID != "aaa111" {
		t.Errorf("wrong todo parsed, got ID %q", todos[0].ID)
	}
}

// SerializeDayFile -> ParseDayFile round-trip with notes, multiple todos.
func TestSerializeParseDayFile_RoundTrip_MultipleWithNotes(t *testing.T) {
	created := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

	original := []*Todo{
		{
			ID: "aaa111", Text: "First task", Source: "cli", Status: "inbox",
			Created: created, Tags: []string{"work"}, Notes: []string{"note 1", "note 2"},
		},
		{
			ID: "bbb222", Text: "Second task", Source: "api", Status: "done",
			Created: created, Urgent: true, Important: true, StaleCount: 2,
			Tags: []string{"homelab", "health"}, Done: true,
		},
		{
			ID: "ccc333", Text: "Third task no tags", Source: "cli", Status: "inbox",
			Created: created,
		},
	}

	serialized := SerializeDayFile("2026-03-10", original)
	parsed, err := ParseDayFile(serialized)
	if err != nil {
		t.Fatalf("ParseDayFile error: %v", err)
	}

	if len(parsed) != 3 {
		t.Fatalf("expected 3 todos, got %d", len(parsed))
	}

	// Verify second todo's flags
	if !parsed[1].Urgent || !parsed[1].Important {
		t.Errorf("todo[1] Urgent=%v Important=%v, want true/true", parsed[1].Urgent, parsed[1].Important)
	}
	if parsed[1].StaleCount != 2 {
		t.Errorf("todo[1] StaleCount=%d, want 2", parsed[1].StaleCount)
	}
	if !parsed[1].Done {
		t.Errorf("todo[1] Done=%v, want true", parsed[1].Done)
	}

	// Verify notes on first todo
	if len(parsed[0].Notes) != 2 {
		t.Errorf("todo[0] Notes count=%d, want 2", len(parsed[0].Notes))
	}
}

// --- Tag edge cases ---

func TestFuzzyMatch_ExactMatch(t *testing.T) {
	dir := t.TempDir()
	path := writeTestTags(t, dir)
	ts, _ := NewTagStore(path)

	match := ts.FuzzyMatch("work")
	if match != "work" {
		t.Errorf("FuzzyMatch(work) = %q, want exact match 'work'", match)
	}
}

func TestFuzzyMatch_EmptyString(t *testing.T) {
	dir := t.TempDir()
	path := writeTestTags(t, dir)
	ts, _ := NewTagStore(path)

	match := ts.FuzzyMatch("")
	// NOTE: Empty string has distance equal to the length of each tag.
	// Since all tags are len >= 4, distance > 3, so should return "".
	if match != "" {
		t.Errorf("FuzzyMatch('') = %q, want empty string", match)
	}
}

func TestIsValid_EmptyString(t *testing.T) {
	dir := t.TempDir()
	path := writeTestTags(t, dir)
	ts, _ := NewTagStore(path)

	if ts.IsValid("") {
		t.Error("IsValid('') should return false")
	}
}

func TestAddTag_Duplicate(t *testing.T) {
	dir := t.TempDir()
	path := writeTestTags(t, dir)
	ts, _ := NewTagStore(path)

	// Add "work" which already exists
	before := len(ts.AllTags())
	if err := ts.AddTag("work", "categories"); err != nil {
		t.Fatalf("AddTag error: %v", err)
	}
	after := len(ts.AllTags())

	// NOTE: AddTag does NOT deduplicate — it blindly appends.
	// This is a design decision that should probably be a bug.
	if after != before+1 {
		t.Errorf("NOTE: AddTag with duplicate: before=%d, after=%d — AddTag does not deduplicate", before, after)
	}
}

func TestNewTagStore_NonExistentFile(t *testing.T) {
	_, err := NewTagStore("/nonexistent/path/tags.yaml")
	if err == nil {
		t.Error("expected error for non-existent tags.yaml, got nil")
	}
}

func TestNewTagStore_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tags.yaml")
	if err := os.WriteFile(path, []byte("{{invalid yaml[[["), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := NewTagStore(path)
	if err == nil {
		t.Error("expected error for malformed YAML, got nil")
	}
}

func TestNewTagStore_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tags.yaml")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	ts, err := NewTagStore(path)
	// NOTE: Empty YAML file unmarshals to zero-value struct — no error.
	if err != nil {
		t.Fatalf("NewTagStore with empty file: %v", err)
	}
	if len(ts.AllTags()) != 0 {
		t.Errorf("expected 0 tags from empty file, got %d", len(ts.AllTags()))
	}
}

// --- Store edge cases ---

func TestStore_ReadDay_NonExistentDate(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	todos, err := store.ReadDay("2099-12-31")
	if err != nil {
		t.Fatalf("ReadDay on non-existent date should not error: %v", err)
	}
	if len(todos) != 0 {
		t.Errorf("expected empty slice, got %d todos", len(todos))
	}
}

func TestStore_WriteDay_NilTodos_CreatesHeaderOnly(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	if err := store.WriteDay("2026-03-20", nil); err != nil {
		t.Fatalf("WriteDay nil error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "2026-03-20.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "# 2026-03-20") {
		t.Errorf("expected header, got %q", content)
	}
	// Should be just the header line, no todo lines
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line (header only), got %d lines", len(lines))
	}
}

func TestStore_FindTodoByID_EmptyString(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	created := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	todo := newTestTodo("aaa111", "test", "inbox", created)
	store.AddTodo(todo)

	_, _, err := store.FindTodoByID("")
	if err == nil {
		t.Error("expected error for empty ID, got nil")
	}
}

func TestStore_UpdateTodo_NonExistentID(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	todo := newTestTodo("nonexistent", "text", "inbox", time.Now())
	err := store.UpdateTodo(todo)
	if err == nil {
		t.Error("expected error updating non-existent todo, got nil")
	}
}

func TestStore_CollectAllIDs_NoFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	ids, err := store.CollectAllIDs()
	if err != nil {
		t.Fatalf("CollectAllIDs error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty map, got %d entries", len(ids))
	}
}

func TestStore_ReadDay_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	// Write a 0-byte file
	if err := os.WriteFile(filepath.Join(dir, "2026-03-10.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(dir)

	todos, err := store.ReadDay("2026-03-10")
	if err != nil {
		t.Fatalf("ReadDay on empty file should not error: %v", err)
	}
	if len(todos) != 0 {
		t.Errorf("expected 0 todos from empty file, got %d", len(todos))
	}
}

func TestStore_ReadDay_HeaderOnlyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "2026-03-10.md"), []byte("# 2026-03-10\n"), 0644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(dir)

	todos, err := store.ReadDay("2026-03-10")
	if err != nil {
		t.Fatalf("ReadDay on header-only file should not error: %v", err)
	}
	if len(todos) != 0 {
		t.Errorf("expected 0 todos, got %d", len(todos))
	}
}

func TestStore_ReadDay_GarbageContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "2026-03-10.md"), []byte("totally garbage content\nno headers\nno todos\n"), 0644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(dir)

	todos, err := store.ReadDay("2026-03-10")
	// Should handle gracefully — garbage lines are skipped in ParseDayFile
	if err != nil {
		t.Fatalf("ReadDay on garbage content should not panic/error: %v", err)
	}
	if len(todos) != 0 {
		t.Errorf("expected 0 todos from garbage, got %d", len(todos))
	}
}

func TestStore_AddTodo_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent", "subdir")
	store := NewStore(dir)

	todo := newTestTodo("aaa111", "test", "inbox", time.Now())
	if err := store.AddTodo(todo); err != nil {
		t.Fatalf("AddTodo should create dir: %v", err)
	}

	// Verify it was saved
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected data dir to be created")
	}
}

func TestStore_BackfillGaps_DirDoesNotExist(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	store := NewStore(dir)

	if err := store.BackfillGaps(); err != nil {
		t.Fatalf("BackfillGaps should create dir: %v", err)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected data dir to be created by BackfillGaps")
	}
}

// --- Stale edge cases ---

func TestFindStaleItems_ExactlySevenDaysAgo(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Created exactly 7 days ago — should this be stale?
	// The cutoff uses time.Now().AddDate(0,0,-7) and checks Created.Before(cutoff)
	// So 7 days ago at the same time: Before() returns false (equal), so NOT stale.
	exactlySevenDays := time.Now().AddDate(0, 0, -7)
	todo := newTestTodo("aaa111", "boundary", "inbox", exactlySevenDays)
	store.AddTodo(todo)

	stales, err := FindStaleItems(store)
	if err != nil {
		t.Fatal(err)
	}

	// NOTE: Exactly 7 days ago is NOT stale (Before is strict inequality).
	// This test documents that boundary behavior.
	if len(stales) != 0 {
		t.Errorf("NOTE: Todo created exactly 7 days ago should NOT be stale (strict Before), got %d stale items", len(stales))
	}
}

func TestFindStaleItems_SixDays23Hours(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Created 6 days 23 hours ago — should NOT be stale
	almostSeven := time.Now().Add(-time.Duration(6*24+23) * time.Hour)
	todo := newTestTodo("aaa111", "almost stale", "inbox", almostSeven)
	store.AddTodo(todo)

	stales, err := FindStaleItems(store)
	if err != nil {
		t.Fatal(err)
	}

	if len(stales) != 0 {
		t.Errorf("Todo created 6d23h ago should not be stale, got %d stale items", len(stales))
	}
}

func TestReviveTodo_RepeatedRevives(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	logger := NewLogger(dir)

	created := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	todo := newTestTodo("aaa111", "looper", "stale", created)
	todo.StaleCount = 0
	store.AddTodo(todo)

	// Revive 3 times: 0->1->2->3
	for i := 0; i < 3; i++ {
		found, _, err := store.FindTodoByID("aaa111")
		if err != nil {
			t.Fatal(err)
		}
		found.Status = "stale" // simulate going stale again
		store.UpdateTodo(found)

		found2, _, _ := store.FindTodoByID("aaa111")
		if err := ReviveTodo(store, logger, found2); err != nil {
			t.Fatalf("ReviveTodo iteration %d: %v", i, err)
		}
	}

	final, _, err := store.FindTodoByID("aaa111")
	if err != nil {
		t.Fatal(err)
	}
	if final.StaleCount != 3 {
		t.Errorf("After 3 revives, StaleCount=%d, want 3", final.StaleCount)
	}
	if final.Status != "inbox" {
		t.Errorf("After revive, Status=%q, want inbox", final.Status)
	}
}

func TestMarkStale_AlreadyStaleTodo(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	logger := NewLogger(dir)

	created := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	todo := newTestTodo("aaa111", "already stale", "stale", created)
	store.AddTodo(todo)

	// NOTE: MarkStale on already-stale todo: sets status to "stale" (no-op on status)
	// but logs a status change "stale->stale". This is arguably a bug — should it be idempotent?
	err := MarkStale(store, logger, todo)
	if err != nil {
		t.Fatalf("MarkStale on already-stale todo errored: %v", err)
	}

	found, _, _ := store.FindTodoByID("aaa111")
	if found.Status != "stale" {
		t.Errorf("Status = %q, want stale", found.Status)
	}
}

// --- Logger edge cases ---

func TestLogger_AppendToExistingLog(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)

	// Write some initial content
	logger.LogDelete("2026-03-10", "aaa111")

	// Append more
	logger.LogStatusChange("2026-03-10", "bbb222", "inbox", "today")

	data, err := os.ReadFile(filepath.Join(dir, "2026-03-10.log"))
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "aaa111") {
		t.Error("first line should contain aaa111")
	}
	if !strings.Contains(lines[1], "bbb222") {
		t.Error("second line should contain bbb222")
	}
}

func TestLogger_SpecialCharsInText(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(dir)

	todo := &Todo{
		ID: "aaa111", Text: `text with "quotes" and {braces} and newline\n`,
		Source: "cli", Tags: []string{"work"},
	}

	if err := logger.LogCreated("2026-03-10", todo); err != nil {
		t.Fatalf("LogCreated with special chars: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "2026-03-10.log"))
	if err != nil {
		t.Fatal(err)
	}
	// Verify the log line was written (fmt.Sprintf %q handles quoting)
	content := string(data)
	if !strings.Contains(content, "aaa111") {
		t.Error("log should contain the todo ID")
	}
	if !strings.Contains(content, "created") {
		t.Error("log should contain 'created'")
	}
}

// --- Scaling Tests ---

func TestParseDayFile_100Todos(t *testing.T) {
	var b strings.Builder
	b.WriteString("# 2026-03-10\n")
	created := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 100; i++ {
		todo := &Todo{
			ID: fmt.Sprintf("%06x", i), Text: fmt.Sprintf("Task number %d", i),
			Source: "cli", Status: "inbox", Created: created,
			Tags: []string{"work"},
		}
		b.WriteString("\n")
		b.WriteString(todo.ToMarkdown())
		b.WriteString("\n")
	}

	todos, err := ParseDayFile(b.String())
	if err != nil {
		t.Fatalf("ParseDayFile with 100 todos: %v", err)
	}
	if len(todos) != 100 {
		t.Errorf("expected 100 todos, got %d", len(todos))
	}
}

func TestCollectAllIDs_ManyFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	created := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

	// Create 50 files with 20 todos each
	for d := 0; d < 50; d++ {
		date := created.AddDate(0, 0, d)
		dateStr := date.Format("2006-01-02")
		var todos []*Todo
		for i := 0; i < 20; i++ {
			todos = append(todos, &Todo{
				ID: fmt.Sprintf("d%02dt%02d", d, i), Text: fmt.Sprintf("Task %d-%d", d, i),
				Source: "cli", Status: "inbox", Created: date,
			})
		}
		store.WriteDay(dateStr, todos)
	}

	ids, err := store.CollectAllIDs()
	if err != nil {
		t.Fatalf("CollectAllIDs error: %v", err)
	}
	if len(ids) != 1000 {
		t.Errorf("expected 1000 IDs, got %d", len(ids))
	}
}

func TestReadAllDays_ManyFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	created := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

	for d := 0; d < 100; d++ {
		date := created.AddDate(0, 0, d)
		dateStr := date.Format("2006-01-02")
		store.WriteDay(dateStr, []*Todo{{
			ID: fmt.Sprintf("t%03d", d), Text: fmt.Sprintf("Task %d", d),
			Source: "cli", Status: "inbox", Created: date,
		}})
	}

	allDays, err := store.ReadAllDays()
	if err != nil {
		t.Fatalf("ReadAllDays error: %v", err)
	}
	if len(allDays) != 100 {
		t.Errorf("expected 100 days, got %d", len(allDays))
	}
}

func TestTodo_ManyTags_RoundTrip(t *testing.T) {
	created := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	tags := make([]string, 50)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag%d", i)
	}

	todo := &Todo{
		ID: "abc123", Text: "Many tags", Source: "cli", Status: "inbox",
		Created: created, Tags: tags,
	}

	md := todo.ToMarkdown()
	lines := strings.Split(md, "\n")
	parsed, err := ParseTodoBlock(lines)
	if err != nil {
		t.Fatalf("ParseTodoBlock error: %v", err)
	}

	if len(parsed.Tags) != 50 {
		t.Errorf("expected 50 tags after round-trip, got %d", len(parsed.Tags))
	}
}

func TestTodo_VeryLongText_RoundTrip(t *testing.T) {
	created := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	longText := strings.Repeat("a", 10000)

	todo := &Todo{
		ID: "abc123", Text: longText, Source: "cli", Status: "inbox",
		Created: created, Tags: []string{"work"},
	}

	md := todo.ToMarkdown()
	lines := strings.Split(md, "\n")
	parsed, err := ParseTodoBlock(lines)
	if err != nil {
		t.Fatalf("ParseTodoBlock error: %v", err)
	}

	if len(parsed.Text) != 10000 {
		t.Errorf("expected 10000 char text, got %d", len(parsed.Text))
	}
}

// NOTE: GenerateID uses 3 bytes = 16,777,216 possible values. With 10,000 IDs,
// the birthday paradox gives ~0.3% collision probability per run. This test
// uses 1000 IDs where collision probability is negligible (~0.003%).
// The system mitigates collisions via IsUniqueID retry loop, but the ID space
// is small enough that collisions WILL happen in production with ~5000+ todos.
func TestGenerateID_1000Unique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id, err := GenerateID()
		if err != nil {
			t.Fatalf("GenerateID error at iteration %d: %v", i, err)
		}
		if seen[id] {
			t.Fatalf("duplicate ID %q at iteration %d out of 1000", id, i)
		}
		seen[id] = true
	}
}

// --- IO failure tests ---

func TestStore_WriteDay_ReadOnlyDir(t *testing.T) {
	dir := t.TempDir()
	roDir := filepath.Join(dir, "readonly")
	os.MkdirAll(roDir, 0755)
	// Make it read-only
	os.Chmod(roDir, 0555)
	t.Cleanup(func() { os.Chmod(roDir, 0755) }) // restore for cleanup

	store := NewStore(roDir)
	todo := newTestTodo("aaa111", "test", "inbox", time.Now())

	err := store.WriteDay("2026-03-10", []*Todo{todo})
	if err == nil {
		t.Error("expected error writing to read-only directory, got nil")
	}
}
