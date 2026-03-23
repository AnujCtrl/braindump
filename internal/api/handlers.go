package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/anujp/braindump/internal/core"
)

// Handlers holds dependencies for the API handlers.
type Handlers struct {
	store    *core.Store
	logger   *core.Logger
	tagStore *core.TagStore
}

// todoJSON is the JSON-serializable representation of a Todo.
type todoJSON struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	Source     string   `json:"source"`
	Status     string   `json:"status"`
	Created    string   `json:"created"`
	Urgent     bool     `json:"urgent"`
	Important  bool     `json:"important"`
	StaleCount int      `json:"stale_count"`
	Tags       []string `json:"tags"`
	Notes      []string `json:"notes"`
	Subtasks   []string `json:"subtasks"`
	Done       bool     `json:"done"`
}

const timeFormat = "2006-01-02T15:04:05"

func toJSON(t *core.Todo) todoJSON {
	tags := t.Tags
	if tags == nil {
		tags = []string{}
	}
	notes := t.Notes
	if notes == nil {
		notes = []string{}
	}
	subtasks := t.Subtasks
	if subtasks == nil {
		subtasks = []string{}
	}
	return todoJSON{
		ID:         t.ID,
		Text:       t.Text,
		Source:     t.Source,
		Status:     t.Status,
		Created:    t.Created.Format(timeFormat),
		Urgent:     t.Urgent,
		Important:  t.Important,
		StaleCount: t.StaleCount,
		Tags:       tags,
		Notes:      notes,
		Subtasks:   subtasks,
		Done:       t.Done,
	}
}

func todosToJSON(todos []*core.Todo) []todoJSON {
	result := make([]todoJSON, 0, len(todos))
	for _, t := range todos {
		result = append(result, toJSON(t))
	}
	return result
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

var validStatuses = map[string]bool{
	"unprocessed": true,
	"inbox":       true,
	"active":      true,
	"waiting":     true,
	"done":        true,
	"stale":       true,
}

// generateUniqueID generates an ID that does not collide with existing IDs.
func generateUniqueID(store *core.Store) (string, error) {
	existing, err := store.CollectAllIDs()
	if err != nil {
		return "", err
	}
	for i := 0; i < 100; i++ {
		id, err := core.GenerateID()
		if err != nil {
			return "", err
		}
		if core.IsUniqueID(id, existing) {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique ID after 100 attempts")
}

// CreateTodo handles POST /api/todo
func (h *Handlers) CreateTodo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text      string   `json:"text"`
		Tags      []string `json:"tags"`
		Source    string   `json:"source"`
		Urgent    bool     `json:"urgent"`
		Important bool     `json:"important"`
		Note      string   `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	source := req.Source
	if source == "" {
		source = "api"
	}

	id, err := generateUniqueID(h.store)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generating ID: "+err.Error())
		return
	}

	now := time.Now()
	todo := &core.Todo{
		ID:            id,
		Text:          req.Text,
		Source:        source,
		Status:        "inbox",
		Created:       now,
		StatusChanged: now,
		Urgent:        req.Urgent,
		Important:     req.Important,
		Tags:          req.Tags,
		Done:          false,
	}

	if req.Note != "" {
		todo.Notes = []string{req.Note}
	}

	if err := h.store.AddTodo(todo); err != nil {
		writeError(w, http.StatusInternalServerError, "saving todo: "+err.Error())
		return
	}

	date := now.Format("2006-01-02")
	if err := h.logger.LogCreated(date, todo); err != nil {
		// Log failure is non-fatal, but we note it
		fmt.Printf("warning: failed to log creation: %v\n", err)
	}

	writeJSON(w, http.StatusCreated, toJSON(todo))
}

// ListTodos handles GET /api/todo
func (h *Handlers) ListTodos(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	dateFilter := query.Get("date")
	tagFilter := query.Get("tag")
	statusFilter := query.Get("status")
	sourceFilter := query.Get("source")

	var allTodos []*core.Todo

	if dateFilter != "" {
		todos, err := h.store.ReadDay(dateFilter)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "reading day: "+err.Error())
			return
		}
		allTodos = todos
	} else {
		dayMap, err := h.store.ReadAllDays()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "reading todos: "+err.Error())
			return
		}
		for _, todos := range dayMap {
			allTodos = append(allTodos, todos...)
		}
	}

	// Apply filters
	filtered := make([]*core.Todo, 0, len(allTodos))
	for _, t := range allTodos {
		if statusFilter != "" && t.Status != statusFilter {
			continue
		}
		if sourceFilter != "" && t.Source != sourceFilter {
			continue
		}
		if tagFilter != "" {
			found := false
			for _, tag := range t.Tags {
				if strings.EqualFold(tag, tagFilter) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		filtered = append(filtered, t)
	}

	writeJSON(w, http.StatusOK, todosToJSON(filtered))
}

// EditTodo handles PUT /api/todo/{id}
func (h *Handlers) EditTodo(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Text      *string  `json:"text"`
		Tags      []string `json:"tags"`
		Urgent    *bool    `json:"urgent"`
		Important *bool    `json:"important"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	todo, date, err := h.store.FindTodoByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "todo not found: "+id)
		return
	}

	// Track and apply edits
	if req.Text != nil && *req.Text != todo.Text {
		old := todo.Text
		todo.Text = *req.Text
		h.logger.LogEdit(date, id, "text", old, *req.Text)
	}
	if req.Tags != nil {
		old := strings.Join(todo.Tags, ",")
		todo.Tags = req.Tags
		h.logger.LogEdit(date, id, "tags", old, strings.Join(req.Tags, ","))
	}
	if req.Urgent != nil && *req.Urgent != todo.Urgent {
		old := boolStr(todo.Urgent)
		todo.Urgent = *req.Urgent
		h.logger.LogEdit(date, id, "urgent", old, boolStr(*req.Urgent))
	}
	if req.Important != nil && *req.Important != todo.Important {
		old := boolStr(todo.Important)
		todo.Important = *req.Important
		h.logger.LogEdit(date, id, "important", old, boolStr(*req.Important))
	}

	if err := h.store.UpdateTodo(todo); err != nil {
		writeError(w, http.StatusInternalServerError, "updating todo: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toJSON(todo))
}

// DeleteTodo handles DELETE /api/todo/{id}
func (h *Handlers) DeleteTodo(w http.ResponseWriter, r *http.Request, id string) {
	_, date, err := h.store.FindTodoByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "todo not found: "+id)
		return
	}

	if err := h.store.DeleteTodo(id); err != nil {
		writeError(w, http.StatusInternalServerError, "deleting todo: "+err.Error())
		return
	}

	h.logger.LogDelete(date, id)
	w.WriteHeader(http.StatusNoContent)
}

