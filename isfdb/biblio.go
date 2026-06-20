// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"sort"
	"strings"
)

// ── Series type priority constants ────────────────────────────────────────────
// These match the Python SERIES_TYPE_* values and control the display order
// of series sections in the bibliography.
const (
	SeriesTypeFiction     = 1
	SeriesTypeEdit        = 2
	SeriesTypeAnth        = 3
	SeriesTypeNonfic      = 4
	SeriesTypeSF          = 5
	SeriesTypePoem        = 6
	SeriesTypeEssay       = 7
	SeriesTypeCoverArt    = 8
	SeriesTypeInteriorArt = 9
	SeriesTypeReview      = 10
	SeriesTypeInterview   = 11
	SeriesTypeOther       = 12
)

// seriesTypePriority maps a title's ttype to the series priority bucket
// it belongs to (matching Python's type_order dict).
var seriesTypePriority = map[string]int{
	"NOVEL":       SeriesTypeFiction,
	"COLLECTION":  SeriesTypeFiction,
	"SERIAL":      SeriesTypeFiction,
	"OMNIBUS":     SeriesTypeFiction,
	"EDITOR":      SeriesTypeEdit,
	"ANTHOLOGY":   SeriesTypeAnth,
	"NONFICTION":  SeriesTypeNonfic,
	"SHORTFICTION": SeriesTypeSF,
	"POEM":        SeriesTypePoem,
	"ESSAY":       SeriesTypeEssay,
	"COVERART":    SeriesTypeCoverArt,
	"INTERIORART": SeriesTypeInteriorArt,
	"REVIEW":      SeriesTypeReview,
	"INTERVIEW":   SeriesTypeInterview,
}

// seriesTypeLabel maps a series priority to its display heading.
var seriesTypeLabel = map[int]string{
	SeriesTypeFiction:     "Fiction",
	SeriesTypeEdit:        "Magazine Editor",
	SeriesTypeAnth:        "Anthology",
	SeriesTypeNonfic:      "Nonfiction",
	SeriesTypeSF:          "Short Fiction",
	SeriesTypePoem:        "Poem",
	SeriesTypeEssay:       "Essay",
	SeriesTypeCoverArt:    "Cover Art",
	SeriesTypeInteriorArt: "Interior Art",
	SeriesTypeReview:      "Review",
	SeriesTypeInterview:   "Interview",
}

// titleTypeLabel maps a title ttype to its bibliography section heading.
var titleTypeLabel = map[string]string{
	"NOVEL":       "Novels",
	"COLLECTION":  "Collections",
	"ANTHOLOGY":   "Anthologies",
	"OMNIBUS":     "Omnibus",
	"SERIAL":      "Serials",
	"SHORTFICTION": "Short Fiction",
	"ESSAY":       "Essays",
	"REVIEW":      "Reviews",
	"POEM":        "Poems",
	"EDITOR":      "Magazine Editor",
	"INTERVIEW":   "Interviews by This Author",
	"NONFICTION":  "Nonfiction",
	"INTERIORART": "Interior Art",
	"COVERART":    "Cover Art",
	"CHAPBOOK":    "Chapbooks",
}

// orderedSections is the display order for the bibliography, matching Python's
// ordered_title_types. Integers are series-type buckets; strings are flat title types.
// This single slice drives both series sections and flat title sections.
var orderedSections = []any{
	SeriesTypeFiction,
	"NOVEL", "COLLECTION", "OMNIBUS", "SERIAL",
	SeriesTypeEdit, "EDITOR",
	SeriesTypeAnth, "ANTHOLOGY",
	"CHAPBOOK",
	SeriesTypeNonfic, "NONFICTION",
	SeriesTypeSF, "SHORTFICTION",
	SeriesTypePoem, "POEM",
	SeriesTypeEssay, "ESSAY",
	SeriesTypeCoverArt, "COVERART",
	SeriesTypeInteriorArt, "INTERIORART",
	SeriesTypeReview, "REVIEW",
	SeriesTypeInterview, "INTERVIEW",
}

// ── Series struct ─────────────────────────────────────────────────────────────

// Series holds one row from the series table.
type Series struct {
	SeriesID       int
	SeriesTitle    string
	SeriesParent   sql.NullInt32
	SeriesType     sql.NullInt32
	ParentPosition sql.NullInt32
	NoteID         sql.NullInt32
}

// ── BibliographyData ──────────────────────────────────────────────────────────

