// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import "database/sql"

// This file serves as the interface to the database. It defines:
// - The Note struct, which holds data from the notes table
// - SQL functions to access note data

type Note struct {
	NoteID   int
	NoteNote sql.NullString
}

func SQLloadNoteData(db *sql.DB, id int) (*Note, error) {
	var n Note
	row := db.QueryRow("SELECT * FROM notes WHERE note_id=?", id)
	if err := row.Scan(&n.NoteID, &n.NoteNote); err != nil {
		return nil, err
	}
	return &n, nil
}

// SQLgetNotes returns the note text for a given note_id, or an empty string
// if the id is 0 or the note is not found.
func SQLgetNotes(db *sql.DB, noteID int) (string, error) {
	if noteID == 0 {
		return "", nil
	}
	n, err := SQLloadNoteData(db, noteID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return n.NoteNote.String, nil
}
