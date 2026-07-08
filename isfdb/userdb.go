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

// UserDBClose closes the user data database.
func UserDBClose() {
	if UserDB != nil {
		UserDB.Close()
		UserDB = nil
	}
}