// BibliographyData holds all pre-fetched data needed to render an author's
// bibliography. It is built once by LoadBibliographyData and then passed to
// the rendering functions.
type BibliographyData struct {
	// Canonical titles (title_parent=0) by this author, sorted for Summary display.
	CanonicalTitles []*Title

	// Variant titles (non-serials) indexed by parent title ID.
	VariantTitles map[int][]*Title

	// Serial variants indexed by parent title ID.
	SerialTitles map[int][]*Title

	// Parent title IDs that appear directly in at least one publication.
	ParentTitlesWithPubs map[int]bool

	// Co-author records for canonical titles, excluding the main author.
	// titleID -> []AuthorRef
	ParentAuthors map[int][]AuthorRef

	// Author records for variant titles. titleID -> []AuthorRef
	VariantAuthors map[int][]AuthorRef

	// Full series tree: seriesID -> *Series (includes top-level and sub-series).
	SeriesTree map[int]*Series

	// Parent→children relationships: parentSeriesID -> []childSeriesID
	SeriesParent map[int][]int

	// Top-level series priority: top seriesID -> SeriesType* constant
	SeriesPriority map[int]int

	// Top-level series genre flag: top seriesID -> true if at least one
	// title in the tree is genre (non-genre=No).
	SeriesGenre map[int]bool
}

// ── SQL loaders ───────────────────────────────────────────────────────────────

