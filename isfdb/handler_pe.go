// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
)

// PeHandler serves /pe.cgi?series_id — the title-series page.
// It matches Python's pe.py / seriesClass.py output.
func PeHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) == 0 {
		http.Error(w, "Usage: /pe.cgi?series_id", http.StatusBadRequest)
		return
	}

	seriesID, err := strconv.Atoi(params[0])
	if err != nil || seriesID <= 0 {
		http.Error(w, "Invalid series ID", http.StatusBadRequest)
		return
	}

	// ── Load series root ──────────────────────────────────────────────────
	ser, err := SQLLoadSeries(DB, seriesID)
	if err != nil {
		http.Error(w, "Series not found", http.StatusNotFound)
		log.Println(err)
		return
	}

	// ── Load parent series name (for "Sub-series of:" link) ───────────────
	parentName := ""
	parentID := 0
	if ser.SeriesParent.Valid && ser.SeriesParent.Int32 > 0 {
		parentID = int(ser.SeriesParent.Int32)
		ps, err2 := SQLLoadSeries(DB, parentID)
		if err2 == nil {
			parentName = ps.SeriesTitle
		}
	}

	// ── Load note ─────────────────────────────────────────────────────────
	noteText := ""
	if ser.NoteID.Valid && ser.NoteID.Int32 > 0 {
		noteText, err = SQLgetNotes(DB, int(ser.NoteID.Int32))
		if err != nil {
			log.Println(err)
		}
	}

	// ── Load webpages ─────────────────────────────────────────────────────
	webpages, err := SQLloadSeriesWebpages(DB, seriesID)
	if err != nil {
		log.Println(err)
	}
	domains, err := SQLLoadRecognizedDomains(DB)
	if err != nil {
		log.Println(err)
	}

	// ── Build series tree (walk down from root) ───────────────────────────
	seriesData, childrenMap, allSeriesIDs, err := BuildSeriesPageTree(DB, seriesID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// ── Load all canonical titles in the tree ─────────────────────────────
	seriesTitles, flatTitles, err := SQLLoadSeriesListTitles(DB, allSeriesIDs)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Collect all canonical title IDs
	canonicalIDs := make([]int, 0, len(flatTitles))
	for _, t := range flatTitles {
		canonicalIDs = append(canonicalIDs, t.TitleID)
	}

	// ── Load variant titles for all canonical titles ───────────────────────
	variants, err := sqlLoadVTsForTitleList(DB, canonicalIDs)
	if err != nil {
		log.Println(err)
	}

	// Collect variant IDs
	variantIDs := make([]int, 0, len(variants))
	for _, v := range variants {
		variantIDs = append(variantIDs, v.TitleID)
	}

	// ── Build variant / serial maps ───────────────────────────────────────
	variantDict, serialDict := buildVariants(flatTitles, variants)

	// ── Load parent (canonical) authors — all ca_status=1, no exclusions ──
	parentAuthors, err := sqlLoadCoAuthors(DB, canonicalIDs, 0)
	if err != nil {
		log.Println(err)
		parentAuthors = map[int][]AuthorRef{}
	}

	// ── Load variant authors ───────────────────────────────────────────────
	variantAuthors, err := sqlLoadCoAuthors(DB, variantIDs, 0)
	if err != nil {
		log.Println(err)
		variantAuthors = map[int][]AuthorRef{}
	}

	// ── Determine which parent titles have pubs ───────────────────────────
	allVTIDs := variantIDs
	for _, t := range flatTitles {
		allVTIDs = append(allVTIDs, t.TitleID)
	}
	parentsWithPubs, err := sqlTitlesWithPubs(DB, allVTIDs)
	if err != nil {
		log.Println(err)
		parentsWithPubs = map[int]bool{}
	}

	// ── Assemble the display-context BibliographyData ─────────────────────
	// Only the variant-display fields are needed; author-biblio fields are left nil.
	bd := &BibliographyData{
		ParentAuthors:       parentAuthors,
		VariantTitles:       variantDict,
		SerialTitles:        serialDict,
		ParentTitlesWithPubs: parentsWithPubs,
		VariantAuthors:      variantAuthors,
	}

	// ── Detect any EDITOR title in the entire tree (for Issue Grid link) ──
	hasEditorInTree := false
	for _, t := range flatTitles {
		if t.TitleTType.String == "EDITOR" {
			hasEditorInTree = true
			break
		}
	}

	// ── Render ────────────────────────────────────────────────────────────
	pageTitle := "Series: " + ser.SeriesTitle
	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "series", "", "")

	// ── ContentBox 1: metadata ────────────────────────────────────────────
	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintln(w, `<ul>`)
	fmt.Fprintf(w, "<li><b>Series: </b>%s\n", ISFDBText(ser.SeriesTitle))

	if parentID > 0 && parentName != "" {
		fmt.Fprintf(w, "<li><b>Sub-series of:</b> <a href=\"/pe.cgi?%d\">%s</a>\n",
			parentID, ISFDBText(parentName))
	}

	PrintWebPages(w, webpages, "<li>", domains)

	if noteText != "" {
		fmt.Fprintln(w, "<li>")
		fmt.Fprintln(w, FormatNote(noteText, "Note", "short", seriesID, "Series", false))
	}

	fmt.Fprintln(w, `</ul>`)
	fmt.Fprintln(w, `</div>`)

	// ── ContentBox 2: series tree ─────────────────────────────────────────
	fmt.Fprintln(w, `<div class="ContentBox">`)
	if len(flatTitles) == 0 {
		fmt.Fprintln(w, `<p><b>This series is empty and will be deleted.</b>`)
	} else {
		fmt.Fprintln(w, `<ul>`)
		printSeriesPageNode(w, ser, seriesTitles, childrenMap, seriesData, bd, hasEditorInTree)
		fmt.Fprintln(w, `</ul>`)
	}
	fmt.Fprintln(w, `</div>`)

	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}

