package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
