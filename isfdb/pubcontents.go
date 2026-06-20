// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"
)

// PubContent represents one row from the pub_content table.
type PubContent struct {
	PubcID  int
	TitleID int
	PubID   int
	Page    string // may be empty
}

// roman2int converts a roman numeral string to an integer; returns 0 on failure.
func roman2int(s string) int {
	s = strings.ToLower(s)
	vals := map[byte]int{'m': 1000, 'd': 500, 'c': 100, 'l': 50, 'x': 10, 'v': 5, 'i': 1}
	sum := 0
	for i := 0; i < len(s); i++ {
		v, ok := vals[s[i]]
		if !ok {
			return 0
		}
		if i+1 < len(s) {
			if next, ok2 := vals[s[i+1]]; ok2 && next > v {
				v = -v
			}
		}
		sum += v
	}
	return sum
}

// convertPageNumber returns (group, normalizedPage, decimalPart) for sorting.
// Groups: 1=no page, 2=cover area, 3=roman, 4=arabic, 5=back matter.
func convertPageNumber(page string) (int, int, string) {
	if page == "" {
		return 1, 0, ""
	}
	// If there is a pipe, the sort value is to the right of it.
	if idx := strings.Index(page, "|"); idx >= 0 {
		page = page[idx+1:]
	}
	if page == "" {
		return 1, 0, ""
	}
	// Strip square brackets.
	if len(page) >= 2 && page[0] == '[' && page[len(page)-1] == ']' {
		page = page[1 : len(page)-1]
	}
	switch page {
	case "fc":
		return 2, 1, ""
	case "sp":
		return 2, 2, ""
	case "rj":
		return 2, 3, ""
	case "edge":
		return 2, 4, ""
	case "te":
		return 2, 5, ""
	case "fe":
		return 2, 6, ""
	case "be":
		return 2, 7, ""
	case "fep":
		return 2, 8, ""
	case "bp":
		return 2, 9, ""
	case "ep":
		return 5, 1, ""
	case "bep":
		return 5, 2, ""
	case "bc":
		return 5, 3, ""
	}
	parts := strings.SplitN(page, ".", 2)
	decimal := ""
	if len(parts) == 2 {
		decimal = parts[1]
	}
	intPart := parts[0]
	if n, err := strconv.Atoi(intPart); err == nil {
		return 4, n, decimal
	}
	if r := roman2int(intPart); r > 0 {
		return 3, r, decimal
	}
	return 1, 0, decimal
}

// SQLGetPubContentListRaw fetches all pub_content rows for a publication.
func SQLGetPubContentListRaw(db *sql.DB, pubID int) ([]*PubContent, error) {
	rows, err := db.Query(
		"SELECT pubc_id, title_id, pub_id, pubc_page FROM pub_content WHERE pub_id=?", pubID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*PubContent
	pos := 1
	for rows.Next() {
		var pc PubContent
		var page sql.NullString
		if err := rows.Scan(&pc.PubcID, &pc.TitleID, &pc.PubID, &page); err != nil {
			return nil, err
		}
		if page.Valid {
			pc.Page = page.String
		}
		pc.PubcID = pos // repurpose as position for stable sort
		pos++
		result = append(result, &pc)
	}
	return result, rows.Err()
}

// GetSortedPubContents returns pub_content records sorted by page number,
// matching the Python getPubContentList() sort order.
func GetSortedPubContents(db *sql.DB, pubID int) ([]*PubContent, error) {
	records, err := SQLGetPubContentListRaw(db, pubID)
	if err != nil {
		return nil, err
	}
	type sortKey struct {
		group    int
		norm     int
		decimal  string
		position int
		pc       *PubContent
	}
	keys := make([]sortKey, len(records))
	for i, pc := range records {
		g, n, d := convertPageNumber(pc.Page)
		keys[i] = sortKey{g, n, d, i, pc}
	}
	sort.SliceStable(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.group != b.group {
			return a.group < b.group
		}
		if a.norm != b.norm {
			return a.norm < b.norm
		}
		if a.decimal != b.decimal {
			return a.decimal < b.decimal
		}
		return a.position < b.position
	})
	sorted := make([]*PubContent, len(keys))
	for i, k := range keys {
		sorted[i] = k.pc
	}
	return sorted, nil
}

// SQLgetSeriesName returns the series_title for a given series_id.
func SQLgetSeriesName(db *sql.DB, seriesID int) (string, error) {
	var name sql.NullString
	err := db.QueryRow("SELECT series_title FROM series WHERE series_id=?", seriesID).Scan(&name)
	if err != nil {
		return "", err
	}
	return name.String, nil
}

// SQLgetSeriesNamesBatch fetches series titles for a set of series IDs.
func SQLgetSeriesNamesBatch(db *sql.DB, seriesIDs []int) (map[int]string, error) {
	result := make(map[int]string)
	if len(seriesIDs) == 0 {
		return result, nil
	}
	ph := make([]string, len(seriesIDs))
	args := make([]any, len(seriesIDs))
	for i, id := range seriesIDs {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT series_id, series_title FROM series WHERE series_id IN ("+strings.Join(ph, ",")+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		result[id] = name.String
	}
	return result, rows.Err()
}

// SQLfindReviewedTitlesBatch returns a map of reviewTitleID -> reviewedTitleID
// for a set of review title IDs.
func SQLfindReviewedTitlesBatch(db *sql.DB, reviewIDs []int) (map[int]int, error) {
	result := make(map[int]int)
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
		"SELECT review_id, title_id FROM title_relationships WHERE review_id IN ("+strings.Join(ph, ",")+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var reviewID, titleID int
		if err := rows.Scan(&reviewID, &titleID); err != nil {
			return nil, err
		}
		result[reviewID] = titleID
	}
	return result, rows.Err()
}
