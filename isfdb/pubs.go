// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// This file serves as the interface to the database. It defines:
// - The main pub struct, which holds data from the pubs table
// - The various SQL functions use to access pub-related data.

type Pub struct {
	PubID        int
	PubTitle     sql.NullString
	PubTag       sql.NullString
	PubYear      sql.NullString
	PublisherID  sql.NullInt32
	PubPages     sql.NullString
	PubPType     sql.NullString
	PubCType     sql.NullString
	PubISBN      sql.NullString
	PubFrontImage sql.NullString
	PubPrice     sql.NullString
	NoteID       sql.NullInt32
	PubSeriesID  sql.NullInt32
	PubSeriesNum sql.NullString
	PubCatalog   sql.NullString
}

func SQLloadPubData(db *sql.DB, id int) (*Pub, error) {
	var p Pub

	row := db.QueryRow("SELECT * FROM pubs WHERE pub_id = ?", id)
	if err := row.Scan(
		&p.PubID, &p.PubTitle, &p.PubTag,
		&p.PubYear, &p.PublisherID, &p.PubPages,
		&p.PubPType, &p.PubCType, &p.PubISBN,
		&p.PubFrontImage, &p.PubPrice, &p.NoteID,
		&p.PubSeriesID, &p.PubSeriesNum, &p.PubCatalog,
	); err != nil {
		return nil, err
	}

	return &p, nil
}

