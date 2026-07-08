// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// conditionOptions are the valid book condition values in descending order.
var conditionOptions = []string{
	"", "As New", "Fine", "Near Fine", "Very Good", "Good", "Fair", "Poor",
}

// collectionNavCtx returns a NavContext for collection pages.
func collectionNavCtx(itemID int) NavContext {
	return NavContext{ShowCollection: true, CollectionItemID: itemID}
}

// ── collection_new.cgi ───────────────────────────────────────────────────────

// CollectionNewHandler serves /collection_new.cgi?PUB_ID —
// presents a form for adding a publication to the collection.
func CollectionNewHandler(w http.ResponseWriter, r *http.Request) {
	pubID, err := strconv.Atoi(r.URL.RawQuery)
	if err != nil || pubID <= 0 {
		http.Error(w, "Invalid publication ID", http.StatusBadRequest)
		return
	}

	pub, err := SQLloadPubData(DB, pubID)
	if err != nil {
		http.Error(w, "Publication not found", http.StatusNotFound)
		return
	}

	HTMLheader(w, "Add to Collection")
	PrintNavbar(w, "publication", "", "", NavContext{CollectionPubID: pubID, ShowCollection: true})

	fmt.Fprintln(w, `<div class="ContentBoxEdit">`)
	fmt.Fprintf(w, "<b>Publication:</b> <a href=\"/pub.cgi?%d\">%s</a><p>\n",
		pubID, ISFDBText(pub.PubTitle.String))

	fmt.Fprintf(w, "<form method=\"post\" action=\"%s://%s/collection_submitnew.cgi\">\n",
		PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<input type=\"hidden\" name=\"pub_id\" value=\"%d\">\n", pubID)

	fmt.Fprintln(w, `<table class="generic_table">`)
	printCollectionFormFields(w, collectionFormValues{})
	fmt.Fprintln(w, `</table><p>`)

	fmt.Fprintln(w, `<input type="submit" value="Add to Collection">`)
	fmt.Fprintln(w, `</form>`)
	fmt.Fprintln(w, `</div>`)

	HTMLtrailer(w)
}

// ── collection_submitnew.cgi ─────────────────────────────────────────────────

