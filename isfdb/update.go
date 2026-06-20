// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"bufio"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// metadataFileID is the Google Drive file ID of the JSON metadata file.
const metadataFileID = "1PizblERl79rL_1pjlv-7IXA6MZrb31N9"

// UpdateStatus represents the current state of a database update.
type UpdateStatus int

const (
	UpdateIdle        UpdateStatus = iota
	UpdateChecking                 // fetching metadata JSON
	UpdateDownloading              // downloading the gz file
	UpdateImporting                // running SQL into new DB
	UpdateDone                     // finished successfully
	UpdateError                    // failed
)

// UpdateState holds all mutable update state, protected by a mutex.
type UpdateState struct {
	mu        sync.Mutex
	Status    UpdateStatus
	Message   string // human-readable progress / error text
	Available bool   // true when a newer DB version exists on Drive
	NewDate   string // upload_date from the metadata JSON
	Running   bool   // true while a goroutine is active
}

// Upd is the global update state, accessible from handlers.
var Upd = &UpdateState{}

func (s *UpdateState) set(status UpdateStatus, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
	s.Message = msg
}

// Snapshot returns a consistent read of all fields.
func (s *UpdateState) Snapshot() (status UpdateStatus, msg string, available bool, newDate string, running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Status, s.Message, s.Available, s.NewDate, s.Running
}

// gdriveRe matches the file ID inside a Google Drive sharing URL.
var gdriveRe = regexp.MustCompile(`/file/d/([^/?]+)`)

// gdriveDownloadURL converts any Google Drive URL (sharing or file/d/...)
// to a direct download URL that bypasses the virus-scan confirmation page.
func gdriveDownloadURL(raw string) string {
	if m := gdriveRe.FindStringSubmatch(raw); len(m) > 1 {
		return fmt.Sprintf(
			"https://drive.usercontent.google.com/download?id=%s&export=download&confirm=t",
			m[1])
	}
	// Assume it's already a direct URL.
	return raw
}

func metadataURL() string {
	return fmt.Sprintf(
		"https://drive.usercontent.google.com/download?id=%s&export=download&confirm=t",
		metadataFileID)
}

type dbMetadata struct {
	UploadDate string `json:"upload_date"`
	UploadFile string `json:"upload_file"`
}

