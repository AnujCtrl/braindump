package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/anujp/braindump/internal/core"
)

const tagsYAML = `categories:
  - homelab
  - minecraft
  - work
  - health
  - errands
energy:
  - quick-win
  - deep-focus
  - braindump
`

func setupTestEnv(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "tags.yaml"), []byte(tagsYAML), 0644); err != nil {
		t.Fatalf("writing tags.yaml: %v", err)
	}
	os.Setenv("TODO_DATA_DIR", tmpDir)
	return tmpDir, func() { os.Unsetenv("TODO_DATA_DIR") }
}

// captureOutput captures stdout during function execution.
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// executeCmd runs a cobra command with the given args and captures stdout.
func executeCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var execErr error
	output := captureOutput(func() {
		RootCmd.SetArgs(args)
		execErr = RootCmd.Execute()
		RootCmd.SetArgs(nil)
	})
	return output, execErr
}

// extractID parses the captured todo ID from output like "Captured (a1b2c3)"
var idRegex = regexp.MustCompile(`Captured \(([a-f0-9]+)\)`)

func extractID(t *testing.T, output string) string {
	t.Helper()
	matches := idRegex.FindStringSubmatch(output)
	if len(matches) < 2 {
		t.Fatalf("could not extract ID from output: %q", output)
	}
	return matches[1]
}

// --- Capture Tests ---

func TestCaptureBasic(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "fix", "the", "server")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	if !strings.Contains(output, "Captured") {
		t.Fatalf("expected 'Captured' in output, got: %q", output)
	}

	id := extractID(t, output)

	// Verify the todo exists in today's file
	today := time.Now().Format("2006-01-02")
	todos, err := store.ReadDay(today)
	if err != nil {
		t.Fatalf("reading day: %v", err)
	}

	var found *core.Todo
	for _, todo := range todos {
		if todo.ID == id {
			found = todo
			break
		}
	}
	if found == nil {
		t.Fatal("captured todo not found in today's file")
	}
	if found.Text != "fix the server" {
		t.Errorf("expected text 'fix the server', got %q", found.Text)
	}
}

func TestCaptureWithTag(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "fix", "server", "#homelab")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	foundTag := false
	for _, tag := range todo.Tags {
		if tag == "homelab" {
			foundTag = true
		}
	}
	if !foundTag {
		t.Errorf("expected tag 'homelab', got tags: %v", todo.Tags)
	}
	if todo.Text != "fix server" {
		t.Errorf("expected text 'fix server', got %q", todo.Text)
	}
}

func TestCaptureNoTagsDefaultBraindump(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "random", "thought")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	foundBraindump := false
	for _, tag := range todo.Tags {
		if tag == "braindump" {
			foundBraindump = true
		}
	}
	if !foundBraindump {
		t.Errorf("expected default tag 'braindump', got tags: %v", todo.Tags)
	}
}

func TestCaptureWithDoubleDashSeparator(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "--", "dump", "drives")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	if todo.Text != "dump drives" {
		t.Errorf("expected text 'dump drives', got %q", todo.Text)
	}
}

func TestCaptureWithNoteFlag(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "call", "dentist", "#health", "--note", "555-1234")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	if todo.Text != "call dentist" {
		t.Errorf("expected text 'call dentist', got %q", todo.Text)
	}
	foundHealth := false
	for _, tag := range todo.Tags {
		if tag == "health" {
			foundHealth = true
		}
	}
	if !foundHealth {
		t.Errorf("expected tag 'health', got tags: %v", todo.Tags)
	}
	if len(todo.Notes) == 0 || todo.Notes[0] != "555-1234" {
		t.Errorf("expected note '555-1234', got notes: %v", todo.Notes)
	}
}

// --- List Tests ---

func TestListShowsTodaysTodos(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Capture a todo first
	capOut, err := executeCmd(t, "buy", "milk", "#errands")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	// List todos
	output, err := executeCmd(t, "ls")
	if err != nil {
		t.Fatalf("ls failed: %v", err)
	}

	if !strings.Contains(output, id) {
		t.Errorf("expected todo ID %s in ls output, got: %q", id, output)
	}
	if !strings.Contains(output, "buy milk") {
		t.Errorf("expected 'buy milk' in ls output, got: %q", output)
	}
}

func TestListFilterByTag(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Capture two todos with different tags
	_, err := executeCmd(t, "task", "one", "#homelab")
	if err != nil {
		t.Fatalf("capture 1 failed: %v", err)
	}
	capOut2, err := executeCmd(t, "task", "two", "#work")
	if err != nil {
		t.Fatalf("capture 2 failed: %v", err)
	}
	id2 := extractID(t, capOut2)

	// Filter by work tag
	output, err := executeCmd(t, "ls", "--tag", "work")
	if err != nil {
		t.Fatalf("ls --tag failed: %v", err)
	}

	if !strings.Contains(output, id2) {
		t.Errorf("expected todo with #work in output, got: %q", output)
	}
	if strings.Contains(output, "task one") {
		t.Errorf("did not expect 'task one' (homelab) in work-filtered output, got: %q", output)
	}
}