// printSeriesPageNode renders one node of the series tree: the series header
// link, titles for this series, then recurses into children.
// Matches Python's printSeries().
func printSeriesPageNode(w io.Writer, ser *Series,
	seriesTitles map[int][]*Title,
	childrenMap map[int][]int,
	seriesData map[int]*Series,
	bd *BibliographyData,
	hasEditorInTree bool,
) {
	// Series header line
	pos := ""
	if ser.ParentPosition.Valid && ser.ParentPosition.Int32 > 0 {
		pos = fmt.Sprintf("%d ", ser.ParentPosition.Int32)
	}
	fmt.Fprintf(w, "<li>%s<a href=\"/pe.cgi?%d\">%s</a>\n",
		pos, ser.SeriesID, ISFDBText(ser.SeriesTitle))

	// Issue grid link if any EDITOR title exists anywhere in the tree
	if hasEditorInTree {
		fmt.Fprintf(w, "<a href=\"/seriesgrid.cgi?%d\">(View Issue Grid)</a>\n", ser.SeriesID)
	}

	// Titles directly in this series
	fmt.Fprintln(w, `<ul>`)
	for _, t := range seriesTitles[ser.SeriesID] {
		numStr := ""
		if t.TitleSeriesNum.Valid {
			numStr = fmt.Sprintf("%d", t.TitleSeriesNum.Int32)
			if t.TitleSeriesNum2.Valid && t.TitleSeriesNum2.String != "" {
				numStr += "." + t.TitleSeriesNum2.String
			}
		}
		fmt.Fprintf(w, "<li>%s\n", numStr)
		coAuthors := bd.ParentAuthors[t.TitleID]
		displayMainTitle(w, t, 0, coAuthors, 0, 0, false)
		displayVariants(w, t, bd, 0, 0)
	}
	fmt.Fprintln(w, `</ul>`)

	// Recurse into child series
	children := childrenMap[ser.SeriesID]
	if len(children) > 0 {
		fmt.Fprintln(w, `<ul>`)
		for _, childID := range children {
			if child, ok := seriesData[childID]; ok {
				printSeriesPageNode(w, child, seriesTitles, childrenMap, seriesData, bd, hasEditorInTree)
			}
		}
		fmt.Fprintln(w, `</ul>`)
	}
}
