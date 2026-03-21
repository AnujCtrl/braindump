package core

import (
	"time"
)

const staleDays = 7

// allTodos is a helper that flattens ReadAllDays into a single slice.
func allTodos(store *Store) ([]*Todo, error) {
	dayMap, err := store.ReadAllDays()
	if err != nil {
		return nil, err
	}
	var all []*Todo
	for _, todos := range dayMap {
		all = append(all, todos...)
	}
	return all, nil
}

// FindStaleItems scans all todos and returns those with status "inbox"
// whose created time is more than 7 days ago.
func FindStaleItems(store *Store) ([]*Todo, error) {
	all, err := allTodos(store)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -staleDays)
	var stale []*Todo
	for _, todo := range all {
		if todo.Status == "inbox" && todo.Created.Before(cutoff) {
			stale = append(stale, todo)
		}
	}
	return stale, nil
}

// MarkStale changes a todo's status to "stale".
// Updates the todo in the store and logs the change.
func MarkStale(store *Store, logger *Logger, todo *Todo) error {
	oldStatus := todo.Status
	todo.Status = "stale"

	date := todo.Created.Format("2006-01-02")
	if err := store.UpdateTodo(todo); err != nil {
		return err
	}
	return logger.LogStatusChange(date, todo.ID, oldStatus, "stale")
}

// ReviveTodo changes a stale todo back to "inbox" and increments stale_count.
// Updates the todo in the store and logs the change.
func ReviveTodo(store *Store, logger *Logger, todo *Todo) error {
	todo.Status = "inbox"
	todo.StaleCount++

	date := todo.Created.Format("2006-01-02")
	if err := store.UpdateTodo(todo); err != nil {
		return err
	}
	return logger.LogRevive(date, todo.ID, todo.StaleCount)
}

// FindLoopingItems returns todos with stale_count >= 2 across all files.
func FindLoopingItems(store *Store) ([]*Todo, error) {
	all, err := allTodos(store)
	if err != nil {
		return nil, err
	}

	var looping []*Todo
	for _, todo := range all {
		if todo.StaleCount >= 2 {
			looping = append(looping, todo)
		}
	}
	return looping, nil
}

// RunStaleCheck finds and marks all stale items. Returns count of newly staled items.
func RunStaleCheck(store *Store, logger *Logger) (int, error) {
	staleItems, err := FindStaleItems(store)
	if err != nil {
		return 0, err
	}

	for _, todo := range staleItems {
		if err := MarkStale(store, logger, todo); err != nil {
			return 0, err
		}
	}
	return len(staleItems), nil
}
