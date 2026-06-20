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
// - The main title struct, which holds data from the titles table
// - The various SQL functions use to access title-related data.

type Title struct {
	TitleID          int
	TitleTitle       sql.NullString
	TitleTranslator  sql.NullString
	TitleSynopsis    sql.NullInt32
	NoteID           sql.NullInt32
	SeriesID         sql.NullInt32
	TitleSeriesNum   sql.NullInt32
	TitleCopyright   sql.NullString
	TitleStoryLen    sql.NullString
	TitleTType       sql.NullString
	TitleWikipedia   sql.NullString
	TitleViews       int
	TitleParent      int
	TitleRating      sql.NullFloat64
	TitleAnnualViews int
	TitleCTL         int
	TitleLanguage    sql.NullInt32
	TitleSeriesNum2  sql.NullString
	TitleNonGenre    sql.NullString
	TitleGraphic     sql.NullString
	TitleNVZ         sql.NullString
	TitleJVN         sql.NullString
	TitleContent     sql.NullString
}

func SQLloadTitleData(db *sql.DB, id int) (*Title, error) {
	var t Title

	row := db.QueryRow("SELECT * FROM titles WHERE title_id = ?", id)
	if err := row.Scan(
		&t.TitleID, &t.TitleTitle, &t.TitleTranslator,
		&t.TitleSynopsis, &t.NoteID, &t.SeriesID,
		&t.TitleSeriesNum, &t.TitleCopyright, &t.TitleStoryLen,
		&t.TitleTType, &t.TitleWikipedia, &t.TitleViews,
		&t.TitleParent, &t.TitleRating, &t.TitleAnnualViews,
		&t.TitleCTL, &t.TitleLanguage, &t.TitleSeriesNum2,
		&t.TitleNonGenre, &t.TitleGraphic, &t.TitleNVZ,
		&t.TitleJVN, &t.TitleContent,
	); err != nil {
		return nil, err
	}

	return &t, nil
}

// SQLGetPubTitles returns all Title records belonging to a publication,
// in pub_content order, by joining pub_content with titles.
func SQLGetPubTitles(db *sql.DB, pubID int) ([]*Title, error) {
	rows, err := db.Query(
		"SELECT title_id FROM pub_content WHERE pub_id=? ORDER BY title_id",
		pubID,
	)
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
		return nil, nil
	}

	ph := make([]string, len(titleIDs))
	args := make([]any, len(titleIDs))
	for i, id := range titleIDs {
		ph[i] = "?"
		args[i] = id
	}
	trows, err := db.Query(
		"SELECT title_id, title_title, title_translator, title_synopsis, note_id, series_id, "+
			"title_seriesnum, title_copyright, title_storylen, title_ttype, title_wikipedia, "+
			"title_views, title_parent, title_rating, title_annualviews, title_ctl, "+
			"title_language, title_seriesnum_2, title_non_genre, title_graphic, title_nvz, "+
			"title_jvn, title_content "+
			"FROM titles WHERE title_id IN ("+strings.Join(ph, ",")+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer trows.Close()

	titleMap := make(map[int]*Title, len(titleIDs))
	for trows.Next() {
		var t Title
		if err := trows.Scan(
			&t.TitleID, &t.TitleTitle, &t.TitleTranslator,
			&t.TitleSynopsis, &t.NoteID, &t.SeriesID,
			&t.TitleSeriesNum, &t.TitleCopyright, &t.TitleStoryLen,
			&t.TitleTType, &t.TitleWikipedia, &t.TitleViews,
			&t.TitleParent, &t.TitleRating, &t.TitleAnnualViews,
			&t.TitleCTL, &t.TitleLanguage, &t.TitleSeriesNum2,
			&t.TitleNonGenre, &t.TitleGraphic, &t.TitleNVZ,
			&t.TitleJVN, &t.TitleContent,
		); err != nil {
			return nil, err
		}
		titleMap[t.TitleID] = &t
	}
	if err := trows.Err(); err != nil {
		return nil, err
	}

	// Return in pub_content order
	result := make([]*Title, 0, len(titleIDs))
	for _, id := range titleIDs {
		if t, ok := titleMap[id]; ok {
			result = append(result, t)
		}
	}
	return result, nil
}

