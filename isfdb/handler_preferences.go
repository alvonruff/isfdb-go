// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
)

// PreferencesHandler serves GET /mypreferences.cgi — the user preferences form.
func PreferencesHandler(w http.ResponseWriter, r *http.Request) {
	p, err := LoadUserPrefs()
	if err != nil {
		http.Error(w, "Failed to load preferences: "+err.Error(), http.StatusInternalServerError)
		return
	}

	HTMLheader(w, "User Preferences")
	PrintNavbar(w, "mypreferences", "", "")

	fmt.Fprintln(w, `<div id="content">`)
	fmt.Fprintln(w, `<div class="ContentBoxEdit">`)
	fmt.Fprintln(w, `<form id="data" method="POST" action="/submitpreferences.cgi">`)
	fmt.Fprintln(w, `<table class="generic_table">`)

	// Section: Publication Pages
	fmt.Fprintln(w, `<tr class="generic_table_header"><th colspan="2">Publication Pages</th></tr>`)
	fmt.Fprintln(w, `<tr class="table1"><td colspan="2">`)
	printCheckbox(w, "concise_disp", "Use concise Publication listing by default", p.ConciseDisp != 0)
	fmt.Fprintln(w, `</td></tr>`)

	// Section: Title Pages
	fmt.Fprintln(w, `<tr class="generic_table_header"><th colspan="2">Title Pages</th></tr>`)
	fmt.Fprintln(w, `<tr class="table1"><td colspan="2">`)
	printCheckbox(w, "covers_display", "Display cover images on Title pages", p.CoversDisplay)
	fmt.Fprintln(w, `<br>`)
	printCheckbox(w, "cover_links_display", "Display cover scan indicators on Title and search pages", p.CoverLinksDisplay)
	fmt.Fprintln(w, `<br>`)
	printCheckbox(w, "suppress_awards", "Do not display awards on Title pages", p.SuppressAwards)
	fmt.Fprintln(w, `<br>`)
	printCheckbox(w, "suppress_reviews", "Do not display reviews on Title pages", p.SuppressReviews)
	fmt.Fprintln(w, `</td></tr>`)

	// Section: Searching
	fmt.Fprintln(w, `<tr class="generic_table_header"><th colspan="2">Searching</th></tr>`)
	fmt.Fprintln(w, `<tr class="table1"><td colspan="2">`)
	printCheckbox(w, "keep_spaces_in_searches", "Keep leading and trailing spaces when searching", p.KeepSpacesInSearches)
	fmt.Fprintln(w, `</td></tr>`)

	// Section: Translations
	fmt.Fprintln(w, `<tr class="generic_table_header"><th colspan="2">Translations</th></tr>`)
	fmt.Fprintln(w, `<tr class="table1"><td>Display translations on Author and Series pages:</td><td>`)
	fmt.Fprintln(w, `<select name="display_all_languages">`)
	for _, val := range []string{"All", "None", "Selected"} {
		sel := ""
		if val == p.DisplayAllLanguages {
			sel = ` selected="selected"`
		}
		fmt.Fprintf(w, "<option%s>%s</option>\n", sel, val)
	}
	fmt.Fprintln(w, `</select></td></tr>`)

	fmt.Fprintln(w, `<tr class="table2"><td>Default language for Author and Series pages:</td><td>`)
	fmt.Fprintln(w, `<select name="default_language">`)
	// Sort language names alphabetically; look up ID for each.
	type langEntry struct {
		id   int
		name string
	}
	langs := make([]langEntry, 0, len(Languages))
	for id, name := range Languages {
		langs = append(langs, langEntry{id, name})
	}
	sort.Slice(langs, func(i, j int) bool { return langs[i].name < langs[j].name })
	for _, l := range langs {
		sel := ""
		if l.id == p.DefaultLanguage {
			sel = ` selected="selected"`
		}
		fmt.Fprintf(w, "<option value=\"%d\"%s>%s</option>\n", l.id, sel, ISFDBText(l.name))
	}
	fmt.Fprintln(w, `</select></td></tr>`)

	fmt.Fprintln(w, `<tr class="table1"><td colspan="2">`)
	printCheckbox(w, "display_title_translations", "Display translations on Title pages", p.DisplayTitleTranslations)
	fmt.Fprintln(w, `</td></tr>`)

	// Section: Display
	fmt.Fprintln(w, `<tr class="generic_table_header"><th colspan="2">Display</th></tr>`)
	fmt.Fprintln(w, `<tr class="table1"><td>Color theme:</td><td>`)
	fmt.Fprintln(w, `<select name="dark_mode">`)
	darkModeOptions := []struct {
		val  int
		label string
	}{
		{2, "System default"},
		{0, "Light"},
		{1, "Dark"},
	}
	for _, opt := range darkModeOptions {
		sel := ""
		if opt.val == p.DarkMode {
			sel = ` selected="selected"`
		}
		fmt.Fprintf(w, "<option value=\"%d\"%s>%s</option>\n", opt.val, sel, opt.label)
	}
	fmt.Fprintln(w, `</select></td></tr>`)

	// Submit
	fmt.Fprintln(w, `<tr class="table2"><td colspan="2">`)
	fmt.Fprintln(w, `<input type="submit" value="Update Preferences">`)
	fmt.Fprintln(w, `</td></tr>`)

	fmt.Fprintln(w, `</table>`)
	fmt.Fprintln(w, `</form>`)
	fmt.Fprintln(w, `</div>`)
	fmt.Fprintln(w, `</div>`)

	HTMLtrailer(w)
}

