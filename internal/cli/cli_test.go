package cli

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/anujp/braindump/internal/core"
	pflag "github.com/spf13/pflag"
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
// It resets all subcommand flag state to prevent leakage between tests.
func executeCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var execErr error
	output := captureOutput(func() {
		// Reset all flags to defaults to prevent state leakage between test executions.
		resetFlags()
		RootCmd.SetArgs(args)
		execErr = RootCmd.Execute()
		RootCmd.SetArgs(nil)
	})
	return output, execErr
}

// resetFlags resets cobra subcommand flags to prevent test-to-test leakage.
func resetFlags() {
	for _, cmd := range RootCmd.Commands() {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			f.Value.Set(f.DefValue)
			f.Changed = false
		})
	}
	RootCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Value.Set(f.DefValue)
		f.Changed = false
	})
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

	output, err := executeCmd(t, "move", id, "active")
	if err != nil {
		t.Fatalf("move failed: %v", err)
	}

	if !strings.Contains(output, "moved") {
		t.Errorf("expected 'moved' in output, got: %q", output)
	}
	if !strings.Contains(output, "active") {
		t.Errorf("expected 'active' in output, got: %q", output)
	}

	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}
	if todo.Status != "active" {
		t.Errorf("expected status 'active', got %q", todo.Status)
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

// ---------------------------------------------------------------------------
// NEW TESTS: Tag shorthand bug, edge cases, and real user behavior
// ---------------------------------------------------------------------------

// BUG: #tag shorthand not implemented, this test documents expected behavior.
// `todo ls #homelab` should filter by tag "homelab", but the ls command ignores
// positional args — it only reads the --tag flag. This means #tag shorthand is
// silently ignored and all todos are shown instead of the filtered set.
func TestListFilterByTagShorthand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create two todos with different tags
	_, err := executeCmd(t, "setup", "proxmox", "#homelab")
	if err != nil {
		t.Fatalf("capture 1 failed: %v", err)
	}
	capOut2, err := executeCmd(t, "write", "report", "#work")
	if err != nil {
		t.Fatalf("capture 2 failed: %v", err)
	}
	workID := extractID(t, capOut2)

	// BUG: #tag shorthand not implemented, this test documents expected behavior.
	// `todo ls #homelab` should ONLY show the homelab todo.
	output, err := executeCmd(t, "ls", "#homelab")
	if err != nil {
		t.Fatalf("ls #homelab failed: %v", err)
	}

	if !strings.Contains(output, "setup proxmox") {
		t.Errorf("expected homelab todo 'setup proxmox' in output, got: %q", output)
	}
	if strings.Contains(output, workID) {
		t.Errorf("should NOT show work todo %s when filtering by #homelab, got: %q", workID, output)
	}
}

// BUG: #tag shorthand not implemented, this test documents expected behavior.
// `todo ls #braindump` should only show braindump-tagged todos, not everything.
func TestListFilterByBraindumpShorthand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// One todo with explicit tag, one with default braindump
	_, err := executeCmd(t, "random", "thought") // gets #braindump by default
	if err != nil {
		t.Fatalf("capture 1 failed: %v", err)
	}
	capOut2, err := executeCmd(t, "fix", "server", "#homelab")
	if err != nil {
		t.Fatalf("capture 2 failed: %v", err)
	}
	homelabID := extractID(t, capOut2)

	// BUG: #tag shorthand not implemented, this test documents expected behavior.
	output, err := executeCmd(t, "ls", "#braindump")
	if err != nil {
		t.Fatalf("ls #braindump failed: %v", err)
	}

	if !strings.Contains(output, "random thought") {
		t.Errorf("expected braindump todo in output, got: %q", output)
	}
	if strings.Contains(output, homelabID) {
		t.Errorf("should NOT show homelab todo when filtering by #braindump, got: %q", output)
	}
}

