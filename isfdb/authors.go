// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"sort"
	"strings"
)

// This file serves as the interface to the database. It defines:
// - The main author struct, which holds data from the authors table
// - The various SQL functions use to access author-related data.

type Author struct {
	AuthorID          int
	AuthorCanonical   sql.NullString
	AuthorLegalName   sql.NullString
	AuthorBirthPlace  sql.NullString
	AuthorBirthDate   sql.NullString
	AuthorDeathDate   sql.NullString
	NoteID            sql.NullInt32
	AuthorWikipedia   sql.NullString
	AuthorViews       sql.NullInt32
	AuthorIMDB        sql.NullString
	AuthorMarque      int
	AuthorImage       sql.NullString
	AuthorAnnualViews int
	AuthorLastName    sql.NullString
	AuthorLanguage    sql.NullInt32
	AuthorNote        sql.NullString
}

func SQLloadAuthorData(db *sql.DB, id int) (*Author, error) {
	var a Author

	row := db.QueryRow("SELECT * FROM authors WHERE author_id = ?", id)
	if err := row.Scan(
		&a.AuthorID, &a.AuthorCanonical, &a.AuthorLegalName,
		&a.AuthorBirthPlace, &a.AuthorBirthDate, &a.AuthorDeathDate,
		&a.NoteID, &a.AuthorWikipedia, &a.AuthorViews, &a.AuthorIMDB,
		&a.AuthorMarque, &a.AuthorImage, &a.AuthorAnnualViews,
		&a.AuthorLastName, &a.AuthorLanguage, &a.AuthorNote,
	); err != nil {
		return nil, err
	}

	return &a, nil
}

// SQLgetBriefActualFromPseudo returns the canonical authors that au_id is a pseudonym for.
// (pseudonyms.pseudonym = au_id  →  author is the real person)
func SQLgetBriefActualFromPseudo(db *sql.DB, auID int) ([]AuthorRef, error) {
	rows, err := db.Query(
		"SELECT a.author_id, a.author_canonical FROM authors a "+
			"JOIN pseudonyms p ON p.author_id = a.author_id "+
			"WHERE p.pseudonym = ? "+
			"ORDER BY a.author_lastname, a.author_canonical",
		auID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuthorRefs(rows)
}

// SQLgetBriefPseudoFromActual returns the pseudonyms used by au_id.
// (pseudonyms.author_id = au_id  →  pseudonym is the pen name)
func SQLgetBriefPseudoFromActual(db *sql.DB, auID int) ([]AuthorRef, error) {
	rows, err := db.Query(
		"SELECT a.author_id, a.author_canonical FROM authors a "+
			"JOIN pseudonyms p ON p.pseudonym = a.author_id "+
			"WHERE p.author_id = ? "+
			"ORDER BY a.author_lastname, a.author_canonical",
		auID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuthorRefs(rows)
}

// scanAuthorRefs is a shared helper for scanning (author_id, author_canonical) result sets.
func scanAuthorRefs(rows *sql.Rows) ([]AuthorRef, error) {
	var result []AuthorRef
	for rows.Next() {
		var ref AuthorRef
		var canonical sql.NullString
		if err := rows.Scan(&ref.AuthorID, &canonical); err != nil {
			return nil, err
		}
		ref.Canonical = canonical.String
		result = append(result, ref)
	}
	return result, rows.Err()
}

// SQLTitleAuthors returns the canonical author names for a given title,
// ordered by last name then canonical name. ca_status=1 identifies authors
// (as opposed to editors or interviewees).
func SQLTitleAuthors(db *sql.DB, titleID int) ([]string, error) {
	// Step 1 - get author_ids from canonical_author for this title
	rows, err := db.Query("SELECT author_id FROM canonical_author WHERE title_id=? AND ca_status=1", titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authorIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		authorIDs = append(authorIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(authorIDs) == 0 {
		return []string{}, nil
	}

	// Step 2 - fetch each author record and collect canonical name + last name for sorting
	type authorEntry struct {
		canonical string
		lastName  string
	}
	entries := make([]authorEntry, 0, len(authorIDs))
	for _, aid := range authorIDs {
		a, err := SQLloadAuthorData(db, aid)
		if err != nil {
			return nil, err
		}
		entries = append(entries, authorEntry{
			canonical: a.AuthorCanonical.String,
			lastName:  a.AuthorLastName.String,
		})
	}

	// Step 3 - sort by last name, then canonical name
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].lastName != entries[j].lastName {
			return entries[i].lastName < entries[j].lastName
		}
		return entries[i].canonical < entries[j].canonical
	})

	// Step 4 - return just the canonical names
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.canonical
	}
	return names, nil
}

