// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// This file serves as the interface to the database. It defines:
// - The Award, AwardType and AwardCat structs
// - SQL functions to access award-related data

type Award struct {
	AwardID     int
	AwardTitle  sql.NullString
	AwardAuthor sql.NullString
	AwardYear   sql.NullString
	AwardTType  sql.NullString
	AwardAType  sql.NullString
	AwardLevel  sql.NullString
	AwardMovie  sql.NullString
	AwardTypeID sql.NullInt32
	AwardCatID  sql.NullInt32
	AwardNoteID sql.NullInt32
}

type AwardType struct {
	AwardTypeID        int
	AwardTypeCode      sql.NullString
	AwardTypeName      sql.NullString
	AwardTypeWikipedia sql.NullString
	AwardTypeNoteID    sql.NullInt32
	AwardTypeBy        sql.NullString
	AwardTypeFor       sql.NullString
	AwardTypeShortName sql.NullString
	AwardTypePoll      sql.NullString
	AwardTypeNonGenre  sql.NullString
}

type AwardCat struct {
	AwardCatID     int
	AwardCatName   sql.NullString
	AwardCatTypeID sql.NullInt32
	AwardCatOrder  sql.NullInt32
	AwardCatNoteID sql.NullInt32
}

// AwardDisplay holds all resolved data for one award row, ready for rendering.
type AwardDisplay struct {
	AwardID       int
	AwardYear     string
	AwardLevel    string
	AwardNote     string
	AwardMovie    string
	AwardTitle    string
	TitleID       int
	TypeID        int
	TypeName      string
	TypeShortName string
	TypePoll      string
	CatID         int
	CatName       string
	Authors       []AuthorRef // for title-based awards
	AwardAuthors  []string    // for non-title-based awards
	SpecialLevel  string      // populated if level > 70
}

var specialAwards = map[string]string{
	"71": "No Winner -- Insufficient Votes",
	"72": "Not on ballot -- Insufficient Nominations",
	"73": "No Award Given This Year",
	"81": "Withdrawn",
	"82": "Withdrawn -- Nomination Declined",
	"83": "Withdrawn -- Conflict of Interest",
	"84": "Withdrawn -- Official Publication in a Previous Year",
	"85": "Withdrawn -- Ineligible",
	"90": "Finalists",
	"91": "Made First Ballot",
	"92": "Preliminary Nominees",
	"93": "Honorable Mentions",
	"98": "Early Submissions",
	"99": "Nominations Below Cutoff",
}

func SQLGetAwardTypeById(db *sql.DB, id int) (*AwardType, error) {
	var t AwardType
	row := db.QueryRow("SELECT * FROM award_types WHERE award_type_id=?", id)
	if err := row.Scan(
		&t.AwardTypeID, &t.AwardTypeCode, &t.AwardTypeName, &t.AwardTypeWikipedia,
		&t.AwardTypeNoteID, &t.AwardTypeBy, &t.AwardTypeFor, &t.AwardTypeShortName,
		&t.AwardTypePoll, &t.AwardTypeNonGenre,
	); err != nil {
		return nil, err
	}
	return &t, nil
}

func SQLGetAwardCatById(db *sql.DB, id int) (*AwardCat, error) {
	var c AwardCat
	row := db.QueryRow("SELECT * FROM award_cats WHERE award_cat_id=?", id)
	if err := row.Scan(
		&c.AwardCatID, &c.AwardCatName, &c.AwardCatTypeID,
		&c.AwardCatOrder, &c.AwardCatNoteID,
	); err != nil {
		return nil, err
	}
	return &c, nil
}