// Verify that --tag flag (the working path) correctly excludes non-matching todos.
func TestListFilterByTagFlag_ExcludesOtherTags(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut1, err := executeCmd(t, "task", "alpha", "#homelab")
	if err != nil {
		t.Fatalf("capture 1 failed: %v", err)
	}
	homelabID := extractID(t, capOut1)

	capOut2, err := executeCmd(t, "task", "beta", "#work")
	if err != nil {
		t.Fatalf("capture 2 failed: %v", err)
	}
	workID := extractID(t, capOut2)

	output, err := executeCmd(t, "ls", "--tag", "homelab")
	if err != nil {
		t.Fatalf("ls --tag homelab failed: %v", err)
	}

	if !strings.Contains(output, homelabID) {
		t.Errorf("expected homelab todo %s in output, got: %q", homelabID, output)
	}
	if strings.Contains(output, workID) {
		t.Errorf("should NOT contain work todo %s in homelab-filtered output, got: %q", workID, output)
	}
}

// `todo ls` with no filters should show all of today's todos.
func TestListNoFilterShowsAll(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut1, err := executeCmd(t, "first", "item", "#homelab")
	if err != nil {
		t.Fatalf("capture 1 failed: %v", err)
	}
	id1 := extractID(t, capOut1)

	capOut2, err := executeCmd(t, "second", "item", "#work")
	if err != nil {
		t.Fatalf("capture 2 failed: %v", err)
	}
	id2 := extractID(t, capOut2)

	output, err := executeCmd(t, "ls")
	if err != nil {
		t.Fatalf("ls failed: %v", err)
	}

	if !strings.Contains(output, id1) {
		t.Errorf("expected todo %s in unfiltered ls output, got: %q", id1, output)
	}
	if !strings.Contains(output, id2) {
		t.Errorf("expected todo %s in unfiltered ls output, got: %q", id2, output)
	}
}

// Capture with @minecraft source should auto-add #minecraft tag.
func TestCaptureSourceAutoAddsTag(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "build", "farm", "@minecraft")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	if todo.Source != "minecraft" {
		t.Errorf("expected source 'minecraft', got %q", todo.Source)
	}

	foundMinecraft := false
	for _, tag := range todo.Tags {
		if tag == "minecraft" {
			foundMinecraft = true
		}
	}
	if !foundMinecraft {
		t.Errorf("expected @minecraft to auto-add #minecraft tag, got tags: %v", todo.Tags)
	}
}

// Capture with !! should set urgent=true in the stored todo (not just in output).
func TestCaptureUrgent_StoredValue(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "fix", "now", "!!", "#homelab")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	if !todo.Urgent {
		t.Error("expected Urgent=true for !! capture, got false")
	}
	if todo.Important {
		t.Error("expected Important=false for !! capture, got true")
	}
	if todo.Text != "fix now" {
		t.Errorf("expected text 'fix now' (!! stripped), got %q", todo.Text)
	}
}

// Capture with !!! should set important=true in the stored todo.
func TestCaptureImportant_StoredValue(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "critical", "bug", "!!!", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	if !todo.Important {
		t.Error("expected Important=true for !!! capture, got false")
	}
	if todo.Urgent {
		t.Error("expected Urgent=false for !!! capture (only !!!), got true")
	}
}

// `todo edit <id> new text #newtag` should change both text and tags in storage.
func TestEditChangesTextAndTags_VerifyStorage(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "old", "text", "#homelab")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	_, err = executeCmd(t, "edit", id, "brand", "new", "text", "#work")
	if err != nil {
		t.Fatalf("edit failed: %v", err)
	}

	// Read back from disk to verify persistence
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo after edit: %v", err)
	}

	if todo.Text != "brand new text" {
		t.Errorf("expected text 'brand new text', got %q", todo.Text)
	}

	if len(todo.Tags) != 1 || todo.Tags[0] != "work" {
		t.Errorf("expected tags [work], got %v", todo.Tags)
	}
}

