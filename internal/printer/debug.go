package printer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LogPrintDebug writes a debug log AFTER a print attempt.
// Includes metadata (mode, device, print result) and the content that was sent.
// For ESC/POS mode, content should be the command script (human-readable).
// For text mode, content is the plain receipt text.
// Label differentiates logs when multiple prints happen in the same second
// (e.g. test name, "cli", etc).
func LogPrintDebug(dataDir string, todoID string, label string, mode string, device string, content string, printErr error) error {
	debugDir := filepath.Join(dataDir, ".debug")
	os.MkdirAll(debugDir, 0755)

	ts := time.Now().Format("2006-01-02T15-04-05")
	status := "ok"
	if printErr != nil {
		status = fmt.Sprintf("FAILED: %v", printErr)
	}

	header := fmt.Sprintf("timestamp: %s\ntodo: %s\nlabel: %s\nmode: %s\ndevice: %s\nprint: %s\n---\n",
		ts, todoID, label, mode, device, status)

	filename := fmt.Sprintf("debug-%s-%s-%s.log", ts, todoID, label)
	return os.WriteFile(filepath.Join(debugDir, filename), []byte(header+content), 0644)
}
