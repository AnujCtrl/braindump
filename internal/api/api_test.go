package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anujp/braindump/internal/core"
)

func setupTestAPI(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	tmpDir := t.TempDir()

	tagsContent := `categories:
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
	os.WriteFile(filepath.Join(tmpDir, "tags.yaml"), []byte(tagsContent), 0644)

	store := core.NewStore(tmpDir)
	logger := core.NewLogger(tmpDir)
	tagStore, err := core.NewTagStore(filepath.Join(tmpDir, "tags.yaml"))
	if err != nil {
		t.Fatalf("failed to create tag store: %v", err)
	}

	router := NewRouter(store, logger, tagStore)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	return server, tmpDir
}

func postJSON(t *testing.T, url string, body interface{}) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func putJSON(t *testing.T, url string, body interface{}) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	return resp
}

func patchJSON(t *testing.T, url string, body interface{}) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new PATCH request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", url, err)
	}
	return resp
}

func doDelete(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("new DELETE request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	return resp
}

func readJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("unmarshal response %q: %v", string(body), err)
	}
}

// createTodo is a helper that creates a todo and returns its ID.
func createTodo(t *testing.T, serverURL string, text string, tags []string) string {
	t.Helper()
	body := map[string]interface{}{"text": text}
	if tags != nil {
		body["tags"] = tags
	}
	resp := postJSON(t, serverURL+"/api/todo", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("createTodo: expected 201, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	readJSON(t, resp, &result)
	id, ok := result["id"].(string)
	if !ok || id == "" {
		t.Fatalf("createTodo: missing id in response")
	}
	return id
}

// ---------- Tests ----------

func TestHealthEndpoint(t *testing.T) {
	server, _ := setupTestAPI(t)

	resp, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	readJSON(t, resp, &result)
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", result["status"])
	}
}

func TestCreateTodoWithTags(t *testing.T) {
	server, _ := setupTestAPI(t)

	body := map[string]interface{}{
		"text": "set up proxmox cluster",
		"tags": []string{"homelab"},
	}
	resp := postJSON(t, server.URL+"/api/todo", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	readJSON(t, resp, &result)

	if result["id"] == nil || result["id"].(string) == "" {
		t.Error("expected non-empty id")
	}
	if result["text"] != "set up proxmox cluster" {
		t.Errorf("expected text 'set up proxmox cluster', got %q", result["text"])
	}
	tags, ok := result["tags"].([]interface{})
	if !ok || len(tags) != 1 || tags[0] != "homelab" {
		t.Errorf("expected tags [homelab], got %v", result["tags"])
	}
	if result["status"] != "inbox" {
		t.Errorf("expected status inbox, got %q", result["status"])
	}
}

func TestCreateTodoDefaultSource(t *testing.T) {
	server, _ := setupTestAPI(t)

	body := map[string]interface{}{
		"text": "buy milk",
	}
	resp := postJSON(t, server.URL+"/api/todo", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	readJSON(t, resp, &result)

	if result["source"] != "api" {
		t.Errorf("expected default source 'api', got %q", result["source"])
	}
}

func TestCreateTodoWithNote(t *testing.T) {
	server, _ := setupTestAPI(t)

	body := map[string]interface{}{
		"text": "research k8s",
		"note": "check lightweight distros like k3s",
	}
	resp := postJSON(t, server.URL+"/api/todo", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	readJSON(t, resp, &result)

	notes, ok := result["notes"].([]interface{})
	if !ok || len(notes) != 1 {
		t.Fatalf("expected 1 note, got %v", result["notes"])
	}
	if notes[0] != "check lightweight distros like k3s" {
		t.Errorf("expected note content, got %q", notes[0])
	}
}

func TestCreateTodoWithUrgentImportant(t *testing.T) {
	server, _ := setupTestAPI(t)

	body := map[string]interface{}{
		"text":      "fix broken server",
		"urgent":    true,
		"important": true,
	}
	resp := postJSON(t, server.URL+"/api/todo", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	readJSON(t, resp, &result)

	if result["urgent"] != true {
		t.Errorf("expected urgent=true, got %v", result["urgent"])
	}
	if result["important"] != true {
		t.Errorf("expected important=true, got %v", result["important"])
	}
}

func TestListTodosNoFilter(t *testing.T) {
	server, _ := setupTestAPI(t)

	createTodo(t, server.URL, "task one", nil)
	createTodo(t, server.URL, "task two", nil)

	resp, err := http.Get(server.URL + "/api/todo")
	if err != nil {
		t.Fatalf("GET /api/todo: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result []map[string]interface{}
	readJSON(t, resp, &result)

	if len(result) != 2 {
		t.Errorf("expected 2 todos, got %d", len(result))
	}
}

func TestListTodosFilterByDate(t *testing.T) {
	server, _ := setupTestAPI(t)

	createTodo(t, server.URL, "today task", nil)

	// Use today's date since todos are created with time.Now()
	resp, err := http.Get(server.URL + "/api/todo?date=2000-01-01")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}

	var result []map[string]interface{}
	readJSON(t, resp, &result)

	if len(result) != 0 {
		t.Errorf("expected 0 todos for non-matching date, got %d", len(result))
	}
}

func TestListTodosFilterByTag(t *testing.T) {
	server, _ := setupTestAPI(t)

	createTodo(t, server.URL, "homelab task", []string{"homelab"})
	createTodo(t, server.URL, "work task", []string{"work"})

	resp, err := http.Get(server.URL + "/api/todo?tag=homelab")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}

	var result []map[string]interface{}
	readJSON(t, resp, &result)

	if len(result) != 1 {
		t.Fatalf("expected 1 todo with tag homelab, got %d", len(result))
	}
	if result[0]["text"] != "homelab task" {
		t.Errorf("expected 'homelab task', got %q", result[0]["text"])
	}
}

func TestListTodosFilterByStatus(t *testing.T) {
	server, _ := setupTestAPI(t)

	createTodo(t, server.URL, "inbox item", nil)

	// All created todos have status "inbox"
	resp, err := http.Get(server.URL + "/api/todo?status=inbox")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}

	var result []map[string]interface{}
	readJSON(t, resp, &result)

	if len(result) != 1 {
		t.Errorf("expected 1 inbox todo, got %d", len(result))
	}

	// Filter by status that doesn't match
	resp2, err := http.Get(server.URL + "/api/todo?status=done")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}

	var result2 []map[string]interface{}
	readJSON(t, resp2, &result2)

	if len(result2) != 0 {
		t.Errorf("expected 0 done todos, got %d", len(result2))
	}
}

func TestListTodosEmptyResult(t *testing.T) {
	server, _ := setupTestAPI(t)

	resp, err := http.Get(server.URL + "/api/todo")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Should be an empty JSON array, not null
	trimmed := bytes.TrimSpace(body)
	if string(trimmed) != "[]" {
		t.Errorf("expected empty array '[]', got %q", string(trimmed))
	}
}

func TestEditTodoText(t *testing.T) {
	server, _ := setupTestAPI(t)

	id := createTodo(t, server.URL, "original text", nil)

	newText := "updated text"
	resp := putJSON(t, server.URL+"/api/todo/"+id, map[string]interface{}{
		"text": newText,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	readJSON(t, resp, &result)

	if result["text"] != "updated text" {
		t.Errorf("expected 'updated text', got %q", result["text"])
	}
}

func TestEditTodoTags(t *testing.T) {
	server, _ := setupTestAPI(t)

	id := createTodo(t, server.URL, "tag test", []string{"work"})

	resp := putJSON(t, server.URL+"/api/todo/"+id, map[string]interface{}{
		"tags": []string{"homelab", "minecraft"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	readJSON(t, resp, &result)

	tags := result["tags"].([]interface{})
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}
	if tags[0] != "homelab" || tags[1] != "minecraft" {
		t.Errorf("expected [homelab minecraft], got %v", tags)
	}
}

func TestEditTodoNotFound(t *testing.T) {
	server, _ := setupTestAPI(t)

	resp := putJSON(t, server.URL+"/api/todo/nonexistent", map[string]interface{}{
		"text": "nope",
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteTodo(t *testing.T) {
	server, _ := setupTestAPI(t)

	id := createTodo(t, server.URL, "to be deleted", nil)

	resp := doDelete(t, server.URL+"/api/todo/"+id)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verify it's gone
	listResp, err := http.Get(server.URL + "/api/todo")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	var result []map[string]interface{}
	readJSON(t, listResp, &result)
	if len(result) != 0 {
		t.Errorf("expected 0 todos after delete, got %d", len(result))
	}
}

func TestDeleteTodoNotFound(t *testing.T) {
	server, _ := setupTestAPI(t)

	resp := doDelete(t, server.URL+"/api/todo/nonexistent")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestChangeStatusToToday(t *testing.T) {
	server, _ := setupTestAPI(t)

	id := createTodo(t, server.URL, "promote me", nil)

	resp := patchJSON(t, server.URL+"/api/todo/"+id+"/status", map[string]string{
		"status": "today",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	readJSON(t, resp, &result)

	if result["status"] != "today" {
		t.Errorf("expected status 'today', got %q", result["status"])
	}
}

func TestChangeStatusToDoneSetsFlag(t *testing.T) {
	server, _ := setupTestAPI(t)

	id := createTodo(t, server.URL, "finish me", nil)

	resp := patchJSON(t, server.URL+"/api/todo/"+id+"/status", map[string]string{
		"status": "done",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	readJSON(t, resp, &result)

	if result["status"] != "done" {
		t.Errorf("expected status 'done', got %q", result["status"])
	}
	if result["done"] != true {
		t.Errorf("expected done=true, got %v", result["done"])
	}
}

func TestChangeStatusInvalid(t *testing.T) {
	server, _ := setupTestAPI(t)

	id := createTodo(t, server.URL, "bad status", nil)

	resp := patchJSON(t, server.URL+"/api/todo/"+id+"/status", map[string]string{
		"status": "invalid_status",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestBulkCreate(t *testing.T) {
	server, _ := setupTestAPI(t)

	body := map[string]interface{}{
		"items": []map[string]interface{}{
			{"text": "dump item 1"},
			{"text": "dump item 2"},
			{"text": "dump item 3"},
		},
	}
	resp := postJSON(t, server.URL+"/api/dump", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result struct {
		Count int              `json:"count"`
		Todos []map[string]interface{} `json:"todos"`
	}
	readJSON(t, resp, &result)

	if result.Count != 3 {
		t.Errorf("expected count=3, got %d", result.Count)
	}
	if len(result.Todos) != 3 {
		t.Errorf("expected 3 todos, got %d", len(result.Todos))
	}

	// All items should have status "unprocessed"
	for i, todo := range result.Todos {
		if todo["status"] != "unprocessed" {
			t.Errorf("todo[%d]: expected status 'unprocessed', got %q", i, todo["status"])
		}
	}
}

func TestBulkCreateDefaultTag(t *testing.T) {
	server, _ := setupTestAPI(t)

	body := map[string]interface{}{
		"items": []map[string]interface{}{
			{"text": "no tag item"},
		},
		"default_tag": "braindump",
	}
	resp := postJSON(t, server.URL+"/api/dump", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result struct {
		Count int              `json:"count"`
		Todos []map[string]interface{} `json:"todos"`
	}
	readJSON(t, resp, &result)

	if result.Count != 1 {
		t.Fatalf("expected count=1, got %d", result.Count)
	}

	tags := result.Todos[0]["tags"].([]interface{})
	if len(tags) != 1 || tags[0] != "braindump" {
		t.Errorf("expected tags [braindump], got %v", tags)
	}
}

func TestListTags(t *testing.T) {
	server, _ := setupTestAPI(t)

	resp, err := http.Get(server.URL + "/api/tags")
	if err != nil {
		t.Fatalf("GET /api/tags: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string][]string
	readJSON(t, resp, &result)

	categories := result["categories"]
	if len(categories) != 5 {
		t.Errorf("expected 5 categories, got %d: %v", len(categories), categories)
	}

	energy := result["energy"]
	if len(energy) != 3 {
		t.Errorf("expected 3 energy tags, got %d: %v", len(energy), energy)
	}

	// Check specific tags are present
	found := false
	for _, c := range categories {
		if c == "homelab" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'homelab' in categories")
	}
}

func TestInfoEndpoint(t *testing.T) {
	server, _ := setupTestAPI(t)

	// With no todos, both counts should be 0
	resp, err := http.Get(server.URL + "/api/info")
	if err != nil {
		t.Fatalf("GET /api/info: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	readJSON(t, resp, &result)

	if result["unprocessed"].(float64) != 0 {
		t.Errorf("expected unprocessed=0, got %v", result["unprocessed"])
	}
	if result["looping"].(float64) != 0 {
		t.Errorf("expected looping=0, got %v", result["looping"])
	}
}

func TestInfoWithUnprocessed(t *testing.T) {
	server, _ := setupTestAPI(t)

	// Create items via dump (they get status "unprocessed")
	body := map[string]interface{}{
		"items": []map[string]interface{}{
			{"text": "unprocessed 1"},
			{"text": "unprocessed 2"},
		},
	}
	resp := postJSON(t, server.URL+"/api/dump", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	infoResp, err := http.Get(server.URL + "/api/info")
	if err != nil {
		t.Fatalf("GET /api/info: %v", err)
	}

	var result map[string]interface{}
	readJSON(t, infoResp, &result)

	if result["unprocessed"].(float64) != 2 {
		t.Errorf("expected unprocessed=2, got %v", result["unprocessed"])
	}
}

// ---------------------------------------------------------------------------
// NEW TESTS: Additional API coverage
// ---------------------------------------------------------------------------

// GET /api/todo?tag=homelab should only return todos with that tag, not all.
func TestListTodosFilterByTag_ExcludesOtherTags(t *testing.T) {
	server, _ := setupTestAPI(t)

	createTodo(t, server.URL, "homelab task", []string{"homelab"})
	createTodo(t, server.URL, "work task", []string{"work"})
	createTodo(t, server.URL, "both tags", []string{"homelab", "work"})

	resp, err := http.Get(server.URL + "/api/todo?tag=homelab")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}

	var result []map[string]interface{}
	readJSON(t, resp, &result)

	if len(result) != 2 {
		t.Fatalf("expected 2 todos with tag homelab, got %d", len(result))
	}

	for _, todo := range result {
		tags := todo["tags"].([]interface{})
		hasHomelab := false
		for _, tag := range tags {
			if tag == "homelab" {
				hasHomelab = true
			}
		}
		if !hasHomelab {
			t.Errorf("todo %q should have homelab tag, got tags: %v", todo["text"], tags)
		}
	}
}

// POST /api/todo with empty text should return 400.
func TestCreateTodoEmptyText(t *testing.T) {
	server, _ := setupTestAPI(t)

	resp := postJSON(t, server.URL+"/api/todo", map[string]interface{}{
		"text": "",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty text, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// POST /api/todo with whitespace-only text should return 400.
func TestCreateTodoWhitespaceOnlyText(t *testing.T) {
	server, _ := setupTestAPI(t)

	resp := postJSON(t, server.URL+"/api/todo", map[string]interface{}{
		"text": "   \t  ",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for whitespace-only text, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// PATCH /api/todo/:id/status with non-existent ID should return 404.
func TestChangeStatusNonExistentID(t *testing.T) {
	server, _ := setupTestAPI(t)

	resp := patchJSON(t, server.URL+"/api/todo/zzz999/status", map[string]string{
		"status": "today",
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// POST /api/dump with empty items array should return 201 with count=0.
func TestBulkCreateEmptyItems(t *testing.T) {
	server, _ := setupTestAPI(t)

	body := map[string]interface{}{
		"items": []map[string]interface{}{},
	}
	resp := postJSON(t, server.URL+"/api/dump", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result struct {
		Count int              `json:"count"`
		Todos []map[string]interface{} `json:"todos"`
	}
	readJSON(t, resp, &result)

	if result.Count != 0 {
		t.Errorf("expected count=0, got %d", result.Count)
	}
	if len(result.Todos) != 0 {
		t.Errorf("expected 0 todos, got %d", len(result.Todos))
	}
}

// POST /api/dump with items containing empty text should skip those items.
func TestBulkCreateSkipsEmptyText(t *testing.T) {
	server, _ := setupTestAPI(t)

	body := map[string]interface{}{
		"items": []map[string]interface{}{
			{"text": "valid item"},
			{"text": ""},
			{"text": "   "},
			{"text": "another valid"},
		},
	}
	resp := postJSON(t, server.URL+"/api/dump", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result struct {
		Count int              `json:"count"`
		Todos []map[string]interface{} `json:"todos"`
	}
	readJSON(t, resp, &result)

	if result.Count != 2 {
		t.Errorf("expected count=2 (skipping empty text items), got %d", result.Count)
	}
}

// --- BREAKING TESTS: API edge cases ---

// POST /api/todo with no Content-Type header — should still work since Go's
// json.Decoder doesn't check Content-Type.
func TestCreateTodo_NoContentTypeHeader(t *testing.T) {
	server, _ := setupTestAPI(t)

	body := `{"text": "no content type"}`
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/todo", strings.NewReader(body))
	// Deliberately NOT setting Content-Type
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	// NOTE: Go's json.Decoder works regardless of Content-Type header
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201 (json.Decoder doesn't check Content-Type), got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// POST /api/todo with empty body — should return 400.
func TestCreateTodo_EmptyBody(t *testing.T) {
	server, _ := setupTestAPI(t)

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/todo", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty body, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// POST /api/todo with invalid JSON — should return 400.
func TestCreateTodo_InvalidJSON(t *testing.T) {
	server, _ := setupTestAPI(t)

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/todo", strings.NewReader(`{broken`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// POST /api/todo with extra unknown fields — should be silently ignored.
func TestCreateTodo_ExtraFields_Ignored(t *testing.T) {
	server, _ := setupTestAPI(t)

	body := map[string]interface{}{
		"text":          "with extra fields",
		"unknown_field": "should be ignored",
		"another":       42,
	}
	resp := postJSON(t, server.URL+"/api/todo", body)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201 (extra fields ignored), got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	readJSON(t, resp, &result)
	if result["text"] != "with extra fields" {
		t.Errorf("text = %q, want 'with extra fields'", result["text"])
	}
}

// NOTE: API does NOT validate tags against tags.yaml — any tags are accepted.
// This documents that behavior (could be a bug or intentional).
func TestCreateTodo_InvalidTags_NotValidated(t *testing.T) {
	server, _ := setupTestAPI(t)

	body := map[string]interface{}{
		"text": "invalid tags test",
		"tags": []string{"nonexistent_tag", "also_fake"},
	}
	resp := postJSON(t, server.URL+"/api/todo", body)
	// NOTE: The API accepts any tags without validation against tags.yaml.
	// The CLI validates tags but the API does not.
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("NOTE: API accepted invalid tags (no validation), got status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	readJSON(t, resp, &result)
	tags := result["tags"].([]interface{})
	if len(tags) != 2 || tags[0] != "nonexistent_tag" {
		t.Errorf("tags = %v, expected [nonexistent_tag also_fake]", tags)
	}
}

// PATCH /api/todo/:id/status with empty body — should return 400.
func TestChangeStatus_EmptyBody(t *testing.T) {
	server, _ := setupTestAPI(t)

	id := createTodo(t, server.URL, "test", nil)

	req, _ := http.NewRequest(http.MethodPatch, server.URL+"/api/todo/"+id+"/status", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty body on status change, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// GET /api/todo with multiple filters combined (date + tag + status).
func TestListTodos_MultipleFiltersCombined(t *testing.T) {
	server, _ := setupTestAPI(t)

	createTodo(t, server.URL, "matching task", []string{"homelab"})
	createTodo(t, server.URL, "wrong tag", []string{"work"})

	today := time.Now().Format("2006-01-02")
	url := fmt.Sprintf("%s/api/todo?date=%s&tag=homelab&status=inbox", server.URL, today)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}

	var result []map[string]interface{}
	readJSON(t, resp, &result)

	if len(result) != 1 {
		t.Errorf("expected 1 todo matching all filters, got %d", len(result))
	}
	if len(result) > 0 && result[0]["text"] != "matching task" {
		t.Errorf("wrong todo returned: %q", result[0]["text"])
	}
}

// POST /api/dump with 100 items — all created.
func TestBulkCreate_100Items(t *testing.T) {
	server, _ := setupTestAPI(t)

	items := make([]map[string]interface{}, 100)
	for i := range items {
		items[i] = map[string]interface{}{"text": fmt.Sprintf("dump item %d", i)}
	}

	body := map[string]interface{}{"items": items}
	resp := postJSON(t, server.URL+"/api/dump", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result struct {
		Count int              `json:"count"`
		Todos []map[string]interface{} `json:"todos"`
	}
	readJSON(t, resp, &result)

	if result.Count != 100 {
		t.Errorf("expected count=100, got %d", result.Count)
	}
	if len(result.Todos) != 100 {
		t.Errorf("expected 100 todos, got %d", len(result.Todos))
	}
}

// DELETE /api/todo/:id then GET same id should not be in list.
func TestDeleteThenGet_NotFound(t *testing.T) {
	server, _ := setupTestAPI(t)

	id := createTodo(t, server.URL, "delete and check", nil)

	resp := doDelete(t, server.URL+"/api/todo/"+id)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Try to edit the deleted todo — should get 404
	editResp := putJSON(t, server.URL+"/api/todo/"+id, map[string]interface{}{
		"text": "should fail",
	})
	if editResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", editResp.StatusCode)
	}
	editResp.Body.Close()
}

// GET /api/todo?tag=HOMELAB — case sensitivity test.
func TestListTodos_TagCaseInsensitive(t *testing.T) {
	server, _ := setupTestAPI(t)

	createTodo(t, server.URL, "homelab task", []string{"homelab"})

	// The API handler uses strings.EqualFold for tag comparison
	resp, err := http.Get(server.URL + "/api/todo?tag=HOMELAB")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}

	var result []map[string]interface{}
	readJSON(t, resp, &result)

	if len(result) != 1 {
		t.Errorf("expected 1 todo (case-insensitive tag filter), got %d", len(result))
	}
}

// PUT /api/todo/:id/nonexistent — should get method not allowed or 404.
func TestAPI_UnknownSubpath(t *testing.T) {
	server, _ := setupTestAPI(t)

	id := createTodo(t, server.URL, "test", nil)

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/todo/"+id+"/nonexistent", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	// The router only handles /status suffix; anything else should fail
	if resp.StatusCode == http.StatusOK {
		t.Errorf("expected non-200 for unknown subpath, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// BUG: POST /api/todo with text ending in {braces} — data loss on round-trip.
// The parser strips trailing {} blocks from text as metadata.
func TestCreateTodo_TextEndingWithBraces_DataLoss(t *testing.T) {
	server, _ := setupTestAPI(t)

	body := map[string]interface{}{
		"text": "check the {config}",
		"tags": []string{"work"},
	}
	resp := postJSON(t, server.URL+"/api/todo", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var created map[string]interface{}
	readJSON(t, resp, &created)

	// The response should have the original text (before disk round-trip)
	if created["text"] != "check the {config}" {
		t.Errorf("created response text = %q, want 'check the {config}'", created["text"])
	}

	// Now list and check if it survived the round-trip through disk
	listResp, err := http.Get(server.URL + "/api/todo")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	var todos []map[string]interface{}
	readJSON(t, listResp, &todos)

	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	// BUG: After round-trip through file storage, trailing {config} is stripped
	if todos[0]["text"] != "check the {config}" {
		t.Errorf("BUG: Trailing braces stripped after round-trip: got %q, want %q",
			todos[0]["text"], "check the {config}")
	}
}

// Verify that creating a todo via API and then listing with tag filter
// correctly round-trips through disk storage.
func TestCreateAndListRoundTrip(t *testing.T) {
	server, _ := setupTestAPI(t)

	// Create
	id := createTodo(t, server.URL, "persistent task", []string{"homelab"})

	// List (unfiltered) and verify
	resp, err := http.Get(server.URL + "/api/todo")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	var all []map[string]interface{}
	readJSON(t, resp, &all)

	found := false
	for _, todo := range all {
		if todo["id"] == id {
			found = true
			if todo["text"] != "persistent task" {
				t.Errorf("text mismatch: got %q", todo["text"])
			}
			tags := todo["tags"].([]interface{})
			if len(tags) != 1 || tags[0] != "homelab" {
				t.Errorf("tags mismatch: got %v", tags)
			}
		}
	}
	if !found {
		t.Errorf("created todo %s not found in list", id)
	}
}

// DELETE /api/todo/:id then GET should not include deleted todo.
func TestDeleteThenListExcludes(t *testing.T) {
	server, _ := setupTestAPI(t)

	id1 := createTodo(t, server.URL, "keep me", nil)
	id2 := createTodo(t, server.URL, "delete me", nil)

	resp := doDelete(t, server.URL+"/api/todo/"+id2)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	listResp, err := http.Get(server.URL + "/api/todo")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	var result []map[string]interface{}
	readJSON(t, listResp, &result)

	if len(result) != 1 {
		t.Fatalf("expected 1 todo after delete, got %d", len(result))
	}
	if result[0]["id"] != id1 {
		t.Errorf("expected surviving todo %s, got %s", id1, result[0]["id"])
	}
}
