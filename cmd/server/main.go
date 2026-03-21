package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/anujp/braindump/internal/api"
	"github.com/anujp/braindump/internal/core"
)

func main() {
	dataDir := os.Getenv("TODO_DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}

	store := core.NewStore(dataDir)
	logger := core.NewLogger(dataDir)

	tagStore, err := core.NewTagStore(filepath.Join(dataDir, "tags.yaml"))
	if err != nil {
		log.Fatalf("Failed to load tags: %v", err)
	}

	if err := store.BackfillGaps(); err != nil {
		log.Fatalf("Failed to backfill gaps: %v", err)
	}

	// Run stale check at startup
	count, err := core.RunStaleCheck(store, logger)
	if err != nil {
		log.Printf("Warning: stale check failed: %v", err)
	} else if count > 0 {
		log.Printf("Stale check: marked %d items as stale", count)
	}

	// Schedule daily stale check at midnight
	go scheduleDailyStaleCheck(store, logger)

	router := api.NewRouter(store, logger, tagStore)

	fmt.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// scheduleDailyStaleCheck runs the stale check at midnight each day.
func scheduleDailyStaleCheck(store *core.Store, logger *core.Logger) {
	for {
		now := time.Now()
		// Calculate next midnight
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		timer := time.NewTimer(next.Sub(now))
		<-timer.C

		// Also backfill gaps for the new day
		if err := store.BackfillGaps(); err != nil {
			log.Printf("Warning: daily backfill failed: %v", err)
		}

		count, err := core.RunStaleCheck(store, logger)
		if err != nil {
			log.Printf("Warning: daily stale check failed: %v", err)
		} else if count > 0 {
			log.Printf("Daily stale check: marked %d items as stale", count)
		}
	}
}
