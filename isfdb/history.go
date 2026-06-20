// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"sync"
	"unicode/utf8"
)

const historyMax = 10
const historyLabelWidth = 20

// HistoryEntry is one item in the recent-pages list.
type HistoryEntry struct {
	Label string // "Author", "Title", "Pub", "Award"
	Name  string // truncated to historyLabelWidth runes
	URL   string // e.g. "/author.cgi?12345"
}

var (
	historyMu      sync.Mutex
	historyEntries []HistoryEntry
)

// RecordHistory adds a page view to the top of the history list.
// If the URL already appears anywhere in the list it is moved to the top.
// The list is capped at historyMax entries.
func RecordHistory(label, name, url string) {
	entry := HistoryEntry{
		Label: label,
		Name:  truncateRunes(name, historyLabelWidth),
		URL:   url,
	}

	historyMu.Lock()
	defer historyMu.Unlock()

	// Remove any existing entry for the same URL.
	filtered := historyEntries[:0]
	for _, e := range historyEntries {
		if e.URL != url {
			filtered = append(filtered, e)
		}
	}

	// Prepend and cap.
	combined := make([]HistoryEntry, 0, historyMax)
	combined = append(combined, entry)
	combined = append(combined, filtered...)
	if len(combined) > historyMax {
		combined = combined[:historyMax]
	}
	historyEntries = combined
}

// GetHistory returns a snapshot of the current history list.
func GetHistory() []HistoryEntry {
	historyMu.Lock()
	defer historyMu.Unlock()
	snap := make([]HistoryEntry, len(historyEntries))
	copy(snap, historyEntries)
	return snap
}

// truncateRunes truncates s to at most n runes, appending "…" if cut.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	count := 0
	for i := range s {
		if count == n-1 {
			return s[:i] + "…"
		}
		count++
	}
	return s
}
