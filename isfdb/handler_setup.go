// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"net/http"
)

// SetupHandler serves /setup.cgi — the first-run page shown when no database
// exists. It auto-starts the download on the first GET and shows live progress
// with auto-refresh until complete. Once the download finishes, runUpdate()
// calls ActivateAppRoutes() to swap in the full mux, after which the next
// auto-refresh of /setup.cgi redirects to /index.cgi via the full mux.
func SetupHandler(w http.ResponseWriter, r *http.Request) {
	// Auto-start the download if nothing is running yet.
	if s, _, _, _, running := Upd.Snapshot(); !running && s == UpdateIdle {
		StartUpdate()
	}

	status, msg, _, _, running := Upd.Snapshot()

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))

	fmt.Fprintln(w, `<!DOCTYPE html>`)
	fmt.Fprintln(w, `<html><head>`)
	fmt.Fprintln(w, `<meta charset="utf-8">`)
	fmt.Fprintln(w, `<title>The Desktop SF Database — First Run Setup</title>`)
	fmt.Fprintf(w, "<link rel=\"stylesheet\" href=\"%s://%s/biblio.css\">\n", PROTOCOL, HTMLHOST)

	if running || status == UpdateDone {
		fmt.Fprintln(w, `<meta http-equiv="refresh" content="3">`)
	}

	fmt.Fprintln(w, `</head><body>`)
	fmt.Fprintln(w, `<div id="main">`)
	fmt.Fprintln(w, `<h1>The Desktop SF Database</h1>`)
	fmt.Fprintln(w, `<h2>First Run Setup</h2>`)

	switch status {
	case UpdateDone:
		fmt.Fprintln(w, `<p style="color:green"><b>Download complete!</b></p>`)
		fmt.Fprintln(w, `<p>Loading database…</p>`)

	case UpdateError:
		fmt.Fprintf(w, "<p style=\"color:red\"><b>Error:</b> %s</p>\n", ISFDBText(msg))
		fmt.Fprintln(w, `<p>Please check your internet connection and `)
		fmt.Fprintln(w, `<a href="/setup.cgi">try again</a>.</p>`)

	default:
		fmt.Fprintln(w, `<p>No database was found. Downloading the ISFDB database from Google Drive.</p>`)
		fmt.Fprintln(w, `<p>This will take a few minutes — please leave this page open.</p>`)
		if msg != "" {
			fmt.Fprintf(w, "<p><b>Status:</b> %s</p>\n", ISFDBText(msg))
		}
	}

	fmt.Fprintln(w, `</div>`)
	fmt.Fprintln(w, `</body></html>`)
}