// CollectionSubmitNewHandler serves POST /collection_submitnew.cgi —
// inserts a new collection record and redirects to the collection list.
func CollectionSubmitNewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form data", http.StatusBadRequest)
		return
	}

	pubID, err := strconv.Atoi(r.FormValue("pub_id"))
	if err != nil || pubID <= 0 {
		http.Error(w, "Invalid publication ID", http.StatusBadRequest)
		return
	}

	// Check for duplicate.
	var existing int
	err = UserDB.QueryRow("SELECT COUNT(*) FROM collection WHERE pub_id = ?", pubID).Scan(&existing)
	if err == nil && existing > 0 {
		pub, _ := SQLloadPubData(DB, pubID)
		pubTitle := ""
		if pub != nil {
			pubTitle = pub.PubTitle.String
		}
		HTMLheader(w, "Already in Collection")
		PrintNavbar(w, "publication", "", "", NavContext{CollectionPubID: pubID, ShowCollection: true})
		fmt.Fprintln(w, `<div class="ContentBoxEdit">`)
		fmt.Fprintf(w, "<p><b>Error:</b> <a href=\"/pub.cgi?%d\">%s</a> is already in your collection.</p>\n",
			pubID, ISFDBText(pubTitle))
		fmt.Fprintf(w, "<p><a href=\"/collection_list.cgi\">View your collection</a></p>\n")
		fmt.Fprintln(w, `</div>`)
		HTMLtrailer(w)
		return
	}

	vals := collectionFormValues{
		AcqDate:    sanitizeDate(r.FormValue("col_acq_date")),
		SaleDate:   sanitizeDate(r.FormValue("col_sale_date")),
		Cond:       sanitizeChoice(r.FormValue("col_cond"), conditionOptions),
		Signature:  sanitizeYN(r.FormValue("col_signature")),
		Marginalia: sanitizeYN(r.FormValue("col_marginalia")),
		Source:     strings.TrimSpace(r.FormValue("col_source")),
		PrchPrice:  strings.TrimSpace(r.FormValue("col_prch_price")),
		InsValue:   strings.TrimSpace(r.FormValue("col_ins_value")),
		Location:   strings.TrimSpace(r.FormValue("col_location")),
		Note:       strings.TrimSpace(r.FormValue("col_note")),
	}

	_, err = UserDB.Exec(`
		INSERT INTO collection
			(pub_id, col_acq_date, col_sale_date, col_cond, col_signature,
			 col_marginalia, col_source, col_prch_price, col_ins_value, col_location, col_note)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pubID, vals.AcqDate, vals.SaleDate, vals.Cond, vals.Signature,
		vals.Marginalia, vals.Source, vals.PrchPrice, vals.InsValue, vals.Location, vals.Note)
	if err != nil {
		HTMLheader(w, "Collection Error")
		PrintNavbar(w, "collection", "", "", collectionNavCtx(0))
		fmt.Fprintf(w, "<div class=\"ContentBoxEdit\"><p>Error saving to collection: %s</p></div>\n",
			ISFDBText(err.Error()))
		HTMLtrailer(w)
		return
	}

	http.Redirect(w, r, "/collection_list.cgi", http.StatusSeeOther)
}

// ── collection_list.cgi ──────────────────────────────────────────────────────

// collectionListItem is one row returned by the list/search queries.
type collectionListItem struct {
	ColID      int
	PubID      int
	PubTitle   string
	PubAuthors []AuthorRef
	AuthorLabel string // "Author" or "Editor"
	PubYear    string
	PubISBN    string
	PubCatalog string
	AcqDate    string
	SaleDate   string
	Cond       string
	Signature  string
	Marginalia string
	Source     string
	Location   string
	Note       string
}

// enrichCollectionItems fills PubTitle, PubAuthors, AuthorLabel, PubYear,
// PubISBN, and PubCatalog for each item using batch lookups.
func enrichCollectionItems(items []collectionListItem) []collectionListItem {
	pubIDs := make([]int, len(items))
	for i, it := range items {
		pubIDs[i] = it.PubID
	}
	authorMap, err := SQLPubAuthorsBatch(DB, pubIDs)
	if err != nil {
		authorMap = map[int][]AuthorRef{}
	}
	for i, it := range items {
		pub, err := SQLloadPubData(DB, it.PubID)
		if err == nil && pub != nil {
			items[i].PubTitle = pub.PubTitle.String
			items[i].PubYear = ISFDBconvertDate(pub.PubYear.String, 1)
			items[i].PubISBN = pub.PubISBN.String
			items[i].PubCatalog = pub.PubCatalog.String
			switch pub.PubCType.String {
			case "ANTHOLOGY", "MAGAZINE", "FANZINE":
				items[i].AuthorLabel = "Editor"
			default:
				items[i].AuthorLabel = "Author"
			}
		} else {
			items[i].PubTitle = fmt.Sprintf("(pub #%d)", it.PubID)
			items[i].AuthorLabel = "Author"
		}
		items[i].PubAuthors = authorMap[it.PubID]
	}
	return items
}

// printCollectionTable renders the shared results table used by list and slist.
func printCollectionTable(w http.ResponseWriter, items []collectionListItem) {
	fmt.Fprintln(w, `<table class="collection_table">`)
	fmt.Fprintln(w, `<tr class="generic_table_header">`)
	for _, hdr := range []string{"#", "Publication", "Author/Editor", "Year", "ISBN/Catalog", "Acquired", "Sold", "Cond", "Signed", "Marg.", "Source", "Location", "Note"} {
		fmt.Fprintf(w, "<th>%s</th>", hdr)
	}
	fmt.Fprintln(w, `</tr>`)

	for i, it := range items {
		rowClass := "table1"
		if i%2 == 1 {
			rowClass = "table2"
		}
		fmt.Fprintf(w, "<tr class=\"%s\">\n", rowClass)
		fmt.Fprintf(w, "<td><a href=\"/collection_view.cgi?%d\">%d</a></td>\n", it.ColID, it.ColID)
		fmt.Fprintf(w, "<td><a href=\"/pub.cgi?%d\">%s</a></td>\n", it.PubID, ISFDBText(it.PubTitle))

		// Author/Editor column
		fmt.Fprint(w, "<td>")
		if len(it.PubAuthors) > 0 {
			DisplayPersons(w, it.PubAuthors)
		}
		fmt.Fprintln(w, "</td>")

		// Year
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(it.PubYear))

		// ISBN / Catalog
		isbnCat := it.PubISBN
		if isbnCat == "" {
			isbnCat = it.PubCatalog
		}
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(isbnCat))

		acq := it.AcqDate
		if acq == "0000-00-00" {
			acq = ""
		}
		sale := it.SaleDate
		if sale == "0000-00-00" {
			sale = ""
		}
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(acq))
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(sale))
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(it.Cond))
		signed := ""
		if it.Signature == "y" {
			signed = "Yes"
		}
		fmt.Fprintf(w, "<td>%s</td>\n", signed)
		marg := ""
		if it.Marginalia == "y" {
			marg = "Yes"
		}
		fmt.Fprintf(w, "<td>%s</td>\n", marg)
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(truncate(it.Source, 10)))
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(truncate(it.Location, 10)))
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(truncate(it.Note, 10)))
		fmt.Fprintln(w, `</tr>`)
	}

	fmt.Fprintln(w, `</table>`)
}

// truncate returns at most n runes of s.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// CollectionListHandler serves /collection_list.cgi —
// table of all collection items, most recently added first, max 200.
func CollectionListHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := UserDB.Query(`
		SELECT col_id, pub_id, col_acq_date, col_sale_date, col_cond,
		       col_signature, col_marginalia, col_source, col_location, col_note
		FROM collection
		ORDER BY col_id DESC
		LIMIT 200`)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var items []collectionListItem
	for rows.Next() {
		var it collectionListItem
		if err := rows.Scan(&it.ColID, &it.PubID, &it.AcqDate, &it.SaleDate,
			&it.Cond, &it.Signature, &it.Marginalia,
			&it.Source, &it.Location, &it.Note); err != nil {
			continue
		}
		items = append(items, it)
	}
	items = enrichCollectionItems(items)

	HTMLheader(w, "My Collection")
	PrintNavbar(w, "collection", "", "", NavContext{ShowCollection: true})

	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintf(w, "<b>My Collection</b> (%d item(s))<p>\n", len(items))

	if len(items) == 0 {
		fmt.Fprintln(w, "<p>Your collection is empty.</p>")
		fmt.Fprintln(w, `</div>`)
		HTMLtrailer(w)
		return
	}

	printCollectionTable(w, items)
	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}

// ── collection_view.cgi ──────────────────────────────────────────────────────

// CollectionViewHandler serves /collection_view.cgi?<col_id> —
// shows the full detail of a single collection item.
func CollectionViewHandler(w http.ResponseWriter, r *http.Request) {
	colID, err := strconv.Atoi(r.URL.RawQuery)
	if err != nil || colID <= 0 {
		http.Error(w, "Invalid collection ID", http.StatusBadRequest)
		return
	}

	var it collectionListItem
	it.ColID = colID
	err = UserDB.QueryRow(`
		SELECT pub_id, col_acq_date, col_sale_date, col_cond,
		       col_signature, col_marginalia, col_source, col_location, col_note
		FROM collection WHERE col_id = ?`, colID).Scan(
		&it.PubID, &it.AcqDate, &it.SaleDate, &it.Cond,
		&it.Signature, &it.Marginalia, &it.Source, &it.Location, &it.Note)
	if err != nil {
		http.Error(w, "Collection item not found", http.StatusNotFound)
		return
	}

	pub, err := SQLloadPubData(DB, it.PubID)
	if err == nil && pub != nil {
		it.PubTitle = pub.PubTitle.String
	} else {
		it.PubTitle = fmt.Sprintf("(pub #%d)", it.PubID)
	}

	HTMLheader(w, "Collection Item")
	PrintNavbar(w, "collection", "", "", NavContext{ShowCollection: true, CollectionItemID: colID})

	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintf(w, "<b>Collection Item #%d</b><p>\n", colID)

	fmt.Fprintln(w, `<table class="generic_table">`)

	rowN := 0
	row := func(label, val string) {
		cls := "table1"
		if rowN%2 == 1 {
			cls = "table2"
		}
		rowN++
		fmt.Fprintf(w, "<tr class=\"%s\"><td><b>%s</b></td><td>%s</td></tr>\n", cls, label, val)
	}

	cls := "table1"
	rowN++
	fmt.Fprintf(w, "<tr class=\"%s\"><td><b>Publication</b></td><td><a href=\"/pub.cgi?%d\">%s</a></td></tr>\n",
		cls, it.PubID, ISFDBText(it.PubTitle))
	row("Acquisition Date", ISFDBText(it.AcqDate))
	row("Sale Date", ISFDBText(it.SaleDate))
	row("Condition", ISFDBText(it.Cond))
	signed := "No"
	if it.Signature == "y" {
		signed = "Yes"
	}
	row("Signed", signed)
	marg := "No"
	if it.Marginalia == "y" {
		marg = "Yes"
	}
	row("Marginalia", marg)
	row("Source", ISFDBText(it.Source))
	row("Location", ISFDBText(it.Location))
	row("Note", ISFDBText(it.Note))

	fmt.Fprintln(w, `</table>`)
	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}

// ── collection_edit.cgi ──────────────────────────────────────────────────────

// CollectionEditHandler serves /collection_edit.cgi?<col_id> —
// pre-filled edit form for an existing collection item.
func CollectionEditHandler(w http.ResponseWriter, r *http.Request) {
	colID, err := strconv.Atoi(r.URL.RawQuery)
	if err != nil || colID <= 0 {
		http.Error(w, "Invalid collection ID", http.StatusBadRequest)
		return
	}

	var pubID int
	var vals collectionFormValues
	err = UserDB.QueryRow(`
		SELECT pub_id, col_acq_date, col_sale_date, col_cond,
		       col_signature, col_marginalia, col_source, col_prch_price,
		       col_ins_value, col_location, col_note
		FROM collection WHERE col_id = ?`, colID).Scan(
		&pubID, &vals.AcqDate, &vals.SaleDate, &vals.Cond,
		&vals.Signature, &vals.Marginalia, &vals.Source, &vals.PrchPrice,
		&vals.InsValue, &vals.Location, &vals.Note)
	if err != nil {
		http.Error(w, "Collection item not found", http.StatusNotFound)
		return
	}

	pub, err := SQLloadPubData(DB, pubID)
	pubTitle := fmt.Sprintf("(pub #%d)", pubID)
	if err == nil && pub != nil {
		pubTitle = pub.PubTitle.String
	}

	HTMLheader(w, "Edit Collection Item")
	PrintNavbar(w, "collection", "", "", NavContext{ShowCollection: true, CollectionItemID: colID})

	fmt.Fprintln(w, `<div class="ContentBoxEdit">`)
	fmt.Fprintf(w, "<b>Publication:</b> <a href=\"/pub.cgi?%d\">%s</a><p>\n",
		pubID, ISFDBText(pubTitle))

	fmt.Fprintf(w, "<form method=\"post\" action=\"%s://%s/collection_submitedit.cgi\">\n",
		PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<input type=\"hidden\" name=\"col_id\" value=\"%d\">\n", colID)

	fmt.Fprintln(w, `<table class="generic_table">`)
	printCollectionFormFields(w, vals)
	fmt.Fprintln(w, `</table><p>`)

	fmt.Fprintln(w, `<input type="submit" value="Save Changes">`)
	fmt.Fprintln(w, `</form>`)
	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}

// ── collection_submitedit.cgi ─────────────────────────────────────────────────

// CollectionSubmitEditHandler serves POST /collection_submitedit.cgi —
// updates an existing collection record and redirects to collection_view.cgi.
func CollectionSubmitEditHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form data", http.StatusBadRequest)
		return
	}

	colID, err := strconv.Atoi(r.FormValue("col_id"))
	if err != nil || colID <= 0 {
		http.Error(w, "Invalid collection ID", http.StatusBadRequest)
		return
	}

	vals := collectionFormValues{
		AcqDate:    sanitizeDate(r.FormValue("col_acq_date")),
		SaleDate:   sanitizeDate(r.FormValue("col_sale_date")),
		Cond:       sanitizeChoice(r.FormValue("col_cond"), conditionOptions),
		Signature:  sanitizeYN(r.FormValue("col_signature")),
		Marginalia: sanitizeYN(r.FormValue("col_marginalia")),
		Source:     strings.TrimSpace(r.FormValue("col_source")),
		PrchPrice:  strings.TrimSpace(r.FormValue("col_prch_price")),
		InsValue:   strings.TrimSpace(r.FormValue("col_ins_value")),
		Location:   strings.TrimSpace(r.FormValue("col_location")),
		Note:       strings.TrimSpace(r.FormValue("col_note")),
	}

	_, err = UserDB.Exec(`
		UPDATE collection SET
			col_acq_date   = ?,
			col_sale_date  = ?,
			col_cond       = ?,
			col_signature  = ?,
			col_marginalia = ?,
			col_source     = ?,
			col_prch_price = ?,
			col_ins_value  = ?,
			col_location   = ?,
			col_note       = ?
		WHERE col_id = ?`,
		vals.AcqDate, vals.SaleDate, vals.Cond, vals.Signature,
		vals.Marginalia, vals.Source, vals.PrchPrice, vals.InsValue,
		vals.Location, vals.Note, colID)
	if err != nil {
		HTMLheader(w, "Collection Error")
		PrintNavbar(w, "collection", "", "", NavContext{ShowCollection: true, CollectionItemID: colID})
		fmt.Fprintf(w, "<div class=\"ContentBoxEdit\"><p>Error updating collection: %s</p></div>\n",
			ISFDBText(err.Error()))
		HTMLtrailer(w)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/collection_view.cgi?%d", colID), http.StatusSeeOther)
}

// ── collection_search.cgi ────────────────────────────────────────────────────

// CollectionSearchHandler serves /collection_search.cgi —
// presents a form for searching the collection.
func CollectionSearchHandler(w http.ResponseWriter, r *http.Request) {
	HTMLheader(w, "Search Collection")
	PrintNavbar(w, "collection", "", "", NavContext{ShowCollection: true})

	fmt.Fprintln(w, `<div class="ContentBoxEdit">`)
	fmt.Fprintf(w, "<b>Search Collection</b><p>\n")

	fmt.Fprintf(w, "<form method=\"get\" action=\"%s://%s/collection_slist.cgi\">\n",
		PROTOCOL, HTMLHOST)
	fmt.Fprintln(w, `<table class="generic_table">`)

	row := 0
	tr := func() string {
		cls := "table1"
		if row%2 == 1 {
			cls = "table2"
		}
		row++
		return fmt.Sprintf("<tr class=\"%s\">", cls)
	}

	// Acquisition date
	fmt.Fprintf(w, "%s<td><b>Acquisition Date</b></td><td>\n", tr())
	fmt.Fprintln(w, `<select name="acq_op">`)
	for _, o := range []struct{ v, l string }{
		{"before", "Before"}, {"on", "On"}, {"after", "After"},
	} {
		fmt.Fprintf(w, "<option value=\"%s\">%s</option>\n", o.v, o.l)
	}
	fmt.Fprintln(w, `</select>`)
	fmt.Fprintln(w, `<input name="acq_date" size="12" placeholder="YYYY-MM-DD">`)
	fmt.Fprintln(w, `</td></tr>`)

	// Sale date
	fmt.Fprintf(w, "%s<td><b>Sale Date</b></td><td>\n", tr())
	fmt.Fprintln(w, `<select name="sale_op">`)
	for _, o := range []struct{ v, l string }{
		{"before", "Before"}, {"on", "On"}, {"after", "After"},
	} {
		fmt.Fprintf(w, "<option value=\"%s\">%s</option>\n", o.v, o.l)
	}
	fmt.Fprintln(w, `</select>`)
	fmt.Fprintln(w, `<input name="sale_date" size="12" placeholder="YYYY-MM-DD">`)
	fmt.Fprintln(w, `</td></tr>`)

	// Cross-db fields
	for _, f := range []struct{ name, label, ph string }{
		{"author", "Author", "partial name"},
		{"title", "Title", "partial title"},
		{"isbn", "ISBN", "exact ISBN"},
	} {
		fmt.Fprintf(w, "%s<td><b>%s</b></td><td>"+
			"<input name=\"%s\" size=\"40\" placeholder=\"%s\"></td></tr>\n",
			tr(), f.label, f.name, f.ph)
	}

	fmt.Fprintln(w, `</table><p>`)
	fmt.Fprintln(w, `<input type="submit" value="Search">`)
	fmt.Fprintln(w, `</form>`)
	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}

// ── collection_slist.cgi ─────────────────────────────────────────────────────

// CollectionSlistHandler serves /collection_slist.cgi —
// displays search results from collection_search.cgi.
func CollectionSlistHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad query", http.StatusBadRequest)
		return
	}

	acqDate := strings.TrimSpace(r.FormValue("acq_date"))
	acqOp := r.FormValue("acq_op")
	saleDate := strings.TrimSpace(r.FormValue("sale_date"))
	saleOp := r.FormValue("sale_op")
	authorQ := strings.TrimSpace(r.FormValue("author"))
	titleQ := strings.TrimSpace(r.FormValue("title"))
	isbnQ := strings.TrimSpace(r.FormValue("isbn"))

	// Build the user_data.db query dynamically.
	query := `SELECT col_id, pub_id, col_acq_date, col_sale_date, col_cond,
	                 col_signature, col_marginalia, col_source, col_location, col_note
	          FROM collection WHERE 1=1`
	var args []interface{}

	opSQL := map[string]string{"before": "<", "on": "=", "after": ">"}

	if acqDate != "" {
		op := opSQL[acqOp]
		if op == "" {
			op = "="
		}
		query += " AND col_acq_date " + op + " ?"
		args = append(args, acqDate)
	}
	if saleDate != "" {
		op := opSQL[saleOp]
		if op == "" {
			op = "="
		}
		query += " AND col_sale_date " + op + " ?"
		args = append(args, saleDate)
	}

	// Cross-db: filter collection pub_ids against isfdb.db.
	needCross := authorQ != "" || titleQ != "" || isbnQ != ""
	if needCross {
		// Fetch all pub_ids currently in the collection.
		pubRows, err := UserDB.Query("SELECT pub_id FROM collection")
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		var allPubIDs []int
		for pubRows.Next() {
			var id int
			if pubRows.Scan(&id) == nil {
				allPubIDs = append(allPubIDs, id)
			}
		}
		pubRows.Close()

		crossPubIDs := collectionCrossSearch(allPubIDs, authorQ, titleQ, isbnQ)
		if len(crossPubIDs) == 0 {
			HTMLheader(w, "Collection Search Results")
			PrintNavbar(w, "collection", "", "", NavContext{ShowCollection: true})
			fmt.Fprintln(w, `<div class="ContentBox"><b>Collection Search Results</b><p>`)
			fmt.Fprintln(w, "<p>No matching items found.</p>")
			fmt.Fprintln(w, `</div>`)
			HTMLtrailer(w)
			return
		}
		// Build IN clause against user_data.db.
		placeholders := make([]string, len(crossPubIDs))
		for i, id := range crossPubIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		query += " AND pub_id IN (" + strings.Join(placeholders, ",") + ")"
	}

	query += " ORDER BY col_id DESC LIMIT 200"

	rows, err := UserDB.Query(query, args...)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var items []collectionListItem
	for rows.Next() {
		var it collectionListItem
		if err := rows.Scan(&it.ColID, &it.PubID, &it.AcqDate, &it.SaleDate,
			&it.Cond, &it.Signature, &it.Marginalia,
			&it.Source, &it.Location, &it.Note); err != nil {
			continue
		}
		items = append(items, it)
	}
	items = enrichCollectionItems(items)

	HTMLheader(w, "Collection Search Results")
	PrintNavbar(w, "collection", "", "", NavContext{ShowCollection: true})

	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintf(w, "<b>Collection Search Results</b> (%d item(s))<p>\n", len(items))

	if len(items) == 0 {
		fmt.Fprintln(w, "<p>No matching items found.</p>")
		fmt.Fprintln(w, `</div>`)
		HTMLtrailer(w)
		return
	}

	printCollectionTable(w, items)
	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}

// collectionCrossSearch returns the subset of collectionPubIDs that match the
// given author, title, and/or ISBN terms (ANDed) in isfdb.db.
// Starting from the (small) collection avoids the "first N of a popular author"
// problem that arises when scanning isfdb.db outward.
func collectionCrossSearch(collectionPubIDs []int, authorQ, titleQ, isbnQ string) []int {
	if len(collectionPubIDs) == 0 {
		return nil
	}

	// Build an IN list for the collection's pub_ids.
	placeholders := make([]string, len(collectionPubIDs))
	args := make([]interface{}, len(collectionPubIDs))
	for i, id := range collectionPubIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	inClause := strings.Join(placeholders, ",")

	query := `SELECT DISTINCT pubs.pub_id FROM pubs`
	var joins, wheres []string

	wheres = append(wheres, "pubs.pub_id IN ("+inClause+")")

	if authorQ != "" {
		joins = append(joins, `JOIN pub_authors ON pub_authors.pub_id = pubs.pub_id
			JOIN authors ON authors.author_id = pub_authors.author_id`)
		wheres = append(wheres, "authors.author_canonical LIKE ?")
		args = append(args, "%"+authorQ+"%")
	}
	if titleQ != "" {
		wheres = append(wheres, "pubs.pub_title LIKE ?")
		args = append(args, "%"+titleQ+"%")
	}
	if isbnQ != "" {
		isbn := strings.ReplaceAll(isbnQ, "-", "")
		wheres = append(wheres, "REPLACE(pubs.pub_isbn, '-', '') = ?")
		args = append(args, isbn)
	}

	for _, j := range joins {
		query += " " + j
	}
	query += " WHERE " + strings.Join(wheres, " AND ")

	rows, err := DB.Query(query, args...)
	if err != nil {
		log.Printf("collectionCrossSearch query error: %v\nquery: %s\nargs: %v", err, query, args)
		return nil
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("collectionCrossSearch rows error: %v", err)
	}
	return ids
}


// ── Shared form helpers ───────────────────────────────────────────────────────

// collectionFormValues holds the field values for the add/edit form.
type collectionFormValues struct {
	AcqDate    string
	SaleDate   string
	Cond       string
	Signature  string
	Marginalia string
	Source     string
	PrchPrice  string
	InsValue   string
	Location   string
	Note       string
}

// printCollectionFormFields renders the shared table rows used in both
// collection_new.cgi and collection_edit.cgi.
func printCollectionFormFields(w http.ResponseWriter, v collectionFormValues) {
	if v.AcqDate == "" {
		v.AcqDate = "0000-00-00"
	}
	if v.SaleDate == "" {
		v.SaleDate = "0000-00-00"
	}
	if v.Signature == "" {
		v.Signature = "n"
	}
	if v.Marginalia == "" {
		v.Marginalia = "n"
	}

	row := 0
	tr := func() string {
		cls := "table1"
		if row%2 == 1 {
			cls = "table2"
		}
		row++
		return fmt.Sprintf("<tr class=\"%s\">", cls)
	}

	// Acquisition date
	fmt.Fprintf(w, "%s<td><b>Acquisition Date</b></td>"+
		"<td><input name=\"col_acq_date\" value=\"%s\" size=\"12\"> "+
		"<small>(YYYY-MM-DD, or 0000-00-00 if unknown)</small></td></tr>\n",
		tr(), ISFDBText(v.AcqDate))

	// Sale date
	fmt.Fprintf(w, "%s<td><b>Sale Date</b></td>"+
		"<td><input name=\"col_sale_date\" value=\"%s\" size=\"12\"> "+
		"<small>(0000-00-00 if still owned)</small></td></tr>\n",
		tr(), ISFDBText(v.SaleDate))

	// Condition
	fmt.Fprintf(w, "%s<td><b>Condition</b></td><td><select name=\"col_cond\">\n", tr())
	for _, opt := range conditionOptions {
		selected := ""
		if opt == v.Cond {
			selected = " selected"
		}
		label := opt
		if label == "" {
			label = "(not specified)"
		}
		fmt.Fprintf(w, "<option value=\"%s\"%s>%s</option>\n",
			ISFDBText(opt), selected, ISFDBText(label))
	}
	fmt.Fprintln(w, "</select></td></tr>")

	// Signature
	fmt.Fprintf(w, "%s<td><b>Signed</b></td><td><select name=\"col_signature\">\n", tr())
	for _, opt := range []string{"n", "y"} {
		selected := ""
		if opt == v.Signature {
			selected = " selected"
		}
		label := "No"
		if opt == "y" {
			label = "Yes"
		}
		fmt.Fprintf(w, "<option value=\"%s\"%s>%s</option>\n", opt, selected, label)
	}
	fmt.Fprintln(w, "</select></td></tr>")

	// Marginalia
	fmt.Fprintf(w, "%s<td><b>Marginalia</b></td><td><select name=\"col_marginalia\">\n", tr())
	for _, opt := range []string{"n", "y"} {
		selected := ""
		if opt == v.Marginalia {
			selected = " selected"
		}
		label := "No"
		if opt == "y" {
			label = "Yes"
		}
		fmt.Fprintf(w, "<option value=\"%s\"%s>%s</option>\n", opt, selected, label)
	}
	fmt.Fprintln(w, "</select></td></tr>")

	// Text fields
	for _, f := range []struct{ name, label, val string }{
		{"col_source", "Source", v.Source},
		{"col_prch_price", "Purchase Price", v.PrchPrice},
		{"col_ins_value", "Insurance Value", v.InsValue},
		{"col_location", "Location", v.Location},
	} {
		fmt.Fprintf(w, "%s<td><b>%s</b></td>"+
			"<td><input name=\"%s\" value=\"%s\" size=\"40\"></td></tr>\n",
			tr(), f.label, f.name, ISFDBText(f.val))
	}

	// Note (textarea)
	fmt.Fprintf(w, "%s<td><b>Note</b></td>"+
		"<td><textarea name=\"col_note\" rows=\"4\" cols=\"60\">%s</textarea></td></tr>\n",
		tr(), ISFDBText(v.Note))
}

// sanitizeDate ensures the value looks like a date or returns the zero date.
func sanitizeDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		return s
	}
	return "0000-00-00"
}

// sanitizeYN returns "y" or "n".
func sanitizeYN(s string) string {
	if strings.ToLower(strings.TrimSpace(s)) == "y" {
		return "y"
	}
	return "n"
}

// sanitizeChoice returns s if it is in the allowed list, otherwise "".
func sanitizeChoice(s string, allowed []string) string {
	for _, a := range allowed {
		if s == a {
			return s
		}
	}
	return ""
}