// SQLloadTitlesBatch fetches a set of Title records by ID and returns them as a map.
func SQLloadTitlesBatch(db *sql.DB, titleIDs []int) (map[int]*Title, error) {
	result := make(map[int]*Title)
	if len(titleIDs) == 0 {
		return result, nil
	}
	ph := make([]string, len(titleIDs))
	args := make([]any, len(titleIDs))
	for i, id := range titleIDs {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT title_id, title_title, title_translator, title_synopsis, note_id, series_id, "+
			"title_seriesnum, title_copyright, title_storylen, title_ttype, title_wikipedia, "+
			"title_views, title_parent, title_rating, title_annualviews, title_ctl, "+
			"title_language, title_seriesnum_2, title_non_genre, title_graphic, title_nvz, "+
			"title_jvn, title_content "+
			"FROM titles WHERE title_id IN ("+strings.Join(ph, ",")+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t Title
		if err := rows.Scan(
			&t.TitleID, &t.TitleTitle, &t.TitleTranslator,
			&t.TitleSynopsis, &t.NoteID, &t.SeriesID,
			&t.TitleSeriesNum, &t.TitleCopyright, &t.TitleStoryLen,
			&t.TitleTType, &t.TitleWikipedia, &t.TitleViews,
			&t.TitleParent, &t.TitleRating, &t.TitleAnnualViews,
			&t.TitleCTL, &t.TitleLanguage, &t.TitleSeriesNum2,
			&t.TitleNonGenre, &t.TitleGraphic, &t.TitleNVZ,
			&t.TitleJVN, &t.TitleContent,
		); err != nil {
			return nil, err
		}
		result[t.TitleID] = &t
	}
	return result, rows.Err()
}

// TitleReview holds the result of a review lookup — one row from SQLloadAllTitleReviews.
type TitleReview struct {
	ReviewID       int
	ReviewCopyright sql.NullString
	LanguageID     sql.NullInt32
	ReviewParentID sql.NullInt32
	ParentCopyright sql.NullString
	PubID          int
	PubTitle       sql.NullString
	PubYear        sql.NullString
}