// SubmitPreferencesHandler serves POST /submitpreferences.cgi — saves preferences.
func SubmitPreferencesHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form data", http.StatusBadRequest)
		return
	}

	checkboxVal := func(name string) int {
		if r.FormValue(name) == "on" {
			return 1
		}
		return 0
	}

	conciseDisp := checkboxVal("concise_disp")
	coversDisplay := checkboxVal("covers_display")
	coverLinksDisplay := checkboxVal("cover_links_display")
	keepSpaces := checkboxVal("keep_spaces_in_searches")
	suppressAwards := checkboxVal("suppress_awards")
	suppressReviews := checkboxVal("suppress_reviews")
	displayTitleTranslations := checkboxVal("display_title_translations")

	displayAllLanguages := r.FormValue("display_all_languages")
	switch displayAllLanguages {
	case "All", "None", "Selected":
	default:
		displayAllLanguages = "All"
	}

	defaultLanguage := 17 // English
	if v, err := strconv.Atoi(r.FormValue("default_language")); err == nil {
		defaultLanguage = v
	}

	darkMode := 2 // system default
	if v, err := strconv.Atoi(r.FormValue("dark_mode")); err == nil && v >= 0 && v <= 2 {
		darkMode = v
	}

	// Ensure exactly one row exists, then update it.
	var count int
	UserDB.QueryRow(`SELECT COUNT(*) FROM user_preferences`).Scan(&count)
	if count == 0 {
		if _, err := UserDB.Exec(`INSERT INTO user_preferences DEFAULT VALUES`); err != nil {
			http.Error(w, "Failed to save preferences: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	_, err := UserDB.Exec(`
		UPDATE user_preferences SET
			concise_disp               = ?,
			display_all_languages      = ?,
			default_language           = ?,
			covers_display             = ?,
			cover_links_display        = ?,
			keep_spaces_in_searches    = ?,
			suppress_awards            = ?,
			suppress_reviews           = ?,
			display_title_translations = ?,
			dark_mode                  = ?`,
		conciseDisp, displayAllLanguages, defaultLanguage,
		coversDisplay, coverLinksDisplay, keepSpaces,
		suppressAwards, suppressReviews, displayTitleTranslations,
		darkMode,
	)
	if err != nil {
		http.Error(w, "Failed to save preferences: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/mypreferences.cgi", http.StatusSeeOther)
}

func printCheckbox(w http.ResponseWriter, name, label string, checked bool) {
	ch := ""
	if checked {
		ch = ` checked`
	}
	fmt.Fprintf(w, `<input type="checkbox" name="%s" value="on"%s> %s`+"\n", name, ch, label)
}