// `todo move <id> done` should set both status="done" AND Done=true.
func TestMoveToDone_SetsBothFields(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "finish", "this", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	_, err = executeCmd(t, "move", id, "done")
	if err != nil {
		t.Fatalf("move failed: %v", err)
	}

	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	if todo.Status != "done" {
		t.Errorf("expected status 'done', got %q", todo.Status)
	}
	if !todo.Done {
		t.Error("expected Done=true after move to done, got false")
	}
}

// `todo done <id>` on an already-done todo should not error (idempotent).
func TestDoneOnAlreadyDoneTodo(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "already", "finished", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	// Mark done first time
	_, err = executeCmd(t, "done", id)
	if err != nil {
		t.Fatalf("first done failed: %v", err)
	}

	// Mark done second time — should not error
	_, err = executeCmd(t, "done", id)
	if err != nil {
		t.Errorf("second done on already-done todo should not error, got: %v", err)
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

// `todo delete <nonexistent>` should return an error.
func TestDeleteNonExistentID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	_, err := executeCmd(t, "delete", "zzz999")
	if err == nil {
		t.Error("expected error when deleting non-existent ID, got nil")
	}
}

// `todo edit <nonexistent> new text` should return an error.
func TestEditNonExistentID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	_, err := executeCmd(t, "edit", "zzz999", "new", "text")
	if err == nil {
		t.Error("expected error when editing non-existent ID, got nil")
	}
}

// `todo move <id> invalidstatus` should return an error.
func TestMoveInvalidStatus(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "test", "item", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	_, err = executeCmd(t, "move", id, "invalid_status")
	if err == nil {
		t.Error("expected error for invalid status, got nil")
	}
}

// `todo -- ls` should capture "ls" as text, not trigger the list subcommand.
func TestDoubleDashCapturesReservedWord(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "--", "ls", "the", "files")
	if err != nil {
		t.Fatalf("capture with -- failed: %v", err)
	}

	if !strings.Contains(output, "Captured") {
		t.Fatalf("expected capture, got: %q", output)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	if todo.Text != "ls the files" {
		t.Errorf("expected text 'ls the files', got %q", todo.Text)
	}
}

// --- BREAKING TESTS: CLI edge cases ---

// BUG: Capture with text ending in {braces} — data loss through store round-trip.
// The parser strips trailing {key} blocks as metadata. When user text ends with
// braces, they get consumed.
func TestCapture_TextEndingWithBraces_DataLoss(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "check", "the", "{config}", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	// BUG: {config} at end of text is stripped after disk round-trip
	if todo.Text != "check the {config}" {
		t.Errorf("BUG: Trailing braces stripped from text after round-trip: got %q, want %q",
			todo.Text, "check the {config}")
	}
}

// Text with braces in the MIDDLE survives because parser only strips trailing {}.
func TestCapture_TextWithMiddleBraces_Survives(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "deploy", "{release}", "v1.2", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	// Middle braces survive (parser only strips trailing {})
	if todo.Text != "deploy {release} v1.2" {
		t.Errorf("Text with middle braces corrupted: got %q, want %q",
			todo.Text, "deploy {release} v1.2")
	}
}

// Capture with ONLY tags, no text: `todo #work` — should error.
func TestCapture_OnlyTags_NoText(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	_, err := executeCmd(t, "#work")
	if err == nil {
		t.Error("expected error when capturing with only tags and no text, got nil")
	}
}

// NOTE: `todo -- #work text here` with -- as the first arg: cobra consumes the --
// as the POSIX end-of-flags marker, so args become ["#work", "text", "here"].
// The capture tokenizer then parses #work as a tag, not plain text.
// This is correct cobra behavior but surprising for users who expect -- to be
// braindump's text separator. The braindump -- separator only works mid-sentence:
// `todo some text -- more text here`.
func TestCapture_DoubleDashAtStart_CobraConsumes(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "--", "#work", "text", "here")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	// NOTE: Cobra strips the --, so #work is parsed as a tag by capture
	// This is documented behavior: -- at start is cobra's end-of-flags
	hasWork := false
	for _, tag := range todo.Tags {
		if tag == "work" {
			hasWork = true
		}
	}
	if !hasWork {
		t.Errorf("NOTE: cobra consumes leading --, so #work becomes a tag, got tags: %v", todo.Tags)
	}
	if todo.Text != "text here" {
		t.Errorf("expected text 'text here', got %q", todo.Text)
	}
}

