// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"net/http"
	"os"

	"myproject/isfdb"
)

func main() {
	isfdb.DBPath = "./isfdb.db"

	mux := http.NewServeMux()

	if _, err := os.Stat(isfdb.DBPath); os.IsNotExist(err) {
		// First-run: serve setup page only; ActivateAppRoutes swaps in full mux
		// once the download completes.
		mux.HandleFunc("/setup.cgi", isfdb.SetupHandler)
		mux.HandleFunc("/update.cgi", isfdb.UpdateHandler)
		mux.Handle("/", http.FileServer(http.Dir("./static")))
		mux.HandleFunc("/index.cgi", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/setup.cgi", http.StatusFound)
		})
		mux.HandleFunc("/cgi-bin/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/setup.cgi", http.StatusFound)
		})
		isfdb.ActiveHandler.Store(http.Handler(mux))
		log.Printf("No database found — starting in setup mode at %s://%s\n",
			isfdb.PROTOCOL, isfdb.HTMLHOST)
	} else {
		if err := isfdb.DBopen(); err != nil {
			log.Fatal(err)
		}
		defer isfdb.DBclose()

		if err := isfdb.UserDBOpen(); err != nil {
			log.Fatal(err)
		}
		defer isfdb.UserDBClose()

		go isfdb.CheckForUpdate()
		isfdb.RegisterRoutes()
		log.Printf("Starting server at %s://%s\n", isfdb.PROTOCOL, isfdb.HTMLHOST)
	}

	log.Fatal(http.ListenAndServe(":8080", isfdb.SwappableHandler{}))
}