// getReviewIDs returns all review_ids from title_relationships for a given title_id.
func getReviewIDs(db *sql.DB, titleID int) ([]int, error) {
	rows, err := db.Query("SELECT review_id FROM title_relationships WHERE title_id=?", titleID)
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

// getReviewIDsBatch returns all review_ids from title_relationships for a list of title_ids.
func getReviewIDsBatch(db *sql.DB, titleIDs []int) ([]int, error) {
	if len(titleIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(titleIDs))
	args := make([]any, len(titleIDs))
	for i, id := range titleIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT review_id FROM title_relationships WHERE title_id IN ("+strings.Join(placeholders, ",")+")",
		args...,
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

// getVariantTitleIDs returns all title_ids from titles WHERE title_parent = parentID.
func getVariantTitleIDs(db *sql.DB, parentID int) ([]int, error) {
	rows, err := db.Query("SELECT title_id FROM titles WHERE title_parent=?", parentID)
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

// getPubIDsForTitle returns all pub_ids from pub_content for a given title_id.
func getPubIDsForTitle(db *sql.DB, titleID int) ([]int, error) {
	rows, err := db.Query("SELECT pub_id FROM pub_content WHERE title_id=?", titleID)
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

// buildTitleReviewsBatch constructs TitleReview records for a batch of review
// title_ids using IN-clause queries to minimise round-trips to SQLite.
// overrideParentIDs maps reviewID -> parentID for variant reviews (0 = use title_parent).
func buildTitleReviewsBatch(db *sql.DB, reviewIDs []int, overrideParentIDs map[int]int, seen map[int]*TitleReview) error {
	if len(reviewIDs) == 0 {
		return nil
	}

	// Build the IN clause placeholder string
	inClause := func(n int) string {
		s := make([]string, n)
		for i := range s {
			s[i] = "?"
		}
		return "(" + strings.Join(s, ",") + ")"
	}
	toAny := func(ids []int) []any {
		a := make([]any, len(ids))
		for i, id := range ids {
			a[i] = id
		}
		return a
	}

	// Step 1 - fetch all review title rows in one query
	type titleRow struct {
		titleID    int
		copyright  sql.NullString
		language   sql.NullInt32
		titleParent int
	}
	titleRows := make(map[int]titleRow)
	rows, err := db.Query(
		"SELECT title_id, title_copyright, title_language, title_parent FROM titles WHERE title_id IN "+inClause(len(reviewIDs)),
		toAny(reviewIDs)...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tr titleRow
		if err := rows.Scan(&tr.titleID, &tr.copyright, &tr.language, &tr.titleParent); err != nil {
			return err
		}
		titleRows[tr.titleID] = tr
	}
	rows.Close()

	// Step 2 - collect parent IDs we need to look up
	parentIDset := make(map[int]struct{})
	for _, reviewID := range reviewIDs {
		tr := titleRows[reviewID]
		pid := overrideParentIDs[reviewID]
		if pid == 0 {
			pid = tr.titleParent
		}
		if pid != 0 {
			parentIDset[pid] = struct{}{}
		}
	}
	parentCopyrights := make(map[int]sql.NullString)
	if len(parentIDset) > 0 {
		parentIDs := make([]int, 0, len(parentIDset))
		for id := range parentIDset {
			parentIDs = append(parentIDs, id)
		}
		prows, err := db.Query(
			"SELECT title_id, title_copyright FROM titles WHERE title_id IN "+inClause(len(parentIDs)),
			toAny(parentIDs)...,
		)
		if err != nil {
			return err
		}
		for prows.Next() {
			var pid int
			var cop sql.NullString
			if err := prows.Scan(&pid, &cop); err != nil {
				prows.Close()
				return err
			}
			parentCopyrights[pid] = cop
		}
		prows.Close()
	}

	// Step 3 - fetch all pub_content rows for the review IDs in one query
	type pcRow struct {
		titleID int
		pubID   int
	}
	pcRows, err := db.Query(
		"SELECT title_id, pub_id FROM pub_content WHERE title_id IN "+inClause(len(reviewIDs)),
		toAny(reviewIDs)...,
	)
	if err != nil {
		return err
	}
	var pubContentRows []pcRow
	pubIDset := make(map[int]struct{})
	for pcRows.Next() {
		var pc pcRow
		if err := pcRows.Scan(&pc.titleID, &pc.pubID); err != nil {
			pcRows.Close()
			return err
		}
		pubContentRows = append(pubContentRows, pc)
		pubIDset[pc.pubID] = struct{}{}
	}
	pcRows.Close()

	if len(pubIDset) == 0 {
		return nil
	}

	// Step 4 - fetch all pub title+year in one query
	pubIDs := make([]int, 0, len(pubIDset))
	for id := range pubIDset {
		pubIDs = append(pubIDs, id)
	}
	type pubRow struct {
		pubID    int
		pubTitle sql.NullString
		pubYear  sql.NullString
	}
	pubData := make(map[int]pubRow)
	pubRows, err := db.Query(
		"SELECT pub_id, pub_title, pub_year FROM pubs WHERE pub_id IN "+inClause(len(pubIDs)),
		toAny(pubIDs)...,
	)
	if err != nil {
		return err
	}
	for pubRows.Next() {
		var pr pubRow
		if err := pubRows.Scan(&pr.pubID, &pr.pubTitle, &pr.pubYear); err != nil {
			pubRows.Close()
			return err
		}
		pubData[pr.pubID] = pr
	}
	pubRows.Close()

	// Step 5 - assemble TitleReview records
	for _, pc := range pubContentRows {
		tr := titleRows[pc.titleID]
		pr := pubData[pc.pubID]

		pid := overrideParentIDs[pc.titleID]
		if pid == 0 {
			pid = tr.titleParent
		}
		parentCop := parentCopyrights[pid]

		r := &TitleReview{
			ReviewID:        pc.titleID,
			ReviewCopyright: tr.copyright,
			LanguageID:      tr.language,
			ReviewParentID:  sql.NullInt32{Int32: int32(pid), Valid: pid != 0},
			ParentCopyright: parentCop,
			PubID:           pc.pubID,
			PubTitle:        pr.pubTitle,
			PubYear:         pr.pubYear,
		}
		key := pc.titleID*1000000 + pc.pubID
		if _, exists := seen[key]; !exists {
			seen[key] = r
		}
	}
	return nil
}

// SQLloadAllTitleReviews returns all reviews of a title (and its variants),
// decomposed into simple indexed lookups to avoid slow multi-table JOINs.
func SQLloadAllTitleReviews(db *sql.DB, titleID int) ([]*TitleReview, error) {
	if titleID == 0 {
		return nil, nil
	}

	seen := make(map[int]*TitleReview)

	// Query 1 - Reviews of the main title
	reviewIDs, err := getReviewIDs(db, titleID)
	if err != nil {
		return nil, err
	}
	if err := buildTitleReviewsBatch(db, reviewIDs, map[int]int{}, seen); err != nil {
		return nil, err
	}

	// Query 2 - Reviews of variant titles (VTs) of the main title
	vtIDs, err := getVariantTitleIDs(db, titleID)
	if err != nil {
		return nil, err
	}
	if len(vtIDs) > 0 {
		vtReviewIDs, err := getReviewIDsBatch(db, vtIDs)
		if err != nil {
			return nil, err
		}
		if err := buildTitleReviewsBatch(db, vtReviewIDs, map[int]int{}, seen); err != nil {
			return nil, err
		}
	}

	// Query 3 - Variants of reviews of the main title
	for _, reviewID := range reviewIDs {
		variantReviewIDs, err := getVariantTitleIDs(db, reviewID)
		if err != nil {
			return nil, err
		}
		overrides := make(map[int]int, len(variantReviewIDs))
		for _, vid := range variantReviewIDs {
			overrides[vid] = reviewID
		}
		if err := buildTitleReviewsBatch(db, variantReviewIDs, overrides, seen); err != nil {
			return nil, err
		}
	}

	// Query 4 - Variants of reviews of VTs of the main title
	for _, vtID := range vtIDs {
		vtReviewIDs, err := getReviewIDs(db, vtID)
		if err != nil {
			return nil, err
		}
		for _, reviewID := range vtReviewIDs {
			variantReviewIDs, err := getVariantTitleIDs(db, reviewID)
			if err != nil {
				return nil, err
			}
			overrides := make(map[int]int, len(variantReviewIDs))
			for _, vid := range variantReviewIDs {
				overrides[vid] = reviewID
			}
			if err := buildTitleReviewsBatch(db, variantReviewIDs, overrides, seen); err != nil {
				return nil, err
			}
		}
	}

	// Collect and sort by pub_year then pub_title
	reviews := make([]*TitleReview, 0, len(seen))
	for _, r := range seen {
		reviews = append(reviews, r)
	}
	sort.Slice(reviews, func(i, j int) bool {
		yi, yj := reviews[i].PubYear.String, reviews[j].PubYear.String
		if yi != yj {
			return yi < yj
		}
		return reviews[i].PubTitle.String < reviews[j].PubTitle.String
	})

	return reviews, nil
}

// titleSearchCols is the explicit column list used by title search queries,
// matching the order expected by scanTitles.
const titleSearchCols = `t.title_id, t.title_title, t.title_translator, t.title_synopsis, t.note_id,
	t.series_id, t.title_seriesnum, t.title_copyright, t.title_storylen,
	t.title_ttype, t.title_wikipedia, t.title_views,
	t.title_parent, t.title_rating, t.title_annualviews,
	t.title_ctl, t.title_language, t.title_seriesnum_2,
	t.title_non_genre, t.title_graphic, t.title_nvz,
	t.title_jvn, t.title_content`

// SQLFindTitles searches all titles (including translations) by title string.
func SQLFindTitles(db *sql.DB, target string) ([]*Title, error) {
	like := "%" + target + "%"
	q := `SELECT * FROM (
		SELECT DISTINCT ` + titleSearchCols + ` FROM titles t WHERE t.title_title LIKE ?
		UNION
		SELECT DISTINCT ` + titleSearchCols + ` FROM titles t
		JOIN trans_titles tt ON tt.title_id = t.title_id
		WHERE tt.trans_title_title LIKE ?
	) ORDER BY title_title`
	rows, err := db.Query(q, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTitles(rows)
}

// SQLFindFictionTitles searches fiction-type titles only.
func SQLFindFictionTitles(db *sql.DB, target string) ([]*Title, error) {
	like := "%" + target + "%"
	const ftypes = `'ANTHOLOGY','COLLECTION','EDITOR','NOVEL','OMNIBUS','POEM','SERIAL','SHORTFICTION','CHAPBOOK'`
	q := `SELECT * FROM (
		SELECT DISTINCT ` + titleSearchCols + ` FROM titles t
		WHERE t.title_title LIKE ? AND t.title_ttype IN (` + ftypes + `)
		UNION
		SELECT DISTINCT ` + titleSearchCols + ` FROM titles t
		JOIN trans_titles tt ON tt.title_id = t.title_id
		WHERE tt.trans_title_title LIKE ? AND t.title_ttype IN (` + ftypes + `)
	) ORDER BY title_title`
	rows, err := db.Query(q, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTitles(rows)
}

// SQLFindYear returns all titles whose copyright year matches the given year.
func SQLFindYear(db *sql.DB, year int) ([]*Title, error) {
	q := `SELECT ` + titleSearchCols + ` FROM titles t
	      WHERE substr(t.title_copyright, 1, 4) = ?
	      ORDER BY t.title_ttype, t.title_title`
	rows, err := db.Query(q, fmt.Sprintf("%04d", year))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTitles(rows)
}

// SQLFindMonth returns all titles whose copyright year-month matches "YYYY-MM".
func SQLFindMonth(db *sql.DB, yearMonth string) ([]*Title, error) {
	q := `SELECT ` + titleSearchCols + ` FROM titles t
	      WHERE t.title_copyright LIKE ?
	      ORDER BY t.title_title`
	rows, err := db.Query(q, yearMonth+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTitles(rows)
}