func SQLloadTitleFromAward(db *sql.DB, awardID int) (*Title, error) {
	var titleID int
	row := db.QueryRow("SELECT title_id FROM title_awards WHERE award_id=?", awardID)
	if err := row.Scan(&titleID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return SQLloadTitleData(db, titleID)
}

func SQLgetAuthorByName(db *sql.DB, name string) (*Author, error) {
	var a Author
	row := db.QueryRow("SELECT * FROM authors WHERE author_canonical=?", name)
	if err := row.Scan(
		&a.AuthorID, &a.AuthorCanonical, &a.AuthorLegalName,
		&a.AuthorBirthPlace, &a.AuthorBirthDate, &a.AuthorDeathDate,
		&a.NoteID, &a.AuthorWikipedia, &a.AuthorViews, &a.AuthorIMDB,
		&a.AuthorMarque, &a.AuthorImage, &a.AuthorAnnualViews,
		&a.AuthorLastName, &a.AuthorLanguage, &a.AuthorNote,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// LoadAwardDisplayBatch resolves all fields for a list of awards using batch queries.
func LoadAwardDisplayBatch(db *sql.DB, awards []*Award) ([]*AwardDisplay, error) {
	if len(awards) == 0 {
		return nil, nil
	}

	ph := func(n int) string {
		s := make([]string, n)
		for i := range s {
			s[i] = "?"
		}
		return strings.Join(s, ",")
	}
	toAny := func(ids []int) []any {
		a := make([]any, len(ids))
		for i, id := range ids {
			a[i] = id
		}
		return a
	}

	// Collect unique IDs needed
	awardIDs := make([]int, len(awards))
	typeIDset := make(map[int]struct{})
	catIDset := make(map[int]struct{})
	noteIDset := make(map[int]struct{})
	for i, a := range awards {
		awardIDs[i] = a.AwardID
		if a.AwardTypeID.Valid {
			typeIDset[int(a.AwardTypeID.Int32)] = struct{}{}
		}
		if a.AwardCatID.Valid {
			catIDset[int(a.AwardCatID.Int32)] = struct{}{}
		}
		if a.AwardNoteID.Valid {
			noteIDset[int(a.AwardNoteID.Int32)] = struct{}{}
		}
	}

	// Batch fetch award types
	typeMap := make(map[int]*AwardType)
	if len(typeIDset) > 0 {
		typeIDs := make([]int, 0, len(typeIDset))
		for id := range typeIDset {
			typeIDs = append(typeIDs, id)
		}
		rows, err := db.Query(
			"SELECT award_type_id, award_type_name, award_type_short_name, award_type_poll FROM award_types WHERE award_type_id IN ("+ph(len(typeIDs))+")",
			toAny(typeIDs)...,
		)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var t AwardType
			if err := rows.Scan(&t.AwardTypeID, &t.AwardTypeName, &t.AwardTypeShortName, &t.AwardTypePoll); err != nil {
				rows.Close()
				return nil, err
			}
			typeMap[t.AwardTypeID] = &t
		}
		rows.Close()
	}

	// Batch fetch award categories
	catMap := make(map[int]*AwardCat)
	if len(catIDset) > 0 {
		catIDs := make([]int, 0, len(catIDset))
		for id := range catIDset {
			catIDs = append(catIDs, id)
		}
		rows, err := db.Query(
			"SELECT award_cat_id, award_cat_name FROM award_cats WHERE award_cat_id IN ("+ph(len(catIDs))+")",
			toAny(catIDs)...,
		)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var c AwardCat
			if err := rows.Scan(&c.AwardCatID, &c.AwardCatName); err != nil {
				rows.Close()
				return nil, err
			}
			catMap[c.AwardCatID] = &c
		}
		rows.Close()
	}

	// Batch fetch notes
	noteMap := make(map[int]string)
	if len(noteIDset) > 0 {
		noteIDs := make([]int, 0, len(noteIDset))
		for id := range noteIDset {
			noteIDs = append(noteIDs, id)
		}
		rows, err := db.Query(
			"SELECT note_id, note_note FROM notes WHERE note_id IN ("+ph(len(noteIDs))+")",
			toAny(noteIDs)...,
		)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int
			var note sql.NullString
			if err := rows.Scan(&id, &note); err != nil {
				rows.Close()
				return nil, err
			}
			noteMap[id] = note.String
		}
		rows.Close()
	}

	// Batch fetch title_ids from title_awards
	awardTitleMap := make(map[int]int) // award_id -> title_id
	rows, err := db.Query(
		"SELECT award_id, title_id FROM title_awards WHERE award_id IN ("+ph(len(awardIDs))+")",
		toAny(awardIDs)...,
	)
	if err != nil {
		return nil, err
	}
	titleIDset := make(map[int]struct{})
	for rows.Next() {
		var awardID, titleID int
		if err := rows.Scan(&awardID, &titleID); err != nil {
			rows.Close()
			return nil, err
		}
		awardTitleMap[awardID] = titleID
		titleIDset[titleID] = struct{}{}
	}
	rows.Close()

	// Batch fetch title names
	titleNameMap := make(map[int]string)
	if len(titleIDset) > 0 {
		titleIDs := make([]int, 0, len(titleIDset))
		for id := range titleIDset {
			titleIDs = append(titleIDs, id)
		}
		trows, err := db.Query(
			"SELECT title_id, title_title FROM titles WHERE title_id IN ("+ph(len(titleIDs))+")",
			toAny(titleIDs)...,
		)
		if err != nil {
			return nil, err
		}
		for trows.Next() {
			var id int
			var title sql.NullString
			if err := trows.Scan(&id, &title); err != nil {
				trows.Close()
				return nil, err
			}
			titleNameMap[id] = title.String
		}
		trows.Close()

		// Batch fetch authors for all titles
		// (reuse SQLTitleAuthorsBatch)
	}

	// Batch fetch authors for all title IDs
	titleAuthorMap, err := SQLTitleAuthorsBatch(db, func() []int {
		ids := make([]int, 0, len(titleIDset))
		for id := range titleIDset {
			ids = append(ids, id)
		}
		return ids
	}())
	if err != nil {
		return nil, err
	}

	// Assemble AwardDisplay records
	displays := make([]*AwardDisplay, 0, len(awards))
	for _, a := range awards {
		d := &AwardDisplay{
			AwardID:    a.AwardID,
			AwardYear:  a.AwardYear.String,
			AwardLevel: a.AwardLevel.String,
			AwardTitle: a.AwardTitle.String,
			AwardMovie: a.AwardMovie.String,
		}
		if msg, ok := specialAwards[d.AwardLevel]; ok {
			d.SpecialLevel = msg
		}
		if a.AwardTypeID.Valid {
			if at, ok := typeMap[int(a.AwardTypeID.Int32)]; ok {
				d.TypeID = at.AwardTypeID
				d.TypeName = at.AwardTypeName.String
				d.TypeShortName = at.AwardTypeShortName.String
				d.TypePoll = at.AwardTypePoll.String
			}
		}
		if a.AwardCatID.Valid {
			if ac, ok := catMap[int(a.AwardCatID.Int32)]; ok {
				d.CatID = ac.AwardCatID
				d.CatName = ac.AwardCatName.String
			}
		}
		if a.AwardNoteID.Valid {
			d.AwardNote = noteMap[int(a.AwardNoteID.Int32)]
		}
		if titleID, ok := awardTitleMap[a.AwardID]; ok {
			d.TitleID = titleID
			d.Authors = titleAuthorMap[titleID]
		} else if a.AwardAuthor.Valid && a.AwardAuthor.String != "" {
			d.AwardAuthors = strings.Split(a.AwardAuthor.String, "+")
		}
		displays = append(displays, d)
	}
	return displays, nil
}

