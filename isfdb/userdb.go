// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// UserDB is the global connection to the user data database (user_data.db).
var UserDB *sql.DB

// UserDBPath is the filesystem path to the user data database.
var UserDBPath = "./user_data.db"

// userDBSchema defines all tables that must exist in user_data.db.
// Using CREATE TABLE IF NOT EXISTS makes this safe to run on every startup.
var userDBSchema = []string{
	`CREATE TABLE IF NOT EXISTS collection (
		col_id        INTEGER PRIMARY KEY AUTOINCREMENT,
		pub_id        INTEGER NOT NULL UNIQUE,
		col_acq_date  TEXT    NOT NULL DEFAULT '0000-00-00',
		col_sale_date TEXT    NOT NULL DEFAULT '0000-00-00',
		col_cond      TEXT    NOT NULL DEFAULT '',
		col_signature TEXT    NOT NULL DEFAULT 'n',
		col_marginalia TEXT   NOT NULL DEFAULT 'n',
		col_source    TEXT    NOT NULL DEFAULT '',
		col_prch_price TEXT   NOT NULL DEFAULT '',
		col_ins_value  TEXT   NOT NULL DEFAULT '',
		col_location  TEXT    NOT NULL DEFAULT '',
		col_note      TEXT    NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS collection_pub_id ON collection (pub_id)`,
	`CREATE TABLE IF NOT EXISTS user_preferences (
		user_pref_id              INTEGER PRIMARY KEY AUTOINCREMENT,
		concise_disp              INTEGER DEFAULT 0,
		display_all_languages     TEXT    DEFAULT 'All' CHECK(display_all_languages IN ('All','None','Selected')),
		default_language          INTEGER DEFAULT 0,
		covers_display            INTEGER DEFAULT 1,
		cover_links_display       INTEGER DEFAULT 1,
		keep_spaces_in_searches   INTEGER DEFAULT 0,
		suppress_awards           INTEGER DEFAULT 0,
		suppress_reviews          INTEGER DEFAULT 0,
		display_title_translations INTEGER DEFAULT 1,
		dark_mode                 INTEGER DEFAULT 2
	)`,
}

// UserDBOpen opens the user data database, creating the file and schema if
// they do not already exist. Should be called once at server startup.
func UserDBOpen() error {
	db, err := sql.Open("sqlite3", UserDBPath)
	if err != nil {
		return fmt.Errorf("userdb open: %w", err)
	}

	// Apply schema — CREATE TABLE/INDEX IF NOT EXISTS is idempotent.
	for _, stmt := range userDBSchema {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return fmt.Errorf("userdb schema: %w", err)
		}
	}

	UserDB = db
	return nil
}

// UserPrefs holds the single row from the user_preferences table.
// dark_mode: 0=light, 1=dark, 2=system/auto.
type UserPrefs struct {
	ConciseDisp              int
	DisplayAllLanguages      string
	DefaultLanguage          int
	CoversDisplay            bool
	CoverLinksDisplay        bool
	KeepSpacesInSearches     bool
	SuppressAwards           bool
	SuppressReviews          bool
	DisplayTitleTranslations bool
	DarkMode                 int
}

// LoadUserPrefs returns the user's preferences, creating the default row if
// none exists yet.
func LoadUserPrefs() (UserPrefs, error) {
	var p UserPrefs
	err := UserDB.QueryRow(`
		SELECT concise_disp, display_all_languages, default_language,
		       covers_display, cover_links_display, keep_spaces_in_searches,
		       suppress_awards, suppress_reviews, display_title_translations,
		       dark_mode
		FROM user_preferences LIMIT 1`).Scan(
		&p.ConciseDisp, &p.DisplayAllLanguages, &p.DefaultLanguage,
		&p.CoversDisplay, &p.CoverLinksDisplay, &p.KeepSpacesInSearches,
		&p.SuppressAwards, &p.SuppressReviews, &p.DisplayTitleTranslations,
		&p.DarkMode,
	)
	if err == sql.ErrNoRows {
		if _, err = UserDB.Exec(`INSERT INTO user_preferences DEFAULT VALUES`); err != nil {
			return p, fmt.Errorf("user_preferences insert default: %w", err)
		}
		return LoadUserPrefs()
	}
	if err != nil {
		return p, fmt.Errorf("user_preferences load: %w", err)
	}
	return p, nil
}

// UserDBClose closes the user data database.
func UserDBClose() {
	if UserDB != nil {
		UserDB.Close()
		UserDB = nil
	}
}
