package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Store manages reading and writing daily .md todo files.
type Store struct {
	DataDir string // path to /data/ directory
}

// NewStore creates a new Store with the given data directory path.
func NewStore(dataDir string) *Store {
	return &Store{DataDir: dataDir}
}

// TodayFile returns the path to today's .md file.
func (s *Store) TodayFile() string {
	return s.DateFile(time.Now().Format("2006-01-02"))
}

// DateFile returns the path for a specific date's .md file.
// Date format: "2006-01-02".
func (s *Store) DateFile(date string) string {
	return filepath.Join(s.DataDir, date+".md")
}

// ReadDay reads and parses a day's .md file. Returns an empty slice if the
// file doesn't exist.
func (s *Store) ReadDay(date string) ([]*Todo, error) {
	path := s.DateFile(date)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Todo{}, nil
		}
		return nil, fmt.Errorf("reading day file %s: %w", date, err)
	}
	todos, err := ParseDayFile(string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing day file %s: %w", date, err)
	}
	return todos, nil
}

// ReadAllDays reads all .md files in the data dir. Returns a map of
// date string to todos.
func (s *Store) ReadAllDays() (map[string][]*Todo, error) {
	result := make(map[string][]*Todo)

	entries, err := os.ReadDir(s.DataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("reading data dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		date := strings.TrimSuffix(name, ".md")
		// Validate that it looks like a date.
		if _, err := time.Parse("2006-01-02", date); err != nil {
			continue
		}
		todos, err := s.ReadDay(date)
		if err != nil {
			return nil, err
		}
		result[date] = todos
	}

	return result, nil
}

// WriteDay serializes and writes todos to a day's .md file. Creates the data
// directory if it doesn't exist.
func (s *Store) WriteDay(date string, todos []*Todo) error {
	if err := os.MkdirAll(s.DataDir, 0755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	content := SerializeDayFile(date, todos)
	path := s.DateFile(date)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing day file %s: %w", date, err)
	}
	return nil
}

// AddTodo adds a todo to its creation date's file. Creates the file if needed.
func (s *Store) AddTodo(todo *Todo) error {
	date := todo.Created.Format("2006-01-02")
	todos, err := s.ReadDay(date)
	if err != nil {
		return fmt.Errorf("adding todo: %w", err)
	}
	todos = append(todos, todo)
	return s.WriteDay(date, todos)
}

// FindTodoByID finds a todo by ID across all files. Returns the todo, its
// date string, and any error. Returns an error if the todo is not found.
func (s *Store) FindTodoByID(id string) (*Todo, string, error) {
	allDays, err := s.ReadAllDays()
	if err != nil {
		return nil, "", fmt.Errorf("finding todo %s: %w", id, err)
	}

	for date, todos := range allDays {
		for _, todo := range todos {
			if todo.ID == id {
				return todo, date, nil
			}
		}
	}

	return nil, "", fmt.Errorf("todo not found: %s", id)
}

// UpdateTodo finds and updates an existing todo in its file by ID.
func (s *Store) UpdateTodo(todo *Todo) error {
	_, date, err := s.FindTodoByID(todo.ID)
	if err != nil {
		return fmt.Errorf("updating todo: %w", err)
	}

	todos, err := s.ReadDay(date)
	if err != nil {
		return fmt.Errorf("updating todo: %w", err)
	}

	for i, t := range todos {
		if t.ID == todo.ID {
			todos[i] = todo
			return s.WriteDay(date, todos)
		}
	}

	return fmt.Errorf("todo not found during update: %s", todo.ID)
}

// DeleteTodo removes a todo from its file by ID.
func (s *Store) DeleteTodo(id string) error {
	_, date, err := s.FindTodoByID(id)
	if err != nil {
		return fmt.Errorf("deleting todo: %w", err)
	}

	todos, err := s.ReadDay(date)
	if err != nil {
		return fmt.Errorf("deleting todo: %w", err)
	}

	filtered := make([]*Todo, 0, len(todos))
	for _, t := range todos {
		if t.ID != id {
			filtered = append(filtered, t)
		}
	}

	return s.WriteDay(date, filtered)
}

// EnsureDateFile creates an empty date file if it doesn't exist.
// An empty file contains just the "# YYYY-MM-DD" header line.
func (s *Store) EnsureDateFile(date string) error {
	path := s.DateFile(date)
	if _, err := os.Stat(path); err == nil {
		// File already exists.
		return nil
	}
	return s.WriteDay(date, nil)
}

// BackfillGaps finds gaps between the earliest and latest .md files and
// creates empty files for missing dates. Also ensures today's file exists.
func (s *Store) BackfillGaps() error {
	if err := os.MkdirAll(s.DataDir, 0755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	dates, err := s.listDates()
	if err != nil {
		return fmt.Errorf("backfilling gaps: %w", err)
	}

	today := time.Now().Format("2006-01-02")

	if len(dates) == 0 {
		// No existing files; just ensure today's file exists.
		return s.EnsureDateFile(today)
	}

	earliest := dates[0]
	latest := dates[len(dates)-1]

	// Extend to today if today is after the latest file.
	todayTime, _ := time.Parse("2006-01-02", today)
	latestTime, _ := time.Parse("2006-01-02", latest)
	if todayTime.After(latestTime) {
		latest = today
	}

	// Build a set of existing dates for fast lookup.
	existing := make(map[string]bool, len(dates))
	for _, d := range dates {
		existing[d] = true
	}

	// Walk from earliest to latest, creating missing date files.
	start, _ := time.Parse("2006-01-02", earliest)
	end, _ := time.Parse("2006-01-02", latest)

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		if !existing[ds] {
			if err := s.EnsureDateFile(ds); err != nil {
				return fmt.Errorf("backfilling %s: %w", ds, err)
			}
		}
	}

	return nil
}

// CollectAllIDs collects all todo IDs across all files for uniqueness checking.
func (s *Store) CollectAllIDs() (map[string]bool, error) {
	allDays, err := s.ReadAllDays()
	if err != nil {
		return nil, fmt.Errorf("collecting IDs: %w", err)
	}

	ids := make(map[string]bool)
	for _, todos := range allDays {
		for _, todo := range todos {
			ids[todo.ID] = true
		}
	}
	return ids, nil
}

// listDates returns a sorted slice of date strings from existing .md files
// in the data directory.
func (s *Store) listDates() ([]string, error) {
	entries, err := os.ReadDir(s.DataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing dates: %w", err)
	}

	var dates []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		date := strings.TrimSuffix(name, ".md")
		if _, err := time.Parse("2006-01-02", date); err != nil {
			continue
		}
		dates = append(dates, date)
	}

	sort.Strings(dates)
	return dates, nil
}
