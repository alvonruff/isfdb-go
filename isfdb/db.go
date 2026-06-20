// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// DBPath is the path to the SQLite database file.
// Set this at startup before calling DBopen.
var DBPath = "./isfdb.db"

// DB is the shared database connection, opened once at startup.
var DB *sql.DB

// Languages is a map of lang_id to lang_name, loaded once at startup.
var Languages map[int]string

// DBopen opens the shared database connection using DBPath and loads
// supporting data (languages, templates) into memory.
// Call this once from main() before starting the server.
func DBopen() error {
	var err error
	DB, err = SQLconnect(DBPath)
	if err != nil {
		return err
	}
	Languages, err = loadLanguages(DB)
	if err != nil {
		return err
	}
	Templates, err = loadTemplates(DB)
	if err != nil {
		return err
	}
	isbnRanges, err = loadISBNRanges(DB)
	return err
}

// loadLanguages loads all languages from the database into a map keyed by lang_id.
func loadLanguages(db *sql.DB) (map[int]string, error) {
	rows, err := db.Query("SELECT lang_id, lang_name FROM languages ORDER BY lang_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	langs := make(map[int]string)
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		langs[id] = name
	}
	return langs, rows.Err()
}

// DBclose closes the shared database connection.
func DBclose() {
	if DB != nil {
		DB.Close()
	}
}

// ParseID extracts a record ID from a request.
// Supports both bare query (?112750) and keyed query (?id=112750).
func ParseID(r *http.Request) (int, error) {
	raw := r.URL.RawQuery
	if raw == "" {
		return 0, fmt.Errorf("no ID provided")
	}
	id, err := strconv.Atoi(raw)
	if err != nil {
		id, err = strconv.Atoi(r.URL.Query().Get("id"))
		if err != nil {
			return 0, fmt.Errorf("invalid ID %q", raw)
		}
	}
	return id, nil
}

// ParseRawParams splits the raw query string on '+' and returns the parts.
// This matches the Python SESSION.Parameter convention where URL parameters
// are separated by '+', e.g. /publisheryear.cgi?123+1985+1 → ["123","1985","1"].
func ParseRawParams(r *http.Request) []string {
	raw := r.URL.RawQuery
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "+")
}

func SQLconnect(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	// Performance pragmas
	pragmas := []string{
		"PRAGMA journal_mode=WAL",       // better read concurrency
		"PRAGMA cache_size=-65536",      // 64MB page cache
		"PRAGMA temp_store=MEMORY",      // temp tables in memory
		"PRAGMA mmap_size=268435456",    // 256MB memory-mapped I/O
		"PRAGMA synchronous=NORMAL",     // safe but faster than FULL
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	return db, nil
}