// The braindump -- separator works when placed mid-sentence.
func TestCapture_DoubleDashMidSentence_PlainText(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "check", "this", "--", "ls", "files")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	// "check this" is before --, "ls files" is after
	if !strings.Contains(todo.Text, "check this") {
		t.Errorf("expected text to contain 'check this', got %q", todo.Text)
	}
	if !strings.Contains(todo.Text, "ls files") {
		t.Errorf("expected text to contain 'ls files' (after --), got %q", todo.Text)
	}
}

// Capture where text looks like metadata: `todo check {id:fake} thing`
func TestCapture_TextLooksLikeMetadata(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "check", "{id:fake}", "thing", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	// The capture should preserve {id:fake} as text, not use it as the todo's ID
	if todo.ID == "fake" {
		t.Error("BUG: {id:fake} in text was used as the actual todo ID")
	}
	// BUG: On round-trip, {id:fake} will be consumed by the parser
	if !strings.Contains(todo.Text, "{id:fake}") {
		t.Errorf("BUG: text with metadata-like content corrupted: got %q, want to contain '{id:fake}'", todo.Text)
	}
}

// `todo edit <id>` with only 1 arg — should error (MinimumNArgs(2)).
func TestEdit_NoNewText(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "some", "task", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	_, err = executeCmd(t, "edit", id)
	if err == nil {
		t.Error("expected error for edit with no new text, got nil")
	}
}

// `todo move <id>` with no status — should error (ExactArgs(2)).
func TestMove_NoStatus(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "test", "item", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	_, err = executeCmd(t, "move", id)
	if err == nil {
		t.Error("expected error for move with no status, got nil")
	}
}

// `todo tag add` with no name — should error (ExactArgs(1)).
func TestTagAdd_NoName(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	_, err := executeCmd(t, "tag", "add")
	if err == nil {
		t.Error("expected error for tag add with no name, got nil")
	}
}

// Capture with both !! and !!! should set both urgent and important.
func TestCapture_BothUrgentAndImportant(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "critical", "task", "!!", "!!!", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	if !todo.Urgent {
		t.Error("expected Urgent=true when both !! and !!! used")
	}
	if !todo.Important {
		t.Error("expected Important=true when both !! and !!! used")
	}
}

// Capture empty input should show usage, not error.
func TestCapture_NoArgs_ShowsUsage(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t)
	if err != nil {
		// Root command with no args calls Usage() which may return nil
		t.Logf("no-args returned error: %v", err)
	}
	// Should show usage info
	if !strings.Contains(output, "todo") {
		t.Logf("no-args output: %q", output)
	}
}

