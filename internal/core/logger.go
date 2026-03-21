package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Logger writes state change events to sidecar .log files alongside day .md files.
type Logger struct {
	DataDir string
}

// NewLogger creates a new Logger that writes to the given data directory.
func NewLogger(dataDir string) *Logger {
	return &Logger{DataDir: dataDir}
}

// logPath returns the log file path for a given date (YYYY-MM-DD).
func (l *Logger) logPath(date string) string {
	return filepath.Join(l.DataDir, date+".log")
}

// appendLine appends a single log line to the date's log file.
func (l *Logger) appendLine(date string, line string) error {
	if err := os.MkdirAll(l.DataDir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.logPath(date), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, line)
	return err
}

// timestamp returns the current time formatted for log entries.
func timestamp() string {
	return time.Now().Format("2006-01-02T15:04:05")
}

// LogCreated logs a todo creation event. date is the creation date (YYYY-MM-DD format).
func (l *Logger) LogCreated(date string, todo *Todo) error {
	tags := ""
	if len(todo.Tags) > 0 {
		tags = strings.Join(todo.Tags, ",")
	}
	line := fmt.Sprintf("%s %s created source=%s tags=%s text=%q",
		timestamp(), todo.ID, todo.Source, tags, todo.Text)
	return l.appendLine(date, line)
}

// LogStatusChange logs a status transition.
func (l *Logger) LogStatusChange(date string, id string, from string, to string) error {
	line := fmt.Sprintf("%s %s status %s->%s",
		timestamp(), id, from, to)
	return l.appendLine(date, line)
}

// LogEdit logs a field edit.
func (l *Logger) LogEdit(date string, id string, field string, oldVal string, newVal string) error {
	line := fmt.Sprintf("%s %s edited field=%s old=%q new=%q",
		timestamp(), id, field, oldVal, newVal)
	return l.appendLine(date, line)
}

// LogRevive logs a stale->inbox revive with new stale_count.
func (l *Logger) LogRevive(date string, id string, staleCount int) error {
	line := fmt.Sprintf("%s %s revived stale_count=%d",
		timestamp(), id, staleCount)
	return l.appendLine(date, line)
}

// LogDelete logs a deletion.
func (l *Logger) LogDelete(date string, id string) error {
	line := fmt.Sprintf("%s %s deleted",
		timestamp(), id)
	return l.appendLine(date, line)
}