func TestListByDate(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Manually create a todo for a specific past date
	pastDate := "2026-03-15"
	pastTodo := &core.Todo{
		ID:      "aabbcc",
		Text:    "old task",
		Source:  "cli",
		Status:  "inbox",
		Created: time.Date(2026, 3, 15, 10, 0, 0, 0, time.Local),
		Tags:    []string{"work"},
	}
	pastContent := core.SerializeDayFile(pastDate, []*core.Todo{pastTodo})
	os.WriteFile(filepath.Join(tmpDir, pastDate+".md"), []byte(pastContent), 0644)

	output, err := executeCmd(t, "ls", "--date", pastDate)
	if err != nil {
		t.Fatalf("ls --date failed: %v", err)
	}

	if !strings.Contains(output, "aabbcc") {
		t.Errorf("expected ID 'aabbcc' in date-filtered output, got: %q", output)
	}
	if !strings.Contains(output, "old task") {
		t.Errorf("expected 'old task' in date-filtered output, got: %q", output)
	}
}

// --- Done Tests ---

func TestDoneMarksTodoAsDone(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "finish", "report", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	output, err := executeCmd(t, "done", id)
	if err != nil {
		t.Fatalf("done failed: %v", err)
	}

	if !strings.Contains(output, "Done:") {
		t.Errorf("expected 'Done:' in output, got: %q", output)
	}

	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}
	if todo.Status != "done" {
		t.Errorf("expected status 'done', got %q", todo.Status)
	}
	if !todo.Done {
		t.Error("expected Done=true")
	}
}

// --- Edit Tests ---

func TestEditUpdatesTextAndTags(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "original", "text", "#homelab")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	output, err := executeCmd(t, "edit", id, "new", "text", "#work")
	if err != nil {
		t.Fatalf("edit failed: %v", err)
	}

	if !strings.Contains(output, "edited") {
		t.Errorf("expected 'edited' in output, got: %q", output)
	}

	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}
	if todo.Text != "new text" {
		t.Errorf("expected text 'new text', got %q", todo.Text)
	}
	foundWork := false
	for _, tag := range todo.Tags {
		if tag == "work" {
			foundWork = true
		}
	}
	if !foundWork {
		t.Errorf("expected tag 'work', got tags: %v", todo.Tags)
	}
}

// --- Delete Tests ---

func TestDeleteRemovesTodo(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "remove", "me", "#errands")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	// Verify it exists
	_, _, err = store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("todo should exist before delete: %v", err)
	}

	output, err := executeCmd(t, "delete", id)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if !strings.Contains(output, "deleted") {
		t.Errorf("expected 'deleted' in output, got: %q", output)
	}

	// Verify it's gone
	_, _, err = store.FindTodoByID(id)
	if err == nil {
		t.Error("todo should not exist after delete")
	}
}

// --- Move Tests ---

func TestMoveChangesStatus(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "plan", "dinner", "#errands")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	output, err := executeCmd(t, "move", id, "today")
	if err != nil {
		t.Fatalf("move failed: %v", err)
	}

	if !strings.Contains(output, "moved") {
		t.Errorf("expected 'moved' in output, got: %q", output)
	}
	if !strings.Contains(output, "today") {
		t.Errorf("expected 'today' in output, got: %q", output)
	}

	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}
	if todo.Status != "today" {
		t.Errorf("expected status 'today', got %q", todo.Status)
	}
}

// --- Tag Tests ---

func TestTagListShowsTags(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "tag", "list")
	if err != nil {
		t.Fatalf("tag list failed: %v", err)
	}

	if !strings.Contains(output, "#homelab") {
		t.Errorf("expected '#homelab' in tag list output, got: %q", output)
	}
	if !strings.Contains(output, "#work") {
		t.Errorf("expected '#work' in tag list output, got: %q", output)
	}
	if !strings.Contains(output, "#braindump") {
		t.Errorf("expected '#braindump' in tag list output, got: %q", output)
	}
}

// --- Discovery Tests ---

func TestHashListsAllTags(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "#")
	if err != nil {
		t.Fatalf("# command failed: %v", err)
	}

	if !strings.Contains(output, "categories:") {
		t.Errorf("expected 'categories:' in output, got: %q", output)
	}
	if !strings.Contains(output, "#homelab") {
		t.Errorf("expected '#homelab' in output, got: %q", output)
	}
	if !strings.Contains(output, "energy:") {
		t.Errorf("expected 'energy:' in output, got: %q", output)
	}
}

func TestAtListsSources(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "@")
	if err != nil {
		t.Fatalf("@ command failed: %v", err)
	}

	if !strings.Contains(output, "cli") {
		t.Errorf("expected 'cli' in @ output, got: %q", output)
	}
	if !strings.Contains(output, "api") {
		t.Errorf("expected 'api' in @ output, got: %q", output)
	}
	if !strings.Contains(output, "minecraft") {
		t.Errorf("expected 'minecraft' in @ output, got: %q", output)
	}
}
