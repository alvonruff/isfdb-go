// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"strings"
)

// maxPubsNotInSeriesDisplay is the threshold above which we show a count only
// (no link to a full listing page), matching Python's
// SESSION.max_displayable_pubs_without_pub_series = 500.
const maxPubsNotInSeriesDisplay = 500

// This file serves as the interface to the database. It defines:
// - The main Publisher struct, which holds data from the publishers table
// - The various SQL functions used to access publisher-related data.

type Publisher struct {
	PublisherID        int
	PublisherName      sql.NullString
	PublisherWikipedia sql.NullString
	NoteID             sql.NullInt32
}

func SQLloadPublisherData(db *sql.DB, id int) (*Publisher, error) {
	var p Publisher

	row := db.QueryRow("SELECT * FROM publishers WHERE publisher_id=?", id)
	if err := row.Scan(
		&p.PublisherID, &p.PublisherName, &p.PublisherWikipedia, &p.NoteID,
	); err != nil {
		return nil, err
	}

	return &p, nil
}

// SQLgetPublisherName returns the publisher name for a given publisher ID,
// or an empty string if not found.
func SQLgetPublisherName(db *sql.DB, id int) (string, error) {
	var name sql.NullString
	row := db.QueryRow("SELECT publisher_name FROM publishers WHERE publisher_id=?", id)
	if err := row.Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return name.String, nil
}

// SQLgetPubSeriesName returns the pub_series name for a given pub_series_id,
// or an empty string if not found.
func SQLgetPubSeriesName(db *sql.DB, id int) (string, error) {
	var name sql.NullString
	row := db.QueryRow("SELECT pub_series_name FROM pub_series WHERE pub_series_id=?", id)
	if err := row.Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return name.String, nil
}

// PubSeriesRecord holds one row from the pub_series table.
type PubSeriesRecord struct {
	PubSeriesID   int
	PubSeriesName sql.NullString
	Wikipedia     sql.NullString
	NoteID        sql.NullInt32
}

// SQLCountPubsNotInPubSeries returns the number of publications by this
// publisher that are not assigned to any publication series.
func SQLCountPubsNotInPubSeries(db *sql.DB, publisherID int) (int, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM pubs WHERE publisher_id=? AND pub_series_id IS NULL",
		publisherID,
	).Scan(&count)
	return count, err
}