// SQLGetPubsByPublisherYear returns all pubs for a given publisher in a given
// year (integer), sorted by pub_year then pub_title.
// year=0 matches the "0000-00-00" unknown-date bucket.
func SQLGetPubsByPublisherYear(db *sql.DB, publisherID, year int) ([]*Pub, error) {
	var rows *sql.Rows
	var err error
	if year == 0 {
		rows, err = db.Query(
			"SELECT pub_id, pub_title, pub_tag, pub_year, publisher_id, pub_pages, "+
				"pub_ptype, pub_ctype, pub_isbn, pub_frontimage, pub_price, note_id, "+
				"pub_series_id, pub_series_num, pub_catalog "+
				"FROM pubs WHERE publisher_id=? AND pub_year='0000-00-00' "+
				"ORDER BY pub_year, pub_title",
			publisherID,
		)
	} else {
		rows, err = db.Query(
			"SELECT pub_id, pub_title, pub_tag, pub_year, publisher_id, pub_pages, "+
				"pub_ptype, pub_ctype, pub_isbn, pub_frontimage, pub_price, note_id, "+
				"pub_series_id, pub_series_num, pub_catalog "+
				"FROM pubs WHERE publisher_id=? AND CAST(SUBSTR(pub_year,1,4) AS INTEGER)=? "+
				"ORDER BY pub_year, pub_title",
			publisherID, year,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPubs(rows)
}

// scanPubs is a shared helper for scanning full pubs rows.
func scanPubs(rows *sql.Rows) ([]*Pub, error) {
	var result []*Pub
	for rows.Next() {
		var p Pub
		if err := rows.Scan(
			&p.PubID, &p.PubTitle, &p.PubTag,
			&p.PubYear, &p.PublisherID, &p.PubPages,
			&p.PubPType, &p.PubCType, &p.PubISBN,
			&p.PubFrontImage, &p.PubPrice, &p.NoteID,
			&p.PubSeriesID, &p.PubSeriesNum, &p.PubCatalog,
		); err != nil {
			return nil, err
		}
		result = append(result, &p)
	}
	return result, rows.Err()
}

// SQLGetPubsNotInSeries returns all pubs for a publisher that have no
// pub_series_id, ordered by date (unknown dates last).
// desc=true reverses to most-recent-first.
func SQLGetPubsNotInSeries(db *sql.DB, publisherID int, desc bool) ([]*Pub, error) {
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	q := fmt.Sprintf(
		"SELECT pub_id, pub_title, pub_tag, pub_year, publisher_id, pub_pages, "+
			"pub_ptype, pub_ctype, pub_isbn, pub_frontimage, pub_price, note_id, "+
			"pub_series_id, pub_series_num, pub_catalog "+
			"FROM pubs WHERE publisher_id=? AND pub_series_id IS NULL "+
			"ORDER BY CASE WHEN pub_year='0000-00-00' THEN 1 ELSE 0 END, pub_year %s", dir)
	rows, err := db.Query(q, publisherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPubs(rows)
}

// SQLGetPubsByTitle returns all pubs containing the given title or any of its variants.
func SQLGetPubsByTitle(db *sql.DB, titleID int) ([]*Pub, error) {
	query := fmt.Sprintf(
		"SELECT title_id FROM titles WHERE title_id=%d OR title_parent=%d",
		titleID, titleID,
	)
	return retrievePubsQuery(db, query)
}

// retrievePubsQuery is an internal helper. It executes a query that returns a
// set of title_ids, then fetches all pubs that contain any of those titles.
func retrievePubsQuery(db *sql.DB, titleQuery string) ([]*Pub, error) {
	// Step 1 - collect the list of title_ids
	rows, err := db.Query(titleQuery)
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
		return []*Pub{}, nil
	}

	// Step 2 - get all pub_ids from pub_content in one IN-clause query
	ph := make([]string, len(titleIDs))
	args := make([]any, len(titleIDs))
	for i, id := range titleIDs {
		ph[i] = "?"
		args[i] = id
	}
	pcRows, err := db.Query(
		"SELECT DISTINCT pub_id FROM pub_content WHERE title_id IN ("+strings.Join(ph, ",")+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	var pubIDs []int
	for pcRows.Next() {
		var pubID int
		if err := pcRows.Scan(&pubID); err != nil {
			pcRows.Close()
			return nil, err
		}
		pubIDs = append(pubIDs, pubID)
	}
	pcRows.Close()
	if err := pcRows.Err(); err != nil {
		return nil, err
	}

	if len(pubIDs) == 0 {
		return []*Pub{}, nil
	}

	// Step 3 - fetch all pub records in one IN-clause query
	pph := make([]string, len(pubIDs))
	pargs := make([]any, len(pubIDs))
	for i, id := range pubIDs {
		pph[i] = "?"
		pargs[i] = id
	}
	pubRows, err := db.Query(
		"SELECT pub_id, pub_title, pub_tag, pub_year, publisher_id, pub_pages, pub_ptype, pub_ctype, pub_isbn, pub_frontimage, pub_price, note_id, pub_series_id, pub_series_num, pub_catalog FROM pubs WHERE pub_id IN ("+strings.Join(pph, ",")+")",
		pargs...,
	)
	if err != nil {
		return nil, err
	}
	pubs := make([]*Pub, 0, len(pubIDs))
	for pubRows.Next() {
		var p Pub
		if err := pubRows.Scan(
			&p.PubID, &p.PubTitle, &p.PubTag,
			&p.PubYear, &p.PublisherID, &p.PubPages,
			&p.PubPType, &p.PubCType, &p.PubISBN,
			&p.PubFrontImage, &p.PubPrice, &p.NoteID,
			&p.PubSeriesID, &p.PubSeriesNum, &p.PubCatalog,
		); err != nil {
			pubRows.Close()
			return nil, err
		}
		pubs = append(pubs, &p)
	}
	pubRows.Close()
	if err := pubRows.Err(); err != nil {
		return nil, err
	}

	// Step 4 - sort in Go: unknown years (0000-00-00) last, then by year, title, id.
	sort.Slice(pubs, func(i, j int) bool {
		yi, yj := pubs[i].PubYear.String, pubs[j].PubYear.String
		iZero := yi == "0000-00-00" || yi == ""
		jZero := yj == "0000-00-00" || yj == ""
		if iZero != jZero {
			return jZero // zero years sort last
		}
		if yi != yj {
			return yi < yj
		}
		ti, tj := pubs[i].PubTitle.String, pubs[j].PubTitle.String
		if ti != tj {
			return ti < tj
		}
		return pubs[i].PubID < pubs[j].PubID
	})

	return pubs, nil
}

// SQLGetPubsForSeriesIDs fetches all publications that contain an EDITOR title
// belonging to any of the given series IDs (including variant titles).
// It replaces the N×3-query-per-title loop used by the series grid with 3
// queries total, regardless of how many titles the series contains.
func SQLGetPubsForSeriesIDs(db *sql.DB, seriesIDs []int) ([]*Pub, error) {
	if len(seriesIDs) == 0 {
		return nil, nil
	}

	ph := make([]string, len(seriesIDs))
	args := make([]any, len(seriesIDs))
	for i, id := range seriesIDs {
		ph[i] = "?"
		args[i] = id
	}
	inClause := strings.Join(ph, ",")

	// Step 1 — collect all title IDs (canonical + variants) for these series.
	titleRows, err := db.Query(`
		SELECT t2.title_id
		FROM titles t1
		JOIN titles t2 ON t2.title_id = t1.title_id OR t2.title_parent = t1.title_id
		WHERE t1.series_id IN (`+inClause+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	var titleIDs []int
	seen := map[int]bool{}
	for titleRows.Next() {
		var id int
		if err := titleRows.Scan(&id); err != nil {
			titleRows.Close()
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			titleIDs = append(titleIDs, id)
		}
	}
	titleRows.Close()
	if err := titleRows.Err(); err != nil {
		return nil, err
	}
	if len(titleIDs) == 0 {
		return nil, nil
	}

	// Step 2 — find all pub_ids that contain any of those titles.
	tph := make([]string, len(titleIDs))
	targs := make([]any, len(titleIDs))
	for i, id := range titleIDs {
		tph[i] = "?"
		targs[i] = id
	}
	pcRows, err := db.Query(
		"SELECT DISTINCT pub_id FROM pub_content WHERE title_id IN ("+strings.Join(tph, ",")+")",
		targs...,
	)
	if err != nil {
		return nil, err
	}
	var pubIDs []int
	for pcRows.Next() {
		var id int
		if err := pcRows.Scan(&id); err != nil {
			pcRows.Close()
			return nil, err
		}
		pubIDs = append(pubIDs, id)
	}
	pcRows.Close()
	if err := pcRows.Err(); err != nil {
		return nil, err
	}
	if len(pubIDs) == 0 {
		return nil, nil
	}

	// Step 3 — fetch the pub records.
	pph := make([]string, len(pubIDs))
	pargs := make([]any, len(pubIDs))
	for i, id := range pubIDs {
		pph[i] = "?"
		pargs[i] = id
	}
	pubRows, err := db.Query(
		"SELECT pub_id, pub_title, pub_tag, pub_year, publisher_id, pub_pages, pub_ptype, pub_ctype, pub_isbn, pub_frontimage, pub_price, note_id, pub_series_id, pub_series_num, pub_catalog FROM pubs WHERE pub_id IN ("+strings.Join(pph, ",")+")",
		pargs...,
	)
	if err != nil {
		return nil, err
	}
	defer pubRows.Close()
	return scanPubs(pubRows)
}

// SQLFindPubsByISBN searches for publications by one or more ISBN values.
// The targets slice may contain wildcard patterns (% suffix).
func SQLFindPubsByISBN(db *sql.DB, targets []string) ([]*Pub, error) {
	if len(targets) == 0 {
		return nil, nil
	}
	conditions := make([]string, len(targets))
	args := make([]any, len(targets))
	for i, t := range targets {
		conditions[i] = "pub_isbn LIKE ?"
		args[i] = t
	}
	query := "SELECT pub_id, pub_title, pub_tag, pub_year, publisher_id, pub_pages, " +
		"pub_ptype, pub_ctype, pub_isbn, pub_frontimage, pub_price, note_id, " +
		"pub_series_id, pub_series_num, pub_catalog FROM pubs WHERE (" +
		strings.Join(conditions, " OR ") + ") ORDER BY pub_isbn LIMIT 300"
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPubs(rows)
}

// SQLGetPubsByMonth returns all publications whose pub_year falls within the
// given year/month, using the same date-range logic as Python's SQLGetForthcoming.
func SQLGetPubsByMonth(db *sql.DB, year, month int) ([]*Pub, error) {
	start := fmt.Sprintf("%04d-%02d-00", year, month)
	var end string
	if month == 12 {
		end = fmt.Sprintf("%d-01-00", year+1)
	} else {
		end = fmt.Sprintf("%04d-%02d-00", year, month+1)
	}
	q := "SELECT pub_id, pub_title, pub_tag, pub_year, publisher_id, pub_pages, " +
		"pub_ptype, pub_ctype, pub_isbn, pub_frontimage, pub_price, note_id, " +
		"pub_series_id, pub_series_num, pub_catalog " +
		"FROM pubs WHERE pub_year >= ? AND pub_year < ? ORDER BY pub_year, pub_title"
	rows, err := db.Query(q, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPubs(rows)
}