// AuthorRef holds an author ID and canonical name, used for generating links.
type AuthorRef struct {
	AuthorID  int
	Canonical string
	LastName  string
}

// SQLTitleAuthorsBatch returns a map of titleID -> []AuthorRef for a list of
// title IDs, using IN-clause queries to minimise round-trips.
func SQLTitleAuthorsBatch(db *sql.DB, titleIDs []int) (map[int][]AuthorRef, error) {
	result := make(map[int][]AuthorRef)
	if len(titleIDs) == 0 {
		return result, nil
	}

	ph := make([]string, len(titleIDs))
	args := make([]any, len(titleIDs))
	for i, id := range titleIDs {
		ph[i] = "?"
		args[i] = id
	}

	// Step 1 - get all title_id -> author_id mappings in one query
	rows, err := db.Query(
		"SELECT title_id, author_id FROM canonical_author WHERE ca_status=1 AND title_id IN ("+strings.Join(ph, ",")+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	titleAuthorIDs := make(map[int][]int)
	authorIDset := make(map[int]struct{})
	for rows.Next() {
		var titleID, authorID int
		if err := rows.Scan(&titleID, &authorID); err != nil {
			rows.Close()
			return nil, err
		}
		titleAuthorIDs[titleID] = append(titleAuthorIDs[titleID], authorID)
		authorIDset[authorID] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(authorIDset) == 0 {
		return result, nil
	}

	// Step 2 - fetch all author records in one query
	authorIDs := make([]int, 0, len(authorIDset))
	for id := range authorIDset {
		authorIDs = append(authorIDs, id)
	}
	aph := make([]string, len(authorIDs))
	aargs := make([]any, len(authorIDs))
	for i, id := range authorIDs {
		aph[i] = "?"
		aargs[i] = id
	}
	arows, err := db.Query(
		"SELECT author_id, author_canonical, author_lastname FROM authors WHERE author_id IN ("+strings.Join(aph, ",")+")",
		aargs...,
	)
	if err != nil {
		return nil, err
	}
	authorMap := make(map[int]AuthorRef)
	for arows.Next() {
		var a AuthorRef
		var canonical, lastName sql.NullString
		if err := arows.Scan(&a.AuthorID, &canonical, &lastName); err != nil {
			arows.Close()
			return nil, err
		}
		a.Canonical = canonical.String
		a.LastName = lastName.String
		authorMap[a.AuthorID] = a
	}
	arows.Close()
	if err := arows.Err(); err != nil {
		return nil, err
	}

	// Step 3 - assemble and sort per title
	for titleID, aids := range titleAuthorIDs {
		entries := make([]AuthorRef, 0, len(aids))
		for _, aid := range aids {
			if a, ok := authorMap[aid]; ok {
				entries = append(entries, a)
			}
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].LastName != entries[j].LastName {
				return entries[i].LastName < entries[j].LastName
			}
			return entries[i].Canonical < entries[j].Canonical
		})
		result[titleID] = entries
	}
	return result, nil
}

// SQLTitleBriefAuthorRecords returns AuthorRef records for a given title,
// ordered by last name then canonical name. ca_status=1 identifies authors.
func SQLTitleBriefAuthorRecords(db *sql.DB, titleID int) ([]AuthorRef, error) {
	// Step 1 - get author_ids from canonical_author for this title
	rows, err := db.Query("SELECT author_id FROM canonical_author WHERE title_id=? AND ca_status=1", titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authorIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		authorIDs = append(authorIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(authorIDs) == 0 {
		return []AuthorRef{}, nil
	}

	// Step 2 - fetch each author record
	entries := make([]AuthorRef, 0, len(authorIDs))
	for _, aid := range authorIDs {
		a, err := SQLloadAuthorData(db, aid)
		if err != nil {
			return nil, err
		}
		entries = append(entries, AuthorRef{
			AuthorID:  a.AuthorID,
			Canonical: a.AuthorCanonical.String,
			LastName:  a.AuthorLastName.String,
		})
	}

	// Step 3 - sort by last name, then canonical name
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].LastName != entries[j].LastName {
			return entries[i].LastName < entries[j].LastName
		}
		return entries[i].Canonical < entries[j].Canonical
	})

	return entries, nil
}