// SQLFindPubSeriesForPublisher returns the distinct pub_series_id values used
// by this publisher's publications (NULL entries are excluded).
func SQLFindPubSeriesForPublisher(db *sql.DB, publisherID int) ([]int, error) {
	rows, err := db.Query(
		"SELECT DISTINCT pub_series_id FROM pubs WHERE publisher_id=? AND pub_series_id IS NOT NULL",
		publisherID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SQLLoadPubSeriesBatch loads pub_series records for a list of IDs, returned
// sorted by name.
func SQLLoadPubSeriesBatch(db *sql.DB, ids []int) ([]*PubSeriesRecord, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT pub_series_id, pub_series_name, pub_series_wikipedia, pub_series_note_id "+
			"FROM pub_series WHERE pub_series_id IN ("+strings.Join(ph, ",") + ") "+
			"ORDER BY pub_series_name",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*PubSeriesRecord
	for rows.Next() {
		var r PubSeriesRecord
		if err := rows.Scan(&r.PubSeriesID, &r.PubSeriesName, &r.Wikipedia, &r.NoteID); err != nil {
			return nil, err
		}
		result = append(result, &r)
	}
	return result, rows.Err()
}

// SQLGetPubSeriesPubs returns all publications in a given pub series, sorted
// according to display_order:
//   0 = earliest year first
//   1 = latest year first
//   2 = by series number
func SQLGetPubSeriesPubs(db *sql.DB, pubSeriesID, displayOrder int) ([]*Pub, error) {
	orderClause := ""
	switch displayOrder {
	case 1:
		orderClause = "CASE WHEN pub_year='0000-00-00' THEN 1 ELSE 0 END, pub_year DESC, CAST(pub_series_num AS INTEGER), pub_series_num"
	case 2:
		orderClause = "CASE WHEN pub_series_num IS NULL THEN 1 ELSE 0 END, CAST(pub_series_num AS INTEGER), pub_series_num, pub_year"
	default: // 0
		orderClause = "CASE WHEN pub_year='0000-00-00' THEN 1 ELSE 0 END, pub_year, CAST(pub_series_num AS INTEGER), pub_series_num"
	}
	rows, err := db.Query(
		"SELECT pub_id, pub_title, pub_tag, pub_year, publisher_id, pub_pages, "+
			"pub_ptype, pub_ctype, pub_isbn, pub_frontimage, pub_price, note_id, "+
			"pub_series_id, pub_series_num, pub_catalog "+
			"FROM pubs WHERE pub_series_id=? ORDER BY "+orderClause,
		pubSeriesID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPubs(rows)
}

// AuthorPubCount holds one row from SQLGetAllAuthorsForPublisher.
type AuthorPubCount struct {
	AuthorID  int
	Canonical string
	Count     int
}

// SQLGetPubsForAuthorPublisher returns all publications by a given author for
// a given publisher, sorted by date then title.
func SQLGetPubsForAuthorPublisher(db *sql.DB, publisherID, authorID int) ([]*Pub, error) {
	rows, err := db.Query(
		"SELECT p.pub_id, p.pub_title, p.pub_tag, p.pub_year, p.publisher_id, p.pub_pages, "+
			"p.pub_ptype, p.pub_ctype, p.pub_isbn, p.pub_frontimage, p.pub_price, p.note_id, "+
			"p.pub_series_id, p.pub_series_num, p.pub_catalog "+
			"FROM pub_authors pa "+
			"JOIN pubs p ON p.pub_id = pa.pub_id "+
			"WHERE p.publisher_id = ? AND pa.author_id = ? "+
			"ORDER BY p.pub_year, p.pub_title",
		publisherID, authorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPubs(rows)
}

// SQLGetAllAuthorsForPublisher returns all authors who have publications with
// this publisher, along with their publication count.
// sortBy is "name" (lastname then count desc) or "count" (count desc then lastname).
func SQLGetAllAuthorsForPublisher(db *sql.DB, publisherID int, sortBy string) ([]*AuthorPubCount, error) {
	orderClause := "a.author_lastname, cnt DESC"
	if sortBy == "count" {
		orderClause = "cnt DESC, a.author_lastname"
	}
	rows, err := db.Query(
		"SELECT a.author_id, a.author_canonical, COUNT(a.author_canonical) AS cnt "+
			"FROM authors a "+
			"JOIN pub_authors pa ON pa.author_id = a.author_id "+
			"JOIN pubs p ON p.pub_id = pa.pub_id "+
			"WHERE p.publisher_id = ? "+
			"GROUP BY a.author_id "+
			"ORDER BY "+orderClause,
		publisherID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*AuthorPubCount
	for rows.Next() {
		var r AuthorPubCount
		var canonical sql.NullString
		if err := rows.Scan(&r.AuthorID, &canonical, &r.Count); err != nil {
			return nil, err
		}
		r.Canonical = canonical.String
		result = append(result, &r)
	}
	return result, rows.Err()
}

// SQLGetPublisherYears returns the distinct publication years (as integers) for
// a given publisher, sorted ascending.  The MySQL YEAR() function is replaced
// with CAST(SUBSTR(pub_year,1,4) AS INTEGER) for SQLite.
func SQLGetPublisherYears(db *sql.DB, publisherID int) ([]int, error) {
	rows, err := db.Query(
		"SELECT DISTINCT CAST(SUBSTR(pub_year,1,4) AS INTEGER) AS yr "+
			"FROM pubs WHERE publisher_id=? ORDER BY yr",
		publisherID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var years []int
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err != nil {
			return nil, err
		}
		years = append(years, y)
	}
	return years, rows.Err()
}

// SQLgetPublisherNamesBatch returns a map of publisher_id -> name for a list of IDs.
func SQLgetPublisherNamesBatch(db *sql.DB, ids []int) (map[int32]string, error) {
	result := make(map[int32]string)
	if len(ids) == 0 {
		return result, nil
	}
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT publisher_id, publisher_name FROM publishers WHERE publisher_id IN ("+strings.Join(ph, ",")+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int32
		var name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		result[id] = name.String
	}
	return result, rows.Err()
}

// SQLFindPublisher searches for publishers by name.
func SQLFindPublisher(db *sql.DB, target string, exact bool) ([]*Publisher, error) {
	var rows *sql.Rows
	var err error
	if exact {
		rows, err = db.Query(
			"SELECT DISTINCT * FROM publishers WHERE publisher_name = ? ORDER BY publisher_name",
			target,
		)
	} else {
		like := "%" + target + "%"
		rows, err = db.Query(`
			SELECT * FROM (
				SELECT DISTINCT p.publisher_id, p.publisher_name, p.note_id
				FROM publishers p WHERE p.publisher_name LIKE ?
				UNION
				SELECT DISTINCT p.publisher_id, p.publisher_name, p.note_id
				FROM publishers p
				JOIN trans_publisher tp ON tp.publisher_id = p.publisher_id
				WHERE tp.trans_publisher_name LIKE ?
			) ORDER BY publisher_name`,
			like, like,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*Publisher
	for rows.Next() {
		var p Publisher
		if err := rows.Scan(&p.PublisherID, &p.PublisherName, &p.NoteID); err != nil {
			return nil, err
		}
		result = append(result, &p)
	}
	return result, rows.Err()
}

// SQLFindPubSeries searches for publication series by name.
func SQLFindPubSeries(db *sql.DB, target string, exact bool) ([]*PubSeriesRecord, error) {
	var rows *sql.Rows
	var err error
	if exact {
		rows, err = db.Query(
			"SELECT pub_series_id, pub_series_name, pub_series_wikipedia, pub_series_note_id FROM pub_series WHERE pub_series_name = ? ORDER BY pub_series_name",
			target,
		)
	} else {
		like := "%" + target + "%"
		rows, err = db.Query(`
			SELECT * FROM (
				SELECT DISTINCT ps.pub_series_id, ps.pub_series_name, ps.pub_series_wikipedia, ps.pub_series_note_id
				FROM pub_series ps WHERE ps.pub_series_name LIKE ?
				UNION
				SELECT DISTINCT ps.pub_series_id, ps.pub_series_name, ps.pub_series_wikipedia, ps.pub_series_note_id
				FROM pub_series ps
				JOIN trans_pub_series tps ON tps.pub_series_id = ps.pub_series_id
				WHERE tps.trans_pub_series_name LIKE ?
			) ORDER BY pub_series_name`,
			like, like,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*PubSeriesRecord
	for rows.Next() {
		var ps PubSeriesRecord
		if err := rows.Scan(&ps.PubSeriesID, &ps.PubSeriesName, &ps.Wikipedia, &ps.NoteID); err != nil {
			return nil, err
		}
		result = append(result, &ps)
	}
	return result, rows.Err()
}
