package api

import (
	"net/http"
	"strings"

	"github.com/anujp/braindump/internal/core"
)

// NewRouter creates the HTTP mux with all API routes.
func NewRouter(store *core.Store, logger *core.Logger, tagStore *core.TagStore) http.Handler {
	h := &Handlers{
		store:    store,
		logger:   logger,
		tagStore: tagStore,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/todo", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.CreateTodo(w, r)
		case http.MethodGet:
			h.ListTodos(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/todo/", func(w http.ResponseWriter, r *http.Request) {
		// Parse: /api/todo/{id} or /api/todo/{id}/status
		path := strings.TrimPrefix(r.URL.Path, "/api/todo/")
		parts := strings.SplitN(path, "/", 2)
		id := parts[0]

		if id == "" {
			http.Error(w, "missing todo id", http.StatusBadRequest)
			return
		}

		// Check for /status suffix
		if len(parts) == 2 && parts[1] == "status" {
			if r.Method == http.MethodPatch {
				h.ChangeStatus(w, r, id)
			} else {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		switch r.Method {
		case http.MethodPut:
			h.EditTodo(w, r, id)
		case http.MethodDelete:
			h.DeleteTodo(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/dump", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.BulkCreate(w, r)
	})

	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.ListTags(w, r)
	})

	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.Info(w, r)
	})

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.Health(w, r)
	})

	return mux
}