// sqlLoadCanonicalTitles fetches all canonical (parent=0) titles by this author
// in Summary sort order: series position first, then date, then title.
func sqlLoadCanonicalTitles(db *sql.DB, authorID int) ([]*Title, error) {
	rows, err := db.Query(
		"SELECT t.title_id, t.title_title, t.title_translator, t.title_synopsis, "+
			"t.note_id, t.series_id, t.title_seriesnum, t.title_copyright, "+
			"t.title_storylen, t.title_ttype, t.title_wikipedia, t.title_views, "+
			"t.title_parent, t.title_rating, t.title_annualviews, t.title_ctl, "+
			"t.title_language, t.title_seriesnum_2, t.title_non_genre, t.title_graphic, "+
			"t.title_nvz, t.title_jvn, t.title_content "+
			"FROM titles t "+
			"JOIN canonical_author ca ON ca.title_id = t.title_id "+
			"WHERE ca.author_id = ? AND ca.ca_status = 1 AND t.title_parent = 0 "+
			"ORDER BY "+
			"CASE WHEN t.title_seriesnum IS NULL THEN 1 ELSE 0 END, "+
			"t.title_seriesnum, t.title_seriesnum_2, "+
			"CASE WHEN t.title_copyright = '0000-00-00' THEN 1 ELSE 0 END, "+
			"t.title_copyright, t.title_title",
		authorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTitles(rows)
}

// sqlLoadVTsForAuthor fetches all variant/translation/serial titles whose parent
// is one of this author's canonical titles.
func sqlLoadVTsForAuthor(db *sql.DB, authorID int) ([]*Title, error) {
	rows, err := db.Query(
		"SELECT t.title_id, t.title_title, t.title_translator, t.title_synopsis, "+
			"t.note_id, t.series_id, t.title_seriesnum, t.title_copyright, "+
			"t.title_storylen, t.title_ttype, t.title_wikipedia, t.title_views, "+
			"t.title_parent, t.title_rating, t.title_annualviews, t.title_ctl, "+
			"t.title_language, t.title_seriesnum_2, t.title_non_genre, t.title_graphic, "+
			"t.title_nvz, t.title_jvn, t.title_content "+
			"FROM titles t "+
			"JOIN canonical_author ca ON ca.title_id = t.title_parent "+
			"WHERE ca.author_id = ? AND ca.ca_status = 1 "+
			"ORDER BY t.title_copyright, t.title_title",
		authorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTitles(rows)
}

// sqlLoadSeriesForAuthor fetches all series records that contain at least one
// title by this author.
func sqlLoadSeriesForAuthor(db *sql.DB, authorID int) ([]*Series, error) {
	rows, err := db.Query(
		"SELECT DISTINCT s.series_id, s.series_title, s.series_parent, "+
			"s.series_type, s.series_parent_position, s.series_note_id "+
			"FROM series s "+
			"JOIN titles t ON t.series_id = s.series_id "+
			"JOIN canonical_author ca ON ca.title_id = t.title_id "+
			"WHERE ca.author_id = ? "+
			"ORDER BY s.series_title",
		authorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Series
	for rows.Next() {
		var s Series
		var title sql.NullString
		if err := rows.Scan(&s.SeriesID, &title, &s.SeriesParent,
			&s.SeriesType, &s.ParentPosition, &s.NoteID); err != nil {
			return nil, err
		}
		s.SeriesTitle = title.String
		result = append(result, &s)
	}
	return result, rows.Err()
}

// sqlLoadCoAuthors fetches co-authors for a list of title IDs, excluding the
// main author. Returns titleID -> []AuthorRef.
// We drive from canonical_author (using idx_ca_title_status) then join authors
// (using idx_authors_id) so both joins are indexed point-lookups.
func sqlLoadCoAuthors(db *sql.DB, titleIDs []int, excludeAuthorID int) (map[int][]AuthorRef, error) {
	result := make(map[int][]AuthorRef)
	if len(titleIDs) == 0 {
		return result, nil
	}
	ph := make([]string, len(titleIDs))
	args := make([]any, len(titleIDs)+1)
	for i, id := range titleIDs {
		ph[i] = "?"
		args[i] = id
	}
	args[len(titleIDs)] = excludeAuthorID
	rows, err := db.Query(
		"SELECT ca.title_id, a.author_id, a.author_canonical "+
			"FROM canonical_author ca "+
			"JOIN authors a ON a.author_id = ca.author_id "+
			"WHERE ca.title_id IN ("+strings.Join(ph, ",")+ ") "+
			"AND ca.ca_status = 1 AND ca.author_id <> ? "+
			"ORDER BY a.author_lastname, a.author_canonical",
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

// sqlTitlesWithPubs returns the subset of titleIDs that appear in pub_content.
func sqlTitlesWithPubs(db *sql.DB, titleIDs []int) (map[int]bool, error) {
	result := make(map[int]bool)
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
		"SELECT DISTINCT title_id FROM pub_content WHERE title_id IN ("+strings.Join(ph, ",")+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = true
	}
	return result, rows.Err()
}

// scanTitles is a shared helper for scanning full title rows.
func scanTitles(rows *sql.Rows) ([]*Title, error) {
	var result []*Title
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
		result = append(result, &t)
	}
	return result, rows.Err()
}

// ── Assembly ──────────────────────────────────────────────────────────────────

// buildVariants splits a flat list of variant titles into two maps:
// variant_dict (non-serials) and serial_dict, both keyed by parent title ID.
// Only variants whose parent is in the canonical set are included.
func buildVariants(canonicalTitles []*Title, variants []*Title) (map[int][]*Title, map[int][]*Title) {
	parentSet := make(map[int]bool, len(canonicalTitles))
	for _, t := range canonicalTitles {
		parentSet[t.TitleID] = true
	}

	variantDict := make(map[int][]*Title)
	serialDict := make(map[int][]*Title)

	for _, v := range variants {
		if !parentSet[v.TitleParent] {
			continue
		}
		if v.TitleTType.String == "SERIAL" {
			serialDict[v.TitleParent] = append(serialDict[v.TitleParent], v)
		} else {
			variantDict[v.TitleParent] = append(variantDict[v.TitleParent], v)
		}
	}
	return variantDict, serialDict
}

// buildSeriesTree builds the full series tree data structures from a flat list
// of series records (which may include super-series not directly used by this author).
// It also loads parent series not in the initial list (walking up to the root).
func buildSeriesTree(db *sql.DB, seriesList []*Series) (
	tree map[int]*Series,
	parent map[int][]int, // parentID -> []childIDs
	err error,
) {
	tree = make(map[int]*Series)
	parent = make(map[int][]int)

	// Seed with what we have.
	for _, s := range seriesList {
		tree[s.SeriesID] = s
	}

	// Walk up to root: for each series with a parent not yet in the tree, load it.
	changed := true
	for changed {
		changed = false
		toLoad := []int{}
		for _, s := range tree {
			if s.SeriesParent.Valid {
				pid := int(s.SeriesParent.Int32)
				if _, ok := tree[pid]; !ok {
					toLoad = append(toLoad, pid)
				}
			}
		}
		if len(toLoad) == 0 {
			break
		}
		ph := make([]string, len(toLoad))
		args := make([]any, len(toLoad))
		for i, id := range toLoad {
			ph[i] = "?"
			args[i] = id
		}
		rows, err2 := db.Query(
			"SELECT series_id, series_title, series_parent, series_type, "+
				"series_parent_position, series_note_id FROM series "+
				"WHERE series_id IN ("+strings.Join(ph, ",")+")",
			args...,
		)
		if err2 != nil {
			return nil, nil, err2
		}
		for rows.Next() {
			var s Series
			var title sql.NullString
			if err2 := rows.Scan(&s.SeriesID, &title, &s.SeriesParent,
				&s.SeriesType, &s.ParentPosition, &s.NoteID); err2 != nil {
				rows.Close()
				return nil, nil, err2
			}
			s.SeriesTitle = title.String
			tree[s.SeriesID] = &s
			changed = true
		}
		rows.Close()
	}

	// Build parent->children map.
	for id, s := range tree {
		if s.SeriesParent.Valid {
			pid := int(s.SeriesParent.Int32)
			parent[pid] = append(parent[pid], id)
		}
	}

	return tree, parent, nil
}

// findTopSeries walks up the series tree to return the root (top-level) series ID.
func findTopSeries(seriesID int, tree map[int]*Series) int {
	for {
		s, ok := tree[seriesID]
		if !ok || !s.SeriesParent.Valid || s.SeriesParent.Int32 == 0 {
			return seriesID
		}
		seriesID = int(s.SeriesParent.Int32)
	}
}

// computeSeriesPriority builds the SeriesPriority and SeriesGenre maps from
// the canonical title list and series tree.
func computeSeriesPriority(canonicalTitles []*Title, tree map[int]*Series) (
	priority map[int]int,
	genre map[int]bool,
) {
	// seriesTypeContents[topSeriesID][ttype] = true
	seriesTypeContents := make(map[int]map[string]bool)
	genre = make(map[int]bool)

	for _, t := range canonicalTitles {
		if !t.SeriesID.Valid {
			continue
		}
		topID := findTopSeries(int(t.SeriesID.Int32), tree)

		if seriesTypeContents[topID] == nil {
			seriesTypeContents[topID] = make(map[string]bool)
		}
		seriesTypeContents[topID][t.TitleTType.String] = true

		if t.TitleNonGenre.String != "Yes" {
			genre[topID] = true
		}
	}

	// Assign priority using the same precedence chain as Python.
	priority = make(map[int]int)
	for topID, ttypes := range seriesTypeContents {
		switch {
		case ttypes["NOVEL"] || ttypes["COLLECTION"] || ttypes["SERIAL"] || ttypes["OMNIBUS"]:
			priority[topID] = SeriesTypeFiction
		case ttypes["EDITOR"]:
			priority[topID] = SeriesTypeEdit
		case ttypes["ANTHOLOGY"]:
			priority[topID] = SeriesTypeAnth
		case ttypes["NONFICTION"] && !ttypes["SHORTFICTION"] && !ttypes["POEM"]:
			priority[topID] = SeriesTypeNonfic
		case ttypes["NONFICTION"]:
			priority[topID] = SeriesTypeFiction
		case ttypes["OMNIBUS"]:
			priority[topID] = SeriesTypeFiction
		case ttypes["SHORTFICTION"]:
			priority[topID] = SeriesTypeSF
		case ttypes["POEM"]:
			priority[topID] = SeriesTypePoem
		case ttypes["ESSAY"]:
			priority[topID] = SeriesTypeEssay
		case ttypes["COVERART"]:
			priority[topID] = SeriesTypeCoverArt
		case ttypes["INTERIORART"]:
			priority[topID] = SeriesTypeInteriorArt
		case ttypes["REVIEW"]:
			priority[topID] = SeriesTypeReview
		case ttypes["INTERVIEW"]:
			priority[topID] = SeriesTypeInterview
		default:
			priority[topID] = SeriesTypeOther
		}
	}
	return priority, genre
}

// ── Main entry point ──────────────────────────────────────────────────────────

// LoadBibliographyData fetches and assembles all data needed to render an
// author's Summary bibliography page. It is designed to use a small number
// of batch queries rather than per-title round-trips.
func LoadBibliographyData(db *sql.DB, authorID int) (*BibliographyData, error) {
	bd := &BibliographyData{}

	// ── Step 1: canonical titles ──────────────────────────────────────────
	canonical, err := sqlLoadCanonicalTitles(db, authorID)
	if err != nil {
		return nil, err
	}
	bd.CanonicalTitles = canonical

	if len(canonical) == 0 {
		// Pseudonym with no canonical titles — return mostly empty.
		bd.VariantTitles = map[int][]*Title{}
		bd.SerialTitles = map[int][]*Title{}
		bd.ParentAuthors = map[int][]AuthorRef{}
		bd.VariantAuthors = map[int][]AuthorRef{}
		bd.ParentTitlesWithPubs = map[int]bool{}
		bd.SeriesTree = map[int]*Series{}
		bd.SeriesParent = map[int][]int{}
		bd.SeriesPriority = map[int]int{}
		bd.SeriesGenre = map[int]bool{}
		return bd, nil
	}

	canonicalIDs := make([]int, len(canonical))
	for i, t := range canonical {
		canonicalIDs[i] = t.TitleID
	}

	// ── Step 2: variant titles ────────────────────────────────────────────
	variants, err := sqlLoadVTsForAuthor(db, authorID)
	if err != nil {
		return nil, err
	}
	bd.VariantTitles, bd.SerialTitles = buildVariants(canonical, variants)

	// ── Step 3: which parent titles have direct pubs ──────────────────────
	// Collect all parent IDs referenced by variant/serial dicts.
	vtIDs := make([]int, 0)
	for _, vlist := range bd.VariantTitles {
		for _, v := range vlist {
			vtIDs = append(vtIDs, v.TitleID)
		}
	}
	for _, slist := range bd.SerialTitles {
		for _, s := range slist {
			vtIDs = append(vtIDs, s.TitleID)
		}
	}
	allParentIDs := append(canonicalIDs, vtIDs...)
	bd.ParentTitlesWithPubs, err = sqlTitlesWithPubs(db, canonicalIDs)
	if err != nil {
		return nil, err
	}

	// ── Step 4: co-author data ────────────────────────────────────────────
	bd.ParentAuthors, err = sqlLoadCoAuthors(db, canonicalIDs, authorID)
	if err != nil {
		return nil, err
	}

	if len(vtIDs) > 0 {
		bd.VariantAuthors, err = sqlLoadCoAuthors(db, vtIDs, 0)
		if err != nil {
			return nil, err
		}
	} else {
		bd.VariantAuthors = map[int][]AuthorRef{}
	}

	_ = allParentIDs // used above; keep compiler happy

	// ── Step 5: series tree ───────────────────────────────────────────────
	seriesList, err := sqlLoadSeriesForAuthor(db, authorID)
	if err != nil {
		return nil, err
	}

	bd.SeriesTree, bd.SeriesParent, err = buildSeriesTree(db, seriesList)
	if err != nil {
		return nil, err
	}

	bd.SeriesPriority, bd.SeriesGenre = computeSeriesPriority(canonical, bd.SeriesTree)

	// Sort children within each parent by (ParentPosition, SeriesTitle)
	for pid := range bd.SeriesParent {
		children := bd.SeriesParent[pid]
		sort.SliceStable(children, func(i, j int) bool {
			si := bd.SeriesTree[children[i]]
			sj := bd.SeriesTree[children[j]]
			pi, pj := int32(999999), int32(999999)
			if si.ParentPosition.Valid {
				pi = si.ParentPosition.Int32
			}
			if sj.ParentPosition.Valid {
				pj = sj.ParentPosition.Int32
			}
			if pi != pj {
				return pi < pj
			}
			return si.SeriesTitle < sj.SeriesTitle
		})
		bd.SeriesParent[pid] = children
	}

	return bd, nil
}

// sqlLoadCanonicalTitlesChrono fetches all canonical (parent=0) titles by this
// author sorted chronologically: unknown dates last, then by copyright date,
// then by title.  Used by the Chronological bibliography page.
func sqlLoadCanonicalTitlesChrono(db *sql.DB, authorID int) ([]*Title, error) {
	rows, err := db.Query(
		"SELECT t.title_id, t.title_title, t.title_translator, t.title_synopsis, "+
			"t.note_id, t.series_id, t.title_seriesnum, t.title_copyright, "+
			"t.title_storylen, t.title_ttype, t.title_wikipedia, t.title_views, "+
			"t.title_parent, t.title_rating, t.title_annualviews, t.title_ctl, "+
			"t.title_language, t.title_seriesnum_2, t.title_non_genre, t.title_graphic, "+
			"t.title_nvz, t.title_jvn, t.title_content "+
			"FROM titles t "+
			"JOIN canonical_author ca ON ca.title_id = t.title_id "+
			"WHERE ca.author_id = ? AND ca.ca_status = 1 AND t.title_parent = 0 "+
			"ORDER BY CASE WHEN t.title_copyright = '0000-00-00' THEN 1 ELSE 0 END, "+
			"t.title_copyright, t.title_title",
		authorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTitles(rows)
}

// sqlLoadCanonicalTitlesAlpha fetches all canonical (parent=0) titles by this
// author sorted alphabetically by title then copyright date.
// Used by the Alphabetical bibliography page.
func sqlLoadCanonicalTitlesAlpha(db *sql.DB, authorID int) ([]*Title, error) {
	rows, err := db.Query(
		"SELECT t.title_id, t.title_title, t.title_translator, t.title_synopsis, "+
			"t.note_id, t.series_id, t.title_seriesnum, t.title_copyright, "+
			"t.title_storylen, t.title_ttype, t.title_wikipedia, t.title_views, "+
			"t.title_parent, t.title_rating, t.title_annualviews, t.title_ctl, "+
			"t.title_language, t.title_seriesnum_2, t.title_non_genre, t.title_graphic, "+
			"t.title_nvz, t.title_jvn, t.title_content "+
			"FROM titles t "+
			"JOIN canonical_author ca ON ca.title_id = t.title_id "+
			"WHERE ca.author_id = ? AND ca.ca_status = 1 AND t.title_parent = 0 "+
			"ORDER BY t.title_title COLLATE NOCASE, t.title_copyright",
		authorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTitles(rows)
}

// ── Series-page helpers ───────────────────────────────────────────────────────

// SQLLoadSeries loads a single series record by ID.
func SQLLoadSeries(db *sql.DB, id int) (*Series, error) {
	var s Series
	var title sql.NullString
	err := db.QueryRow(
		"SELECT series_id, series_title, series_parent, series_type, "+
			"series_parent_position, series_note_id FROM series WHERE series_id=?", id,
	).Scan(&s.SeriesID, &title, &s.SeriesParent, &s.SeriesType, &s.ParentPosition, &s.NoteID)
	if err != nil {
		return nil, err
	}
	s.SeriesTitle = title.String
	return &s, nil
}

// SQLFindSeriesChildren returns the IDs of direct child series of parentID,
// ordered by (parent_position nulls-last, series_title), matching Python's
// SQLFindSeriesChildren ORDER BY clause.
func SQLFindSeriesChildren(db *sql.DB, parentID int) ([]int, error) {
	rows, err := db.Query(
		"SELECT series_id FROM series WHERE series_parent=? "+
			"ORDER BY CASE WHEN series_parent_position IS NULL OR series_parent_position='' THEN 1 ELSE 0 END, "+
			"series_parent_position, series_title",
		parentID,
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

// BuildSeriesPageTree walks DOWN from the root series, loading every node in
// the sub-tree.  It returns:
//   - seriesData: seriesID → *Series for every node
//   - childrenMap: seriesID → ordered slice of child IDs
//   - allIDs: every series ID in the tree (BFS order)
func BuildSeriesPageTree(db *sql.DB, rootID int) (
	seriesData map[int]*Series,
	childrenMap map[int][]int,
	allIDs []int,
	err error,
) {
	seriesData = make(map[int]*Series)
	childrenMap = make(map[int][]int)

	root, err := SQLLoadSeries(db, rootID)
	if err != nil {
		return
	}
	seriesData[rootID] = root

	queue := []int{rootID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		allIDs = append(allIDs, id)

		children, err2 := SQLFindSeriesChildren(db, id)
		if err2 != nil {
			err = err2
			return
		}
		childrenMap[id] = children

		for _, childID := range children {
			if _, seen := seriesData[childID]; seen {
				continue
			}
			child, err2 := SQLLoadSeries(db, childID)
			if err2 != nil {
				err = err2
				return
			}
			seriesData[childID] = child
			queue = append(queue, childID)
		}
	}
	return
}

// SQLLoadSeriesListTitles returns the canonical titles for a set of series IDs,
// grouped by series_id (map) and also as a flat list.  Titles are ordered by
// series_id, series-number (NULLs last), series-number-2, copyright.
// This matches Python's SQLLoadSeriesListTitles.
func SQLLoadSeriesListTitles(db *sql.DB, seriesIDs []int) (map[int][]*Title, []*Title, error) {
	if len(seriesIDs) == 0 {
		return nil, nil, nil
	}
	ph := make([]string, len(seriesIDs))
	args := make([]any, len(seriesIDs))
	for i, id := range seriesIDs {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT title_id, title_title, title_translator, title_synopsis, note_id, series_id, "+
			"title_seriesnum, title_copyright, title_storylen, title_ttype, title_wikipedia, "+
			"title_views, title_parent, title_rating, title_annualviews, title_ctl, "+
			"title_language, title_seriesnum_2, title_non_genre, title_graphic, title_nvz, "+
			"title_jvn, title_content "+
			"FROM titles WHERE series_id IN ("+strings.Join(ph, ",") +") "+
			"ORDER BY series_id, "+
			"CASE WHEN title_seriesnum IS NULL THEN 1 ELSE 0 END, "+
			"title_seriesnum, title_seriesnum_2, title_copyright",
		args...,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	bySeriesID := make(map[int][]*Title)
	var flat []*Title
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
			return nil, nil, err
		}
		ptr := &t
		if t.SeriesID.Valid {
			sid := int(t.SeriesID.Int32)
			bySeriesID[sid] = append(bySeriesID[sid], ptr)
		}
		flat = append(flat, ptr)
	}
	return bySeriesID, flat, rows.Err()
}

// sqlLoadVTsForTitleList fetches all variant/translation/serial titles whose
// parent title_id is in the given list.  Used by the series page to load
// variant data for all canonical titles in the series tree.
func sqlLoadVTsForTitleList(db *sql.DB, parentIDs []int) ([]*Title, error) {
	if len(parentIDs) == 0 {
		return nil, nil
	}
	ph := make([]string, len(parentIDs))
	args := make([]any, len(parentIDs))
	for i, id := range parentIDs {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT title_id, title_title, title_translator, title_synopsis, note_id, series_id, "+
			"title_seriesnum, title_copyright, title_storylen, title_ttype, title_wikipedia, "+
			"title_views, title_parent, title_rating, title_annualviews, title_ctl, "+
			"title_language, title_seriesnum_2, title_non_genre, title_graphic, title_nvz, "+
			"title_jvn, title_content "+
			"FROM titles WHERE title_parent IN ("+strings.Join(ph, ",") +") "+
			"ORDER BY title_copyright, title_title",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTitles(rows)
}

// SQLFindSeriesTitlesByID returns all titles that belong directly to a series,
// ordered by series number (NULLs last), secondary number, then copyright date.
// This is the series-grid equivalent of SQLFindSeriesTitles in Python (which
// searches by name; we use ID for correctness and efficiency).
func SQLFindSeriesTitlesByID(db *sql.DB, seriesID int) ([]*Title, error) {
	const q = `
	SELECT title_id, title_title, title_translator, title_synopsis, note_id,
	       series_id, title_seriesnum, title_copyright, title_storylen,
	       title_ttype, title_wikipedia, title_views,
	       title_parent, title_rating, title_annualviews,
	       title_ctl, title_language, title_seriesnum_2,
	       title_non_genre, title_graphic, title_nvz,
	       title_jvn, title_content
	FROM titles
	WHERE series_id = ?
	ORDER BY CASE WHEN title_seriesnum IS NULL THEN 1 ELSE 0 END,
	         CAST(title_seriesnum AS REAL), title_seriesnum_2, title_copyright`
	rows, err := db.Query(q, seriesID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTitles(rows)
}

// SQLBatchVerificationStatus returns the verification status for each pub ID:
//   1 = has a primary verification
//   2 = has at least one active secondary verification (ver_status=1)
//   0 = unverified
func SQLBatchVerificationStatus(db *sql.DB, pubIDs []int) (map[int]int, error) {
	result := make(map[int]int, len(pubIDs))
	if len(pubIDs) == 0 {
		return result, nil
	}

	ph := make([]string, len(pubIDs))
	args := make([]any, len(pubIDs))
	for i, id := range pubIDs {
		ph[i] = "?"
		args[i] = id
	}
	inClause := strings.Join(ph, ",")

	// Primary verifications (status = 1)
	rows, err := db.Query(
		"SELECT DISTINCT pub_id FROM primary_verifications WHERE pub_id IN ("+inClause+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = 1
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Active secondary verifications (ver_status=1) — only for pubs not already primary
	rows2, err := db.Query(
		"SELECT DISTINCT pub_id FROM verification WHERE ver_status=1 AND pub_id IN ("+inClause+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var id int
		if err := rows2.Scan(&id); err != nil {
			return nil, err
		}
		if result[id] == 0 {
			result[id] = 2
		}
	}
	return result, rows2.Err()
}

// SearchSeriesResult holds a series record for search result display.
type SearchSeriesResult struct {
	SeriesID       int
	SeriesTitle    string
	ParentID       int    // 0 if no parent
	ParentTitle    string // "" if no parent
	ParentPosition sql.NullString
}

// SQLFindSeries searches for series by title.
func SQLFindSeries(db *sql.DB, target string, exact bool) ([]*SearchSeriesResult, error) {
	var rows *sql.Rows
	var err error
	if exact {
		rows, err = db.Query(
			"SELECT series_id, series_title, COALESCE(series_parent,0), series_parent_position FROM series WHERE series_title = ? ORDER BY series_title",
			target,
		)
	} else {
		like := "%" + target + "%"
		rows, err = db.Query(`
			SELECT * FROM (
				SELECT DISTINCT s.series_id, s.series_title, COALESCE(s.series_parent,0), s.series_parent_position
				FROM series s WHERE s.series_title LIKE ?
				UNION
				SELECT DISTINCT s.series_id, s.series_title, COALESCE(s.series_parent,0), s.series_parent_position
				FROM series s
				JOIN trans_series ts ON ts.series_id = s.series_id
				WHERE ts.trans_series_name LIKE ?
			) ORDER BY series_title`,
			like, like,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*SearchSeriesResult
	for rows.Next() {
		var r SearchSeriesResult
		if err := rows.Scan(&r.SeriesID, &r.SeriesTitle, &r.ParentID, &r.ParentPosition); err != nil {
			return nil, err
		}
		results = append(results, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Batch-load parent series titles.
	parentIDs := map[int]bool{}
	for _, r := range results {
		if r.ParentID != 0 {
			parentIDs[r.ParentID] = true
		}
	}
	if len(parentIDs) > 0 {
		ids := make([]int, 0, len(parentIDs))
		for id := range parentIDs {
			ids = append(ids, id)
		}
		ph := make([]string, len(ids))
		args := make([]any, len(ids))
		for i, id := range ids {
			ph[i] = "?"
			args[i] = id
		}
		prows, err := db.Query(
			"SELECT series_id, series_title FROM series WHERE series_id IN ("+strings.Join(ph, ",")+")",
			args...,
		)
		if err == nil {
			defer prows.Close()
			parentNames := map[int]string{}
			for prows.Next() {
				var id int
				var name string
				if err := prows.Scan(&id, &name); err == nil {
					parentNames[id] = name
				}
			}
			for _, r := range results {
				if r.ParentID != 0 {
					r.ParentTitle = parentNames[r.ParentID]
				}
			}
		}
	}
	return results, nil
}

// MagazineSearchResult holds one entry in a magazine search result.
type MagazineSearchResult struct {
	DisplayTitle string // title shown in the link (may differ from series title)
	SeriesID     int
	SeriesTitle  string // actual series title (for asterisk detection)
	ParentID     int    // 0 if no parent
	ParentTitle  string
}

// SQLFindMagazine searches for magazine series matching the given target.
// It returns a deduplicated slice of results sorted by display title.
func SQLFindMagazine(db *sql.DB, target string) ([]*MagazineSearchResult, error) {
	like := "%" + target + "%"

	// Step 1: matching series titles.
	rows, err := db.Query(`
		SELECT DISTINCT s.series_id, s.series_title, COALESCE(s.series_parent,0)
		FROM series s
		JOIN titles t ON t.series_id = s.series_id AND t.title_ttype = 'EDITOR'
		WHERE s.series_title LIKE ?
		UNION
		SELECT DISTINCT s.series_id, s.series_title, COALESCE(s.series_parent,0)
		FROM series s
		JOIN titles t ON t.series_id = s.series_id AND t.title_ttype = 'EDITOR'
		JOIN trans_series ts ON ts.series_id = s.series_id
		WHERE ts.trans_series_name LIKE ?`,
		like, like,
	)
	if err != nil {
		return nil, err
	}
	seenIDs := map[int]bool{}
	byID := map[int]*MagazineSearchResult{}
	var order []string // display title order
	for rows.Next() {
		var r MagazineSearchResult
		if err := rows.Scan(&r.SeriesID, &r.SeriesTitle, &r.ParentID); err != nil {
			rows.Close()
			return nil, err
		}
		r.DisplayTitle = r.SeriesTitle
		if !seenIDs[r.SeriesID] {
			seenIDs[r.SeriesID] = true
			byID[r.SeriesID] = &r
			order = append(order, r.DisplayTitle)
		}
	}
	rows.Close()

	// Step 2: magazine titles that match but whose series titles don't.
	rows2, err := db.Query(`
		SELECT DISTINCT t.title_title, s.series_id, s.series_title, COALESCE(s.series_parent,0)
		FROM series s
		JOIN titles t ON t.series_id = s.series_id AND t.title_ttype = 'EDITOR'
		WHERE t.title_title LIKE ? AND s.series_title NOT LIKE ?
		UNION
		SELECT DISTINCT t.title_title, s.series_id, s.series_title, COALESCE(s.series_parent,0)
		FROM series s
		JOIN titles t ON t.series_id = s.series_id AND t.title_ttype = 'EDITOR'
		JOIN trans_titles tt ON tt.title_id = t.title_id
		WHERE tt.trans_title_title LIKE ? AND s.series_title NOT LIKE ?`,
		like, like, like, like,
	)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var titleTitle, seriesTitle string
			var seriesID, parentID int
			if err := rows2.Scan(&titleTitle, &seriesID, &seriesTitle, &parentID); err != nil {
				continue
			}
			if seenIDs[seriesID] {
				continue
			}
			// Strip trailing " - date" suffix from title title.
			if i := strings.LastIndex(titleTitle, " - "); i != -1 {
				titleTitle = titleTitle[:i]
			}
			seenIDs[seriesID] = true
			r := &MagazineSearchResult{
				DisplayTitle: titleTitle,
				SeriesID:     seriesID,
				SeriesTitle:  seriesTitle,
				ParentID:     parentID,
			}
			byID[seriesID] = r
			order = append(order, titleTitle)
		}
	}

	// Sort by display title (case-insensitive).
	sort.Slice(order, func(i, j int) bool {
		return strings.ToLower(order[i]) < strings.ToLower(order[j])
	})

	// Build deduped result slice (order may have dups from the two queries — just iterate byID in title order).
	seen2 := map[int]bool{}
	var results []*MagazineSearchResult
	for _, title := range order {
		for _, r := range byID {
			if r.DisplayTitle == title && !seen2[r.SeriesID] {
				seen2[r.SeriesID] = true
				results = append(results, r)
			}
		}
	}

	// Batch-load parent series titles.
	parentIDs := map[int]bool{}
	for _, r := range results {
		if r.ParentID != 0 {
			parentIDs[r.ParentID] = true
		}
	}
	if len(parentIDs) > 0 {
		ids := make([]int, 0, len(parentIDs))
		for id := range parentIDs {
			ids = append(ids, id)
		}
		ph := make([]string, len(ids))
		args := make([]any, len(ids))
		for i, id := range ids {
			ph[i] = "?"
			args[i] = id
		}
		prows, err := db.Query(
			"SELECT series_id, series_title FROM series WHERE series_id IN ("+strings.Join(ph, ",")+")",
			args...,
		)
		if err == nil {
			defer prows.Close()
			parentNames := map[int]string{}
			for prows.Next() {
				var id int
				var name string
				if err := prows.Scan(&id, &name); err == nil {
					parentNames[id] = name
				}
			}
			for _, r := range results {
				if r.ParentID != 0 {
					r.ParentTitle = parentNames[r.ParentID]
				}
			}
		}
	}
	return results, nil
}
