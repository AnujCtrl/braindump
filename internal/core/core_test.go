package core

import (
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
	if len(id) != 6 {
		t.Errorf("expected 6-char ID, got %q (len %d)", id, len(id))
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
