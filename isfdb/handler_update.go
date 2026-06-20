// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"net/http"
	"os"
)

// UpdateHandler serves /update.cgi — shows update status and allows the user
// to check for or apply a database update.
func UpdateHandler(w http.ResponseWriter, r *http.Request) {
	// Handle POST actions before rendering.
	if r.Method == http.MethodPost {
		r.ParseForm()
		switch r.FormValue("action") {
		case "check":
			go CheckForUpdate()
		case "update":
			StartUpdate()
		}
		http.Redirect(w, r, "/update.cgi", http.StatusSeeOther)
		return
	}

	status, msg, available, newDate, running := Upd.Snapshot()

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, "Database Update")

	// Auto-refresh while an update is running.
	if running {
		fmt.Fprintln(w, `<meta http-equiv="refresh" content="3">`)
	}

	PrintNavbar(w, "update", "", "")
	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintln(w, `<h2>Database Update</h2>`)
	fmt.Fprintf(w, "<p><b>Current database:</b> %s</p>\n", currentDBDate())

	switch status {
	case UpdateChecking:
		fmt.Fprintln(w, `<p>Checking for updates…</p>`)

	case UpdateDownloading, UpdateImporting:
		fmt.Fprintf(w, "<p><b>Update in progress:</b> %s</p>\n", ISFDBText(msg))

	case UpdateDone:
		fmt.Fprintf(w, "<p style=\"color:green\"><b>%s</b></p>\n", ISFDBText(msg))

	case UpdateError:
		fmt.Fprintf(w, "<p style=\"color:red\"><b>Error:</b> %s</p>\n", ISFDBText(msg))
		printUpdateButtons(w, available, newDate, running)

	default: // UpdateIdle
		if available {
			fmt.Fprintf(w, "<p><b>A new database version is available: %s</b></p>\n", ISFDBText(newDate))
		} else if newDate != "" {
			fmt.Fprintln(w, `<p>Your database is up to date.</p>`)
		} else {
			fmt.Fprintln(w, `<p>Update status unknown. Click below to check.</p>`)
		}
		printUpdateButtons(w, available, newDate, running)
	}

	fmt.Fprintln(w, `</div>`)
	fmt.Fprintln(w, `</div>`) // main
	HTMLtrailer(w)
}

func printUpdateButtons(w http.ResponseWriter, available bool, newDate string, running bool) {
	if running {
		return
	}
	fmt.Fprintln(w, `<form method="post" action="/update.cgi">`)
	fmt.Fprintln(w, `<input type="hidden" name="action" value="check">`)
	fmt.Fprintln(w, `<input type="submit" value="Check for Updates">`)
	fmt.Fprintln(w, `</form>`)
	if available {
		fmt.Fprintln(w, `<form method="post" action="/update.cgi">`)
		fmt.Fprintln(w, `<input type="hidden" name="action" value="update">`)
		fmt.Fprintf(w, "<input type=\"submit\" value=\"Install Update (%s)\">\n", ISFDBText(newDate))
		fmt.Fprintln(w, `</form>`)
	}
}

// currentDBDate returns the DB file modification date as a formatted string.
func currentDBDate() string {
	info, err := os.Stat(DBPath)
	if err != nil {
		return "unknown"
	}
	return info.ModTime().Format("January 2, 2006")
}