// LoadAwardDisplay resolves all fields for a single award into an AwardDisplay struct.
func LoadAwardDisplay(db *sql.DB, a *Award) (*AwardDisplay, error) {
	d := &AwardDisplay{
		AwardID:    a.AwardID,
		AwardYear:  a.AwardYear.String,
		AwardLevel: a.AwardLevel.String,
		AwardTitle: a.AwardTitle.String,
		AwardMovie: a.AwardMovie.String,
	}

	// Resolve special level label
	if msg, ok := specialAwards[d.AwardLevel]; ok {
		d.SpecialLevel = msg
	}

	// Resolve award type
	if a.AwardTypeID.Valid {
		at, err := SQLGetAwardTypeById(db, int(a.AwardTypeID.Int32))
		if err == nil {
			d.TypeID = at.AwardTypeID
			d.TypeName = at.AwardTypeName.String
			d.TypeShortName = at.AwardTypeShortName.String
			d.TypePoll = at.AwardTypePoll.String
		}
	}

	// Resolve award category
	if a.AwardCatID.Valid {
		ac, err := SQLGetAwardCatById(db, int(a.AwardCatID.Int32))
		if err == nil {
			d.CatID = ac.AwardCatID
			d.CatName = ac.AwardCatName.String
		}
	}

	// Resolve note
	if a.AwardNoteID.Valid {
		note, err := SQLgetNotes(db, int(a.AwardNoteID.Int32))
		if err == nil {
			d.AwardNote = note
		}
	}

	// Resolve title and authors
	title, err := SQLloadTitleFromAward(db, a.AwardID)
	if err == nil && title != nil {
		d.TitleID = title.TitleID
		authors, err := SQLTitleBriefAuthorRecords(db, title.TitleID)
		if err == nil {
			d.Authors = authors
		}
	} else if a.AwardAuthor.Valid && a.AwardAuthor.String != "" {
		// Non-title-based award: split authors on '+'
		d.AwardAuthors = strings.Split(a.AwardAuthor.String, "+")
	}

	return d, nil
}