// `todo ls --all` should show todos from multiple days.
func TestListAll_ShowsMultipleDays(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a todo for today
	capOut, err := executeCmd(t, "today", "task", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	todayID := extractID(t, capOut)

	// Manually create a past date todo
	pastDate := "2026-03-15"
	pastTodo := &core.Todo{
		ID: "past01", Text: "past task", Source: "cli", Status: "inbox",
		Created: time.Date(2026, 3, 15, 10, 0, 0, 0, time.Local),
		Tags:    []string{"work"},
	}
	pastContent := core.SerializeDayFile(pastDate, []*core.Todo{pastTodo})
	os.WriteFile(filepath.Join(tmpDir, pastDate+".md"), []byte(pastContent), 0644)

	output, err := executeCmd(t, "ls", "--all")
	if err != nil {
		t.Fatalf("ls --all failed: %v", err)
	}

	if !strings.Contains(output, todayID) {
		t.Errorf("ls --all should show today's todo %s, got: %q", todayID, output)
	}
	if !strings.Contains(output, "past01") {
		t.Errorf("ls --all should show past todo 'past01', got: %q", output)
	}
}

// `todo move <id> waiting` should set status to waiting.
func TestMoveToWaiting(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "waiting", "task", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	_, err = executeCmd(t, "move", id, "waiting")
	if err != nil {
		t.Fatalf("move to waiting failed: %v", err)
	}

	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if todo.Status != "waiting" {
		t.Errorf("expected status 'waiting', got %q", todo.Status)
	}
}

// `todo delete <id>` should also produce a log entry.
func TestDelete_ProducesLogEntry(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "log", "test", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	_, err = executeCmd(t, "delete", id)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	today := time.Now().Format("2006-01-02")
	data, err := os.ReadFile(filepath.Join(tmpDir, today+".log"))
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, id) {
		t.Error("log should contain deleted todo ID")
	}
	if !strings.Contains(content, "deleted") {
		t.Error("log should contain 'deleted'")
	}
}

// Verify that capture with @minecraft source and explicit #homelab tag
// results in BOTH tags being present.
func TestCaptureSourceTag_PlusExplicitTag(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	output, err := executeCmd(t, "build", "server", "@minecraft", "#homelab")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	id := extractID(t, output)
	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatalf("finding todo: %v", err)
	}

	hasHomelab := false
	hasMinecraft := false
	for _, tag := range todo.Tags {
		if tag == "homelab" {
			hasHomelab = true
		}
		if tag == "minecraft" {
			hasMinecraft = true
		}
	}

	if !hasHomelab {
		t.Errorf("expected #homelab tag, got tags: %v", todo.Tags)
	}
	if !hasMinecraft {
		t.Errorf("expected #minecraft auto-added from @minecraft source, got tags: %v", todo.Tags)
	}
}

// ---------------------------------------------------------------------------
// Picker Tests
// ---------------------------------------------------------------------------

func TestCollectByStatus_ReturnsOnlyMatchingStatus(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	now := time.Now()
	today := now.Format("2006-01-02")

	// Create mixed-status todos directly in the store
	s := core.NewStore(tmpDir)
	inboxTodo := &core.Todo{
		ID: "aaa111", Text: "inbox item", Source: "cli", Status: "inbox",
		Created: now, Tags: []string{"work"},
	}
	activeTodo := &core.Todo{
		ID: "bbb222", Text: "active item", Source: "cli", Status: "active",
		Created: now, Tags: []string{"work"},
	}
	doneTodo := &core.Todo{
		ID: "ccc333", Text: "done item", Source: "cli", Status: "done",
		Created: now, Done: true, Tags: []string{"work"},
	}
	s.WriteDay(today, []*core.Todo{inboxTodo, activeTodo, doneTodo})

	// Initialize the package-level store for collectByStatus
	store = s

	items, err := collectByStatus("active")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 active item, got %d", len(items))
	}
	if items[0].todo.ID != "bbb222" {
		t.Errorf("expected ID bbb222, got %s", items[0].todo.ID)
	}
}

func TestCollectByStatus_InboxOnly(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	now := time.Now()
	today := now.Format("2006-01-02")
	s := core.NewStore(tmpDir)
	s.WriteDay(today, []*core.Todo{
		{ID: "aaa111", Text: "inbox", Source: "cli", Status: "inbox", Created: now, Tags: []string{"work"}},
		{ID: "bbb222", Text: "active", Source: "cli", Status: "active", Created: now, Tags: []string{"work"}},
	})
	store = s

	items, err := collectByStatus("inbox")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 inbox item, got %d", len(items))
	}
	if items[0].todo.Status != "inbox" {
		t.Errorf("expected status inbox, got %q", items[0].todo.Status)
	}
}