// ChangeStatus handles PATCH /api/todo/{id}/status
func (h *Handlers) ChangeStatus(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if !validStatuses[req.Status] {
		writeError(w, http.StatusBadRequest, "invalid status: "+req.Status)
		return
	}

	todo, date, err := h.store.FindTodoByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "todo not found: "+id)
		return
	}

	oldStatus := todo.Status
	todo.Status = req.Status
	todo.StatusChanged = time.Now()

	if req.Status == "done" {
		todo.Done = true
	}

	if err := h.store.UpdateTodo(todo); err != nil {
		writeError(w, http.StatusInternalServerError, "updating status: "+err.Error())
		return
	}

	h.logger.LogStatusChange(date, id, oldStatus, req.Status)

	writeJSON(w, http.StatusOK, toJSON(todo))
}

// BulkCreate handles POST /api/dump
func (h *Handlers) BulkCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			Text string   `json:"text"`
			Tags []string `json:"tags"`
		} `json:"items"`
		Source     string `json:"source"`
		DefaultTag string `json:"default_tag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	source := req.Source
	if source == "" {
		source = "api"
	}

	now := time.Now()
	date := now.Format("2006-01-02")
	created := make([]*core.Todo, 0, len(req.Items))

	for _, item := range req.Items {
		if strings.TrimSpace(item.Text) == "" {
			continue
		}

		id, err := generateUniqueID(h.store)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "generating ID: "+err.Error())
			return
		}

		tags := item.Tags
		if len(tags) == 0 && req.DefaultTag != "" {
			tags = []string{req.DefaultTag}
		}

		todo := &core.Todo{
			ID:      id,
			Text:    item.Text,
			Source:  source,
			Status:  "unprocessed",
			Created: now,
			Tags:    tags,
		}

		if err := h.store.AddTodo(todo); err != nil {
			writeError(w, http.StatusInternalServerError, "saving todo: "+err.Error())
			return
		}

		h.logger.LogCreated(date, todo)
		created = append(created, todo)
	}

	resp := struct {
		Count int        `json:"count"`
		Todos []todoJSON `json:"todos"`
	}{
		Count: len(created),
		Todos: todosToJSON(created),
	}
	writeJSON(w, http.StatusCreated, resp)
}

// ListTags handles GET /api/tags
func (h *Handlers) ListTags(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.tagStore.GroupedTags())
}

// Info handles GET /api/info
func (h *Handlers) Info(w http.ResponseWriter, r *http.Request) {
	info, err := core.GetInfoLine(h.store)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "getting info: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{
		"unprocessed": info.Unprocessed,
		"looping":     info.Looping,
		"active":      info.Active,
	})
}

// Health handles GET /api/health
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