func SQLloadAwardData(db *sql.DB, id int) (*Award, error) {
	var a Award
	row := db.QueryRow("SELECT * FROM awards WHERE award_id=?", id)
	if err := row.Scan(
		&a.AwardID, &a.AwardTitle, &a.AwardAuthor, &a.AwardYear,
		&a.AwardTType, &a.AwardAType, &a.AwardLevel, &a.AwardMovie,
		&a.AwardTypeID, &a.AwardCatID, &a.AwardNoteID,
	); err != nil {
		return nil, err
	}
	return &a, nil
}

// SQLTitleAwards returns all awards for a given title, including awards
// for any variant titles. Results are ordered by year then award level.
func SQLTitleAwards(db *sql.DB, titleID int) ([]*Award, error) {
	if titleID == 0 {
		return []*Award{}, nil
	}

	// Step 1 - collect title_id and all variant title_ids
	rows, err := db.Query("SELECT DISTINCT title_id FROM titles WHERE title_id=? OR title_parent=?", titleID, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var titleIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		titleIDs = append(titleIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(titleIDs) == 0 {
		return []*Award{}, nil
	}

	// Step 2 - get distinct award_ids from title_awards for all collected title_ids
	awardIDset := make(map[int]struct{})
	for _, tid := range titleIDs {
		taRows, err := db.Query("SELECT award_id FROM title_awards WHERE title_id=?", tid)
		if err != nil {
			return nil, err
		}
		for taRows.Next() {
			var aid int
			if err := taRows.Scan(&aid); err != nil {
				taRows.Close()
				return nil, err
			}
			awardIDset[aid] = struct{}{}
		}
		taRows.Close()
		if err := taRows.Err(); err != nil {
			return nil, err
		}
	}

	if len(awardIDset) == 0 {
		return []*Award{}, nil
	}

	// Step 3 - fetch each award record
	awards := make([]*Award, 0, len(awardIDset))
	for aid := range awardIDset {
		a, err := SQLloadAwardData(db, aid)
		if err != nil {
			return nil, err
		}
		awards = append(awards, a)
	}

	// Step 4 - sort by year then award level, matching the original ORDER BY
	sortAwards(awards)

	return awards, nil
}

// SQLloadAwardsForAuthor returns all awards relevant to an author, matching
// Python's SQLloadAwardsXBA.  It runs two queries and merges the results:
//
//  1. Untitled awards (no title_awards row) whose award_author matches the
//     canonical author name or any pseudonym name, using exact and "+" delimited
//     substring patterns.
//  2. Title-based awards whose title_awards.title_id is in titleIDs.
//
// Results are deduplicated by award_id and sorted by (year, title, abs(level)).
// For title-based awards AwardTitle is set to the linked title's title_title
// (matching the Python "t.title_title as title" column) so sorting is correct.
func SQLloadAwardsForAuthor(db *sql.DB, authorName string, pseudoNames []string, titleIDs []int) ([]*Award, error) {
	seen := make(map[int]*Award)

	// ── Part 1: untitled (non-title-linked) name-matched awards ──────────
	names := append([]string{authorName}, pseudoNames...)
	var condParts []string
	var args []any
	for _, name := range names {
		condParts = append(condParts,
			"(a.award_author = ? OR a.award_author LIKE ? OR a.award_author LIKE ? OR a.award_author LIKE ?)")
		args = append(args, name, name+"+%", "%+"+name, "%+"+name+"+%")
	}
	q1 := "SELECT a.award_id, a.award_title, a.award_author, a.award_year, a.award_ttype, " +
		"a.award_atype, a.award_level, a.award_movie, a.award_type_id, a.award_cat_id, a.award_note_id " +
		"FROM awards a " +
		"WHERE NOT EXISTS (SELECT 1 FROM title_awards ta WHERE ta.award_id = a.award_id) " +
		"AND (" + strings.Join(condParts, " OR ") + ")"
	rows1, err := db.Query(q1, args...)
	if err != nil {
		return nil, err
	}
	for rows1.Next() {
		var a Award
		if err := rows1.Scan(
			&a.AwardID, &a.AwardTitle, &a.AwardAuthor, &a.AwardYear,
			&a.AwardTType, &a.AwardAType, &a.AwardLevel, &a.AwardMovie,
			&a.AwardTypeID, &a.AwardCatID, &a.AwardNoteID,
		); err != nil {
			rows1.Close()
			return nil, err
		}
		seen[a.AwardID] = &a
	}
	rows1.Close()
	if err := rows1.Err(); err != nil {
		return nil, err
	}

	// ── Part 2: title-linked awards for this author's canonical titles ────
	if len(titleIDs) > 0 {
		ph := make([]string, len(titleIDs))
		targs := make([]any, len(titleIDs))
		for i, id := range titleIDs {
			ph[i] = "?"
			targs[i] = id
		}
		// Select t.title_title into the AwardTitle slot so that the sort
		// key matches Python's "t.title_title as title" ORDER BY column.
		q2 := "SELECT a.award_id, t.title_title, a.award_author, a.award_year, a.award_ttype, " +
			"a.award_atype, a.award_level, a.award_movie, a.award_type_id, a.award_cat_id, a.award_note_id " +
			"FROM awards a " +
			"JOIN title_awards ta ON ta.award_id = a.award_id " +
			"JOIN titles t ON t.title_id = ta.title_id " +
			"WHERE ta.title_id IN (" + strings.Join(ph, ",") + ")"
		rows2, err := db.Query(q2, targs...)
		if err != nil {
			return nil, err
		}
		for rows2.Next() {
			var a Award
			if err := rows2.Scan(
				&a.AwardID, &a.AwardTitle, &a.AwardAuthor, &a.AwardYear,
				&a.AwardTType, &a.AwardAType, &a.AwardLevel, &a.AwardMovie,
				&a.AwardTypeID, &a.AwardCatID, &a.AwardNoteID,
			); err != nil {
				rows2.Close()
				return nil, err
			}
			if _, dup := seen[a.AwardID]; !dup {
				seen[a.AwardID] = &a
			}
		}
		rows2.Close()
		if err := rows2.Err(); err != nil {
			return nil, err
		}
	}

	// ── Merge, deduplicate, and sort ─────────────────────────────────────
	result := make([]*Award, 0, len(seen))
	for _, a := range seen {
		result = append(result, a)
	}
	sortAwardsForAuthor(result)
	return result, nil
}

// sortAwardsForAuthor sorts by (year, title, abs(level)) matching Python's
// ORDER BY year, title, ABS(level).
func sortAwardsForAuthor(awards []*Award) {
	for i := 1; i < len(awards); i++ {
		for j := i; j > 0; j-- {
			a, b := awards[j-1], awards[j]
			ay, by := a.AwardYear.String, b.AwardYear.String
			at, bt := a.AwardTitle.String, b.AwardTitle.String
			al, _ := strconv.Atoi(a.AwardLevel.String)
			bl, _ := strconv.Atoi(b.AwardLevel.String)
			if al < 0 {
				al = -al
			}
			if bl < 0 {
				bl = -bl
			}
			less := ay < by ||
				(ay == by && at < bt) ||
				(ay == by && at == bt && al < bl)
			if !less {
				awards[j-1], awards[j] = awards[j], awards[j-1]
			} else {
				break
			}
		}
	}
}

// SQLloadAwardsForCatYear loads all awards for one category and year, ordered
// by level then title.  Uses substr() for year matching (award_year values
// like "1956-00-00" are not valid ISO dates so strftime returns NULL).
func SQLloadAwardsForCatYear(db *sql.DB, catID, year int) ([]*Award, error) {
	const q = `
	SELECT * FROM (
	  SELECT a.award_id, a.award_title, a.award_author, a.award_year,
	         a.award_ttype, a.award_atype, a.award_level, a.award_movie,
	         a.award_type_id, a.award_cat_id, a.award_note_id
	  FROM awards a
	  WHERE a.award_cat_id = ?
	    AND substr(a.award_year, 1, 4) = ?
	    AND NOT EXISTS (SELECT 1 FROM title_awards ta WHERE ta.award_id = a.award_id)

	  UNION

	  SELECT a.award_id, t.title_title, a.award_author, a.award_year,
	         a.award_ttype, a.award_atype, a.award_level, a.award_movie,
	         a.award_type_id, a.award_cat_id, a.award_note_id
	  FROM awards a
	       JOIN title_awards ta ON ta.award_id = a.award_id
	       JOIN titles t        ON t.title_id  = ta.title_id
	  WHERE a.award_cat_id = ?
	    AND substr(a.award_year, 1, 4) = ?
	)
	ORDER BY CAST(award_level AS INTEGER), award_title
	`
	yearStr := strconv.Itoa(year)
	rows, err := db.Query(q, catID, yearStr, catID, yearStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*Award
	for rows.Next() {
		var a Award
		if err := rows.Scan(
			&a.AwardID, &a.AwardTitle, &a.AwardAuthor, &a.AwardYear,
			&a.AwardTType, &a.AwardAType, &a.AwardLevel, &a.AwardMovie,
			&a.AwardTypeID, &a.AwardCatID, &a.AwardNoteID,
		); err != nil {
			return nil, err
		}
		result = append(result, &a)
	}
	return result, rows.Err()
}

// SQLloadAwardsForCat loads all awards for one category, optionally limited to
// wins (winNom=0) or all awards (winNom=1).  Returns awards ordered by year,
// level, title — the caller groups them by year.
func SQLloadAwardsForCat(db *sql.DB, catID, winNom int) ([]*Award, error) {
	winsOnly := winNom == 0
	winsClause := ""
	if winsOnly {
		winsClause = "AND a.award_level = '1'"
	}

	q := fmt.Sprintf(`
	SELECT * FROM (
	  SELECT a.award_id, a.award_title, a.award_author, a.award_year,
	         a.award_ttype, a.award_atype, a.award_level, a.award_movie,
	         a.award_type_id, a.award_cat_id, a.award_note_id
	  FROM awards a
	  WHERE a.award_cat_id = ?
	    AND NOT EXISTS (SELECT 1 FROM title_awards ta WHERE ta.award_id = a.award_id)
	    %s

	  UNION

	  SELECT a.award_id, t.title_title, a.award_author, a.award_year,
	         a.award_ttype, a.award_atype, a.award_level, a.award_movie,
	         a.award_type_id, a.award_cat_id, a.award_note_id
	  FROM awards a
	       JOIN title_awards ta ON ta.award_id = a.award_id
	       JOIN titles t        ON t.title_id  = ta.title_id
	  WHERE a.award_cat_id = ?
	    %s
	)
	ORDER BY award_year, CAST(award_level AS INTEGER), award_title
	`, winsClause, winsClause)

	rows, err := db.Query(q, catID, catID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*Award
	for rows.Next() {
		var a Award
		if err := rows.Scan(
			&a.AwardID, &a.AwardTitle, &a.AwardAuthor, &a.AwardYear,
			&a.AwardTType, &a.AwardAType, &a.AwardLevel, &a.AwardMovie,
			&a.AwardTypeID, &a.AwardCatID, &a.AwardNoteID,
		); err != nil {
			return nil, err
		}
		result = append(result, &a)
	}
	return result, rows.Err()
}

// AwardCatBreakdown holds one row from SQLGetAwardCatBreakdown.
type AwardCatBreakdown struct {
	CatName  string
	CatID    int
	CatOrder sql.NullInt32
	Wins     int
	Total    int
}

// SQLGetAwardCatBreakdown returns categories that have at least one award,
// with win and total counts, sorted by display order (NULLs last) then name.
func SQLGetAwardCatBreakdown(db *sql.DB, typeID int) ([]*AwardCatBreakdown, error) {
	const q = `
	SELECT c.award_cat_name, a.award_cat_id, c.award_cat_order,
	       SUM(CASE WHEN a.award_level = '1' THEN 1 ELSE 0 END),
	       COUNT(a.award_id)
	FROM awards a
	JOIN award_cats c ON a.award_cat_id = c.award_cat_id
	WHERE a.award_type_id = ?
	GROUP BY a.award_cat_id
	ORDER BY CASE WHEN c.award_cat_order IS NULL THEN 1 ELSE 0 END,
	         c.award_cat_order, c.award_cat_name`
	rows, err := db.Query(q, typeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*AwardCatBreakdown
	for rows.Next() {
		var b AwardCatBreakdown
		if err := rows.Scan(&b.CatName, &b.CatID, &b.CatOrder, &b.Wins, &b.Total); err != nil {
			return nil, err
		}
		result = append(result, &b)
	}
	return result, rows.Err()
}

// SQLGetEmptyAwardCategories returns categories for a type that have no awards.
func SQLGetEmptyAwardCategories(db *sql.DB, typeID int) ([]*AwardCat, error) {
	const q = `
	SELECT award_cat_id, award_cat_name, award_cat_type_id,
	       award_cat_order, award_cat_note_id
	FROM award_cats
	WHERE award_cat_type_id = ?
	  AND NOT EXISTS (SELECT 1 FROM awards WHERE award_cat_id = award_cats.award_cat_id)
	ORDER BY CASE WHEN award_cat_order IS NULL THEN 1 ELSE 0 END,
	         award_cat_order, award_cat_name`
	rows, err := db.Query(q, typeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*AwardCat
	for rows.Next() {
		var c AwardCat
		if err := rows.Scan(&c.AwardCatID, &c.AwardCatName, &c.AwardCatTypeID,
			&c.AwardCatOrder, &c.AwardCatNoteID); err != nil {
			return nil, err
		}
		result = append(result, &c)
	}
	return result, rows.Err()
}

// SQLGetAwardTypeByCode loads an AwardType by its 2-character code.
func SQLGetAwardTypeByCode(db *sql.DB, code string) (*AwardType, error) {
	var t AwardType
	row := db.QueryRow("SELECT * FROM award_types WHERE award_type_code=?", code)
	if err := row.Scan(
		&t.AwardTypeID, &t.AwardTypeCode, &t.AwardTypeName, &t.AwardTypeWikipedia,
		&t.AwardTypeNoteID, &t.AwardTypeBy, &t.AwardTypeFor, &t.AwardTypeShortName,
		&t.AwardTypePoll, &t.AwardTypeNonGenre,
	); err != nil {
		return nil, err
	}
	return &t, nil
}

// SQLGetAwardYears returns distinct years (as "YYYY-MM-DD" strings) for an
// award type, in ascending order.
func SQLGetAwardYears(db *sql.DB, typeID int) ([]string, error) {
	rows, err := db.Query(
		`SELECT DISTINCT award_year AS y
		 FROM awards WHERE award_type_id=? AND award_year IS NOT NULL ORDER BY y`, typeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var years []string
	for rows.Next() {
		var y string
		if err := rows.Scan(&y); err != nil {
			return nil, err
		}
		years = append(years, y)
	}
	return years, rows.Err()
}

// AwardYearRow is one row from SQLloadAwardsForYearType — it carries the full
// Award plus the category name and order needed for grouping and display.
type AwardYearRow struct {
	Award    *Award
	CatName  string
	CatOrder sql.NullInt32
}

// SQLloadAwardsForYearType loads all awards for a given award type and year,
// sorted by category order (NULLs last), category name, abs(level), title.
// The result mirrors Python's SQLloadAwardsForYearType UNION query.
func SQLloadAwardsForYearType(db *sql.DB, typeID, year int) ([]*AwardYearRow, error) {
	// Use substr(award_year,1,4) for year matching: award_year values like
	// "1956-00-00" are not valid ISO dates so strftime() returns NULL for them.
	// SQLite forbids expressions in ORDER BY of a compound SELECT (UNION),
	// so the UNION is wrapped in a subquery.
	const q = `
	SELECT * FROM (
	  SELECT a.award_id,
	         a.award_title  AS atitle,
	         a.award_author,
	         a.award_year   AS ayear,
	         a.award_ttype, a.award_atype,
	         a.award_level  AS alvl,
	         a.award_movie,
	         a.award_type_id, a.award_cat_id, a.award_note_id,
	         c.award_cat_name   AS cname,
	         c.award_cat_order  AS corder
	  FROM awards a JOIN award_cats c ON a.award_cat_id = c.award_cat_id
	  WHERE a.award_type_id = ?
	    AND substr(a.award_year, 1, 4) = ?
	    AND NOT EXISTS (SELECT 1 FROM title_awards ta WHERE ta.award_id = a.award_id)

	  UNION

	  SELECT a.award_id,
	         t.title_title  AS atitle,
	         a.award_author,
	         a.award_year   AS ayear,
	         a.award_ttype, a.award_atype,
	         a.award_level  AS alvl,
	         a.award_movie,
	         a.award_type_id, a.award_cat_id, a.award_note_id,
	         c.award_cat_name   AS cname,
	         c.award_cat_order  AS corder
	  FROM awards a
	       JOIN award_cats c    ON a.award_cat_id = c.award_cat_id
	       JOIN title_awards ta ON ta.award_id    = a.award_id
	       JOIN titles t        ON t.title_id     = ta.title_id
	  WHERE a.award_type_id = ?
	    AND substr(a.award_year, 1, 4) = ?
	)
	ORDER BY CASE WHEN corder IS NULL THEN 1 ELSE 0 END,
	         corder, cname,
	         ABS(CAST(alvl AS INTEGER)), atitle
	`
	yearStr := strconv.Itoa(year)
	rows, err := db.Query(q, typeID, yearStr, typeID, yearStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*AwardYearRow
	for rows.Next() {
		var a Award
		var catName string
		var catOrder sql.NullInt32
		if err := rows.Scan(
			&a.AwardID, &a.AwardTitle, &a.AwardAuthor, &a.AwardYear,
			&a.AwardTType, &a.AwardAType, &a.AwardLevel, &a.AwardMovie,
			&a.AwardTypeID, &a.AwardCatID, &a.AwardNoteID,
			&catName, &catOrder,
		); err != nil {
			return nil, err
		}
		result = append(result, &AwardYearRow{Award: &a, CatName: catName, CatOrder: catOrder})
	}
	return result, rows.Err()
}

func sortAwards(awards []*Award) {
	n := len(awards)
	for i := 1; i < n; i++ {
		for j := i; j > 0; j-- {
			yi := awards[j-1].AwardYear.String
			yj := awards[j].AwardYear.String
			if yi > yj || (yi == yj && awards[j-1].AwardLevel.String > awards[j].AwardLevel.String) {
				awards[j-1], awards[j] = awards[j], awards[j-1]
			} else {
				break
			}
		}
	}
}

// SQLSearchAwardTypes searches award_types by short name or full name.
func SQLSearchAwardTypes(db *sql.DB, target string) ([]*AwardType, error) {
	like := "%" + target + "%"
	rows, err := db.Query(
		`SELECT award_type_id, award_type_code, award_type_name, award_type_wikipedia,
		        award_type_note_id, award_type_by, award_type_for, award_type_short_name,
		        award_type_poll, award_type_non_genre
		 FROM award_types
		 WHERE award_type_name LIKE ? OR award_type_short_name LIKE ?
		 ORDER BY award_type_short_name`,
		like, like,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*AwardType
	for rows.Next() {
		var at AwardType
		if err := rows.Scan(
			&at.AwardTypeID, &at.AwardTypeCode, &at.AwardTypeName, &at.AwardTypeWikipedia,
			&at.AwardTypeNoteID, &at.AwardTypeBy, &at.AwardTypeFor, &at.AwardTypeShortName,
			&at.AwardTypePoll, &at.AwardTypeNonGenre,
		); err != nil {
			return nil, err
		}
		result = append(result, &at)
	}
	return result, rows.Err()
}
