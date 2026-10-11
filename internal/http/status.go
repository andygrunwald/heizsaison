package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/andygrunwald/heizsaison/internal/database"
	"github.com/andygrunwald/heizsaison/internal/scheduler"
)

// schedulerStatus reports the three scheduler fields both status responses
// carry. nextScrape is nil when the scheduler has not computed a next run yet,
// which is what keeps it out of the JSON payload.
func schedulerStatus(sched *scheduler.Scheduler) (running bool, lastScrape, nextScrape *time.Time) {
	if sched == nil {
		return false, nil, nil
	}

	if next := sched.NextScrapeAt(); !next.IsZero() {
		nextScrape = &next
	}

	return sched.IsRunning(), sched.LastScrapeAt(), nextScrape
}

// databaseConnected reports whether the database is reachable.
func databaseConnected(db *database.DB) bool {
	return db != nil && db.Ping() == nil
}

// writeJSON writes v as the JSON body of an HTTP response.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