// SQLPubAuthors returns AuthorRef records for a given publication,
// ordered by last name then canonical name.
func SQLPubAuthors(db *sql.DB, pubID int) ([]AuthorRef, error) {
	m, err := SQLPubAuthorsBatch(db, []int{pubID})
	if err != nil {
		return nil, err
	}
	return m[pubID], nil
}

// SQLPubAuthorsBatch returns a map of pubID -> []AuthorRef for a list of pub IDs,
// using IN-clause queries to minimise round-trips.
func SQLPubAuthorsBatch(db *sql.DB, pubIDs []int) (map[int][]AuthorRef, error) {
	result := make(map[int][]AuthorRef)
	if len(pubIDs) == 0 {
		return result, nil
	}

	ph := make([]string, len(pubIDs))
	args := make([]any, len(pubIDs))
	for i, id := range pubIDs {
		ph[i] = "?"
		args[i] = id
	}

	// Step 1 - get all pub_id -> author_id mappings in one query
	rows, err := db.Query(
		"SELECT pub_id, author_id FROM pub_authors WHERE pub_id IN ("+strings.Join(ph, ",")+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pubAuthorIDs := make(map[int][]int)
	authorIDset := make(map[int]struct{})
	for rows.Next() {
		var pubID, authorID int
		if err := rows.Scan(&pubID, &authorID); err != nil {
			return nil, err
		}
		pubAuthorIDs[pubID] = append(pubAuthorIDs[pubID], authorID)
		authorIDset[authorID] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(authorIDset) == 0 {
		return result, nil
	}

	// Step 2 - fetch all author records in one query
	authorIDs := make([]int, 0, len(authorIDset))
	for id := range authorIDset {
		authorIDs = append(authorIDs, id)
	}
	aph := make([]string, len(authorIDs))
	aargs := make([]any, len(authorIDs))
	for i, id := range authorIDs {
		aph[i] = "?"
		aargs[i] = id
	}
	arows, err := db.Query(
		"SELECT author_id, author_canonical, author_lastname FROM authors WHERE author_id IN ("+strings.Join(aph, ",")+")",
		aargs...,
	)
	if err != nil {
		return nil, err
	}
	defer arows.Close()

	authorMap := make(map[int]AuthorRef)
	for arows.Next() {
		var a AuthorRef
		var canonical, lastName sql.NullString
		if err := arows.Scan(&a.AuthorID, &canonical, &lastName); err != nil {
			return nil, err
		}
		a.Canonical = canonical.String
		a.LastName = lastName.String
		authorMap[a.AuthorID] = a
	}
	arows.Close()

	// Step 3 - assemble and sort per pub
	for pubID, aids := range pubAuthorIDs {
		entries := make([]AuthorRef, 0, len(aids))
		for _, aid := range aids {
			if a, ok := authorMap[aid]; ok {
				entries = append(entries, a)
			}
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].LastName != entries[j].LastName {
				return entries[i].LastName < entries[j].LastName
			}
			return entries[i].Canonical < entries[j].Canonical
		})
		result[pubID] = entries
	}
	return result, nil
}