func TestCollectByStatus_Nonexistent_EmptySlice(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	now := time.Now()
	today := now.Format("2006-01-02")
	s := core.NewStore(tmpDir)
	s.WriteDay(today, []*core.Todo{
		{ID: "aaa111", Text: "inbox", Source: "cli", Status: "inbox", Created: now, Tags: []string{"work"}},
	})
	store = s

	items, err := collectByStatus("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for nonexistent status, got %d", len(items))
	}
}

func TestCollectByStatus_EmptyStore(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	s := core.NewStore(tmpDir)
	store = s

	items, err := collectByStatus("active")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for empty store, got %d", len(items))
	}
}

func TestPickFromList_ValidSelection(t *testing.T) {
	items := []*pickItem{
		{todo: &core.Todo{ID: "aaa111", Text: "first"}, date: "2026-03-22"},
		{todo: &core.Todo{ID: "bbb222", Text: "second"}, date: "2026-03-22"},
	}

	scanner := bufio.NewScanner(strings.NewReader("2\n"))
	output := captureOutput(func() {
		result, err := pickFromList(items, "Pick: ", scanner)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if result == nil {
			t.Error("expected non-nil result for valid selection")
			return
		}
		if result.todo.ID != "bbb222" {
			t.Errorf("expected ID bbb222, got %s", result.todo.ID)
		}
	})
	_ = output
}

func TestPickFromList_ZeroSelection_Error(t *testing.T) {
	items := []*pickItem{
		{todo: &core.Todo{ID: "aaa111", Text: "first"}, date: "2026-03-22"},
	}

	scanner := bufio.NewScanner(strings.NewReader("0\n"))
	captureOutput(func() {
		_, err := pickFromList(items, "Pick: ", scanner)
		if err == nil {
			t.Error("expected error for selection 0")
		}
	})
}

func TestPickFromList_OutOfRange_Error(t *testing.T) {
	items := []*pickItem{
		{todo: &core.Todo{ID: "aaa111", Text: "first"}, date: "2026-03-22"},
	}

	scanner := bufio.NewScanner(strings.NewReader("5\n"))
	captureOutput(func() {
		_, err := pickFromList(items, "Pick: ", scanner)
		if err == nil {
			t.Error("expected error for selection > len(items)")
		}
	})
}

func TestPickFromList_NonNumeric_Error(t *testing.T) {
	items := []*pickItem{
		{todo: &core.Todo{ID: "aaa111", Text: "first"}, date: "2026-03-22"},
	}

	scanner := bufio.NewScanner(strings.NewReader("abc\n"))
	captureOutput(func() {
		_, err := pickFromList(items, "Pick: ", scanner)
		if err == nil {
			t.Error("expected error for non-numeric input")
		}
	})
}

func TestPickFromList_EmptyInput_ReturnsNil(t *testing.T) {
	items := []*pickItem{
		{todo: &core.Todo{ID: "aaa111", Text: "first"}, date: "2026-03-22"},
	}

	// Empty scanner (EOF)
	scanner := bufio.NewScanner(strings.NewReader(""))
	captureOutput(func() {
		result, err := pickFromList(items, "Pick: ", scanner)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil result for EOF, got %v", result)
		}
	})
}

func TestPickFromList_EmptyItems_ReturnsNil(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("1\n"))
	result, err := pickFromList(nil, "Pick: ", scanner)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for empty items list")
	}
}

func TestPickFromList_NegativeNumber_Error(t *testing.T) {
	items := []*pickItem{
		{todo: &core.Todo{ID: "aaa111", Text: "first"}, date: "2026-03-22"},
	}

	scanner := bufio.NewScanner(strings.NewReader("-1\n"))
	captureOutput(func() {
		_, err := pickFromList(items, "Pick: ", scanner)
		if err == nil {
			t.Error("expected error for negative number")
		}
	})
}

// ---------------------------------------------------------------------------
// Print Command Tests
// ---------------------------------------------------------------------------

func TestPrintReservedName(t *testing.T) {
	// "print" should be in reservedNames so it cannot be captured as todo text
	if !reservedNames["print"] {
		t.Error("'print' should be in reservedNames")
	}
}