func fetchMetadata() (*dbMetadata, error) {
	resp, err := http.Get(metadataURL())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var meta dbMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// CheckForUpdate fetches the metadata JSON and sets Upd.Available if a newer
// version exists. Safe to call from a goroutine at startup.
func CheckForUpdate() {
	Upd.set(UpdateChecking, "Checking for updates…")

	meta, err := fetchMetadata()
	if err != nil {
		log.Printf("update check: %v", err)
		Upd.set(UpdateIdle, "")
		return
	}

	newDate, err := time.Parse("2006-01-02", meta.UploadDate)
	if err != nil {
		log.Printf("update check: bad date %q: %v", meta.UploadDate, err)
		Upd.set(UpdateIdle, "")
		return
	}

	info, err := os.Stat(DBPath)
	if err != nil {
		Upd.set(UpdateIdle, "")
		return
	}

	Upd.mu.Lock()
	Upd.Status = UpdateIdle
	Upd.Message = ""
	if newDate.After(info.ModTime().Truncate(24 * time.Hour)) {
		Upd.Available = true
		Upd.NewDate = meta.UploadDate
	} else {
		Upd.Available = false
		Upd.NewDate = ""
	}
	Upd.mu.Unlock()
}

// StartUpdate launches the update goroutine. Returns false if already running.
func StartUpdate() bool {
	Upd.mu.Lock()
	defer Upd.mu.Unlock()
	if Upd.Running {
		return false
	}
	Upd.Running = true
	go runUpdate()
	return true
}

func runUpdate() {
	defer func() {
		Upd.mu.Lock()
		Upd.Running = false
		Upd.mu.Unlock()
	}()

	// Re-fetch metadata to get the download URL.
	Upd.set(UpdateDownloading, "Fetching metadata…")
	meta, err := fetchMetadata()
	if err != nil {
		Upd.set(UpdateError, fmt.Sprintf("Could not fetch metadata: %v", err))
		return
	}

	downloadURL := gdriveDownloadURL(meta.UploadFile)

	// Download the gzip file.
	Upd.set(UpdateDownloading, "Downloading database (this may take a few minutes)…")
	dlResp, err := http.Get(downloadURL)
	if err != nil {
		Upd.set(UpdateError, fmt.Sprintf("Download failed: %v", err))
		return
	}
	defer dlResp.Body.Close()

	// Decompress on the fly and import into a temp DB file.
	Upd.set(UpdateImporting, "Decompressing and importing (this may take a minute or two)…")
	gr, err := gzip.NewReader(dlResp.Body)
	if err != nil {
		Upd.set(UpdateError, fmt.Sprintf("Decompression failed: %v", err))
		return
	}
	defer gr.Close()

	newDBPath := DBPath + ".new"
	os.Remove(newDBPath) // clean up any previous failed attempt
	if err := importSQL(gr, newDBPath); err != nil {
		os.Remove(newDBPath)
		Upd.set(UpdateError, fmt.Sprintf("Import failed: %v", err))
		return
	}

	// Atomic swap: current → .old, new → current.
	Upd.set(UpdateImporting, "Finalizing…")
	oldPath := DBPath + ".old"
	os.Remove(oldPath)
	if _, statErr := os.Stat(DBPath); statErr == nil {
		// Existing DB present — back it up before replacing.
		if err := os.Rename(DBPath, oldPath); err != nil {
			os.Remove(newDBPath)
			Upd.set(UpdateError, fmt.Sprintf("Could not back up existing database: %v", err))
			return
		}
	}
	if err := os.Rename(newDBPath, DBPath); err != nil {
		os.Rename(oldPath, DBPath) // roll back if backup exists
		Upd.set(UpdateError, fmt.Sprintf("Could not install new database: %v", err))
		return
	}
	os.Remove(oldPath)

	Upd.mu.Lock()
	Upd.Status = UpdateDone
	Upd.Message = fmt.Sprintf("Database updated to %s. Please restart the server to load the new data.", meta.UploadDate)
	Upd.Available = false
	Upd.NewDate = ""
	Upd.mu.Unlock()
}

// importSQL reads a SQLite dump from r and executes it into a fresh database
// at path. Uses performance pragmas to speed up bulk import.
func importSQL(r io.Reader, path string) error {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}
	defer db.Close()

	// Fast-import pragmas: no WAL, no fsync, temp tables in memory.
	for _, p := range []string{
		"PRAGMA journal_mode=OFF",
		"PRAGMA synchronous=OFF",
		"PRAGMA temp_store=MEMORY",
		"PRAGMA cache_size=-65536",
	} {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("pragma: %w", err)
		}
	}

	// SQLite dumps use lines; a statement ends when a line ends with ';'.
	// Use a 4MB scanner buffer to handle large INSERT lines.
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	var stmt strings.Builder
	inString := false
	for scanner.Scan() {
		line := scanner.Text()
		stmt.WriteString(line)
		stmt.WriteByte('\n')

		// Walk the line tracking single-quote string context.
		// SQLite dumps use '' to escape a literal single quote.
		for i := 0; i < len(line); i++ {
			if inString {
				if line[i] == '\'' {
					if i+1 < len(line) && line[i+1] == '\'' {
						i++ // escaped quote, skip both
					} else {
						inString = false
					}
				}
			} else {
				if line[i] == '\'' {
					inString = true
				}
			}
		}

		// Only treat a trailing ';' as a statement terminator when
		// we are not inside a string literal.
		if !inString && strings.HasSuffix(strings.TrimSpace(line), ";") {
			sql := strings.TrimSpace(stmt.String())
			if sql != "" && sql != ";" {
				if _, err := db.Exec(sql); err != nil {
					return fmt.Errorf("executing %.80q: %w", sql, err)
				}
			}
			stmt.Reset()
		}
	}
	return scanner.Err()
}