// SQLGetCoverAuthorsForPubs returns a map of pub_id to []AuthorRef for the
// cover artists of each pub in the given list, using batch IN-clause queries.
func SQLGetCoverAuthorsForPubs(db *sql.DB, pubIDs []int) (map[int][]AuthorRef, error) {
	results := make(map[int][]AuthorRef)
	if len(pubIDs) == 0 {
		return results, nil
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

	// Step 1 - get all pub_id -> title_id mappings from pub_content in one query
	rows, err := db.Query(
		"SELECT pub_id, title_id FROM pub_content WHERE pub_id IN ("+ph(len(pubIDs))+")",
		toAny(pubIDs)...,
	)
	if err != nil {
		return nil, err
	}
	pubTitleIDs := make(map[int][]int) // pub_id -> []title_id
	titleIDset := make(map[int]struct{})
	for rows.Next() {
		var pubID, titleID int
		if err := rows.Scan(&pubID, &titleID); err != nil {
			rows.Close()
			return nil, err
		}
		pubTitleIDs[pubID] = append(pubTitleIDs[pubID], titleID)
		titleIDset[titleID] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(titleIDset) == 0 {
		return results, nil
	}

	// Step 2 - get title_ttype for all title_ids, filter for COVERART
	titleIDs := make([]int, 0, len(titleIDset))
	for id := range titleIDset {
		titleIDs = append(titleIDs, id)
	}
	trows, err := db.Query(
		"SELECT title_id, title_ttype FROM titles WHERE title_id IN ("+ph(len(titleIDs))+")",
		toAny(titleIDs)...,
	)
	if err != nil {
		return nil, err
	}
	coverTitleIDs := make(map[int]struct{})
	for trows.Next() {
		var titleID int
		var ttype sql.NullString
		if err := trows.Scan(&titleID, &ttype); err != nil {
			trows.Close()
			return nil, err
		}
		if ttype.String == "COVERART" {
			coverTitleIDs[titleID] = struct{}{}
		}
	}
	trows.Close()
	if err := trows.Err(); err != nil {
		return nil, err
	}
	if len(coverTitleIDs) == 0 {
		return results, nil
	}

	// Step 3 - get author_ids from canonical_author for all COVERART title_ids
	coverIDs := make([]int, 0, len(coverTitleIDs))
	for id := range coverTitleIDs {
		coverIDs = append(coverIDs, id)
	}
	caRows, err := db.Query(
		"SELECT title_id, author_id FROM canonical_author WHERE title_id IN ("+ph(len(coverIDs))+")",
		toAny(coverIDs)...,
	)
	if err != nil {
		return nil, err
	}
	titleAuthorIDs := make(map[int][]int) // title_id -> []author_id
	authorIDset := make(map[int]struct{})
	for caRows.Next() {
		var titleID, authorID int
		if err := caRows.Scan(&titleID, &authorID); err != nil {
			caRows.Close()
			return nil, err
		}
		titleAuthorIDs[titleID] = append(titleAuthorIDs[titleID], authorID)
		authorIDset[authorID] = struct{}{}
	}
	caRows.Close()
	if err := caRows.Err(); err != nil {
		return nil, err
	}
	if len(authorIDset) == 0 {
		return results, nil
	}

	// Step 4 - fetch all author records in one query
	authorIDs := make([]int, 0, len(authorIDset))
	for id := range authorIDset {
		authorIDs = append(authorIDs, id)
	}
	arows, err := db.Query(
		"SELECT author_id, author_canonical, author_lastname FROM authors WHERE author_id IN ("+ph(len(authorIDs))+")",
		toAny(authorIDs)...,
	)
	if err != nil {
		return nil, err
	}
	authorMap := make(map[int]AuthorRef)
	for arows.Next() {
		var a AuthorRef
		var canonical, lastName sql.NullString
		if err := arows.Scan(&a.AuthorID, &canonical, &lastName); err != nil {
			arows.Close()
			return nil, err
		}
		a.Canonical = canonical.String
		a.LastName = lastName.String
		authorMap[a.AuthorID] = a
	}
	arows.Close()
	if err := arows.Err(); err != nil {
		return nil, err
	}

	// Step 5 - assemble results: for each pub, find its COVERART titles and their authors
	for pubID, titleIDs := range pubTitleIDs {
		for _, titleID := range titleIDs {
			if _, isCover := coverTitleIDs[titleID]; !isCover {
				continue
			}
			for _, authorID := range titleAuthorIDs[titleID] {
				if a, ok := authorMap[authorID]; ok {
					results[pubID] = append(results[pubID], a)
				}
			}
		}
		sort.Slice(results[pubID], func(i, j int) bool {
			if results[pubID][i].LastName != results[pubID][j].LastName {
				return results[pubID][i].LastName < results[pubID][j].LastName
			}
			return results[pubID][i].Canonical < results[pubID][j].Canonical
		})
	}

	return results, nil
}

// SQLReviewedAuthors returns the authors of the work being reviewed by a REVIEW title.
// In the database, ca_status=3 marks the reviewed title's authors on the review's canonical_author row.
func SQLReviewedAuthors(db *sql.DB, titleID int) ([]AuthorRef, error) {
	rows, err := db.Query(
		"SELECT a.author_id, a.author_canonical "+
			"FROM authors a JOIN canonical_author ca ON ca.author_id = a.author_id "+
			"WHERE ca.title_id = ? AND ca.ca_status = 3",
		titleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuthorRefs(rows)
}

// SQLReviewedAuthorsBatch returns a map of reviewTitleID -> []AuthorRef for the
// authors of the reviewed works (ca_status=3) across a batch of review title IDs.
func SQLReviewedAuthorsBatch(db *sql.DB, reviewIDs []int) (map[int][]AuthorRef, error) {
	result := make(map[int][]AuthorRef)
	if len(reviewIDs) == 0 {
		return result, nil
	}
	ph := make([]string, len(reviewIDs))
	args := make([]any, len(reviewIDs))
	for i, id := range reviewIDs {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT ca.title_id, a.author_id, a.author_canonical "+
			"FROM canonical_author ca JOIN authors a ON a.author_id = ca.author_id "+
			"WHERE ca.ca_status = 3 AND ca.title_id IN ("+strings.Join(ph, ",")+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var titleID int
		var ref AuthorRef
		var canonical sql.NullString
		if err := rows.Scan(&titleID, &ref.AuthorID, &canonical); err != nil {
			return nil, err
		}
		ref.Canonical = canonical.String
		result[titleID] = append(result[titleID], ref)
	}
	return result, rows.Err()
}

// SQLIntervieweeAuthors returns the interviewees for an INTERVIEW title,
// excluding the given author (to avoid showing the page author as a co-interviewee).
// ca_status=2 marks interviewees.
func SQLIntervieweeAuthors(db *sql.DB, titleID int, excludeAuthorID int) ([]AuthorRef, error) {
	rows, err := db.Query(
		"SELECT a.author_id, a.author_canonical "+
			"FROM authors a JOIN canonical_author ca ON ca.author_id = a.author_id "+
			"WHERE ca.title_id = ? AND ca.author_id <> ? AND ca.ca_status = 2",
		titleID, excludeAuthorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuthorRefs(rows)
}

// SQLFindAuthors searches for authors by canonical name (and translated names).
// exact=true matches only the exact canonical name; otherwise LIKE '%target%'.
func SQLFindAuthors(db *sql.DB, target string, exact bool) ([]*Author, error) {
	var rows *sql.Rows
	var err error
	if exact {
		rows, err = db.Query(
			"SELECT DISTINCT * FROM authors WHERE author_canonical = ? ORDER BY author_canonical",
			target,
		)
	} else {
		like := "%" + target + "%"
		rows, err = db.Query(`
			SELECT * FROM (
				SELECT DISTINCT a.author_id, a.author_canonical, a.author_legalname,
				       a.author_birthplace, a.author_birthdate, a.author_deathdate,
				       a.note_id, a.author_wikipedia, a.author_views, a.author_imdb,
				       a.author_marque, a.author_image, a.author_annualviews,
				       a.author_lastname, a.author_language, a.author_note
				FROM authors a WHERE a.author_canonical LIKE ?
				UNION
				SELECT DISTINCT a.author_id, a.author_canonical, a.author_legalname,
				       a.author_birthplace, a.author_birthdate, a.author_deathdate,
				       a.note_id, a.author_wikipedia, a.author_views, a.author_imdb,
				       a.author_marque, a.author_image, a.author_annualviews,
				       a.author_lastname, a.author_language, a.author_note
				FROM authors a
				JOIN trans_authors ta ON ta.author_id = a.author_id
				WHERE ta.trans_author_name LIKE ?
			) ORDER BY author_canonical`,
			like, like,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*Author
	for rows.Next() {
		var a Author
		if err := rows.Scan(
			&a.AuthorID, &a.AuthorCanonical, &a.AuthorLegalName,
			&a.AuthorBirthPlace, &a.AuthorBirthDate, &a.AuthorDeathDate,
			&a.NoteID, &a.AuthorWikipedia, &a.AuthorViews, &a.AuthorIMDB,
			&a.AuthorMarque, &a.AuthorImage, &a.AuthorAnnualViews,
			&a.AuthorLastName, &a.AuthorLanguage, &a.AuthorNote,
		); err != nil {
			return nil, err
		}
		result = append(result, &a)
	}
	return result, rows.Err()
}

// SQLBatchAuthorIsPseudo returns the set of author IDs that are pseudonyms
// (i.e. have at least one entry in canonical_author with ca_status != 1 whose
// parent exists, or more simply: appear as a pseudo in pseudo_authors).
// Returns a map authorID → true for each pseudonym.
func SQLBatchAuthorIsPseudo(db *sql.DB, authorIDs []int) (map[int]bool, error) {
	if len(authorIDs) == 0 {
		return map[int]bool{}, nil
	}
	ph := make([]string, len(authorIDs))
	args := make([]any, len(authorIDs))
	for i, id := range authorIDs {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT DISTINCT author_id FROM pseudonyms WHERE author_id IN ("+
			strings.Join(ph, ",")+") ",
		args...,
	)
	if err != nil {
		// pseudonyms table may not exist; return empty map
		return map[int]bool{}, nil
	}
	defer rows.Close()
	result := map[int]bool{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = true
	}
	return result, rows.Err()
}