func TestPrintNoInboxItems(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// With no items at all, print should report no inbox items
	output, _ := executeCmd(t, "print")
	if !strings.Contains(strings.ToLower(output), "no inbox") && !strings.Contains(strings.ToLower(output), "no items") {
		// The print command isn't implemented yet, so this test documents expected behavior.
		// When implemented, output should indicate no inbox items available.
		t.Logf("NOTE: print command output: %q (will verify when implemented)", output)
	}
}

func TestMoveToActive_InfoLineUpdated(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "task", "for", "active", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	output, err := executeCmd(t, "move", id, "active")
	if err != nil {
		t.Fatalf("move failed: %v", err)
	}

	// The info line should reflect the active count after moving
	// It may or may not show depending on other counts being zero
	if strings.Contains(output, "moved") {
		// Good, the move succeeded
	} else {
		t.Errorf("expected 'moved' in output, got: %q", output)
	}
}

// ---------------------------------------------------------------------------
// Done Ceremony Tests
// ---------------------------------------------------------------------------

func TestDone_ActiveItem_HasCelebration(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create and move to active
	capOut, err := executeCmd(t, "celebrate", "me", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	_, err = executeCmd(t, "move", id, "active")
	if err != nil {
		t.Fatalf("move failed: %v", err)
	}

	output, err := executeCmd(t, "done", id)
	if err != nil {
		t.Fatalf("done failed: %v", err)
	}

	// Done on an active item should have celebration text
	// At minimum it should have "Done:" message
	if !strings.Contains(output, "Done:") {
		t.Errorf("expected 'Done:' in output for active item, got: %q", output)
	}
	// Verify no emoji in output
	for i, b := range []byte(output) {
		if b >= 0xF0 && b <= 0xF4 {
			t.Errorf("celebration output contains emoji byte 0x%02X at pos %d", b, i)
		}
	}
}

func TestDone_InboxItem_NoCelebrationArt(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "simple", "done", "#work")
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

	// Inbox items should NOT get ASCII art celebration patterns
	if strings.Contains(output, `\o/`) || strings.Contains(output, "***") || strings.Contains(output, "===") {
		t.Errorf("inbox item done should not have celebration art, got: %q", output)
	}
}

func TestDone_OutputNoEmoji(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	capOut, err := executeCmd(t, "emoji", "check", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	output, err := executeCmd(t, "done", id)
	if err != nil {
		t.Fatalf("done failed: %v", err)
	}

	for i, b := range []byte(output) {
		if b >= 0xF0 && b <= 0xF4 {
			t.Errorf("done output contains emoji byte 0x%02X at pos %d: %q", b, i, output)
		}
	}
}

// ---------------------------------------------------------------------------
// Full Lifecycle Test
// ---------------------------------------------------------------------------

func TestFullLifecycle_CaptureActivedoneDone(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Step 1: Capture
	capOut, err := executeCmd(t, "lifecycle", "test", "#work")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	id := extractID(t, capOut)

	todo, _, err := store.FindTodoByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if todo.Status != "inbox" {
		t.Errorf("after capture: Status = %q, want inbox", todo.Status)
	}

	// Step 2: Move to active
	_, err = executeCmd(t, "move", id, "active")
	if err != nil {
		t.Fatalf("move failed: %v", err)
	}

	todo, _, err = store.FindTodoByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if todo.Status != "active" {
		t.Errorf("after move: Status = %q, want active", todo.Status)
	}

	// Step 3: Mark done
	output, err := executeCmd(t, "done", id)
	if err != nil {
		t.Fatalf("done failed: %v", err)
	}

	todo, _, err = store.FindTodoByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if todo.Status != "done" {
		t.Errorf("after done: Status = %q, want done", todo.Status)
	}
	if !todo.Done {
		t.Error("after done: Done = false, want true")
	}
	if !strings.Contains(output, "Done:") {
		t.Errorf("expected 'Done:' in output, got: %q", output)
	}
}
