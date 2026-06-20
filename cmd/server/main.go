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

	// First-run mode: no database present yet.
	if _, err := os.Stat(isfdb.DBPath); os.IsNotExist(err) {
		http.HandleFunc("/setup.cgi", isfdb.SetupHandler)
		http.HandleFunc("/update.cgi", isfdb.UpdateHandler)
		http.Handle("/", http.FileServer(http.Dir("./static")))
		http.HandleFunc("/index.cgi", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/setup.cgi", http.StatusFound)
		})
		// Catch-all: redirect any unrecognised page to setup.
		http.HandleFunc("/cgi-bin/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/setup.cgi", http.StatusFound)
		})
		log.Printf("No database found — starting in setup mode at %s://%s\n",
			isfdb.PROTOCOL, isfdb.HTMLHOST)
		log.Fatal(http.ListenAndServe(":8080", nil))
		return
	}

	if err := isfdb.DBopen(); err != nil {
		log.Fatal(err)
	}
	defer isfdb.DBclose()

	// Check for available DB update in the background at startup.
	go isfdb.CheckForUpdate()

	http.HandleFunc("/index.cgi", isfdb.IndexHandler)
	http.HandleFunc("/title.cgi", isfdb.TitleHandler)
	http.HandleFunc("/author.cgi", isfdb.AuthorHandler)
	http.HandleFunc("/pub.cgi", isfdb.PubHandler)
	http.HandleFunc("/publisher.cgi", isfdb.PublisherHandler)
	http.HandleFunc("/publisheryear.cgi", isfdb.PublisherYearHandler)
	http.HandleFunc("/pubseries.cgi", isfdb.PubSeriesHandler)
	http.HandleFunc("/publisher_authors.cgi", isfdb.PublisherAuthorsHandler)
	http.HandleFunc("/publisher_one_author.cgi", isfdb.PublisherOneAuthorHandler)
	http.HandleFunc("/pe.cgi", isfdb.PeHandler)
	http.HandleFunc("/author_alpha.cgi", isfdb.AuthorAlphaHandler)
	http.HandleFunc("/author_chrono.cgi", isfdb.AuthorChronoHandler)
	http.HandleFunc("/author_awards.cgi", isfdb.AuthorAwardsHandler)
	http.HandleFunc("/award_details.cgi", isfdb.AwardDetailsHandler)
	http.HandleFunc("/ay.cgi", isfdb.AyHandler)
	http.HandleFunc("/award_category_year.cgi", isfdb.AwardCategoryYearHandler)
	http.HandleFunc("/award_category.cgi", isfdb.AwardCategoryHandler)
	http.HandleFunc("/awardtype.cgi", isfdb.AwardTypeHandler)
	http.HandleFunc("/pubs_not_in_series.cgi", isfdb.PubsNotInSeriesHandler)
	http.HandleFunc("/note.cgi", isfdb.NoteHandler)
	http.HandleFunc("/seriesgrid.cgi", isfdb.SeriesGridHandler)
	http.HandleFunc("/se.cgi", isfdb.SearchHandler)
	http.HandleFunc("/directory.cgi", isfdb.DirectoryHandler)
	http.HandleFunc("/award_directory.cgi", isfdb.AwardDirectoryHandler)
	http.HandleFunc("/calendar_menu.cgi", isfdb.CalendarMenuHandler)
	http.HandleFunc("/calendar_day.cgi", isfdb.CalendarDayHandler)
	http.HandleFunc("/adv_search_menu.cgi", isfdb.AdvSearchMenuHandler)
	http.HandleFunc("/adv_search_selection.cgi", isfdb.AdvSearchSelectionHandler)
	http.HandleFunc("/adv_search_results.cgi", isfdb.AdvSearchResultsHandler)
	http.HandleFunc("/adv_search_result.cgi", isfdb.AdvSearchResultsHandler)
	http.HandleFunc("/stats-and-tops.cgi", isfdb.StatsHandler)
	http.HandleFunc("/authors_by_debut_year_table.cgi", isfdb.AuthorsByDebutYearTableHandler)
	http.HandleFunc("/authors_by_debut_year.cgi", isfdb.AuthorsByDebutYearHandler)
	http.HandleFunc("/popular_authors_table.cgi", isfdb.PopularAuthorsTableHandler)
	http.HandleFunc("/popular_authors.cgi", isfdb.PopularAuthorsHandler)
	http.HandleFunc("/most_popular_table.cgi", isfdb.MostPopularTableHandler)
	http.HandleFunc("/most_popular.cgi", isfdb.MostPopularHandler)
	http.HandleFunc("/most_reviewed_table.cgi", isfdb.MostReviewedTableHandler)
	http.HandleFunc("/most_reviewed.cgi", isfdb.MostReviewedHandler)
	http.HandleFunc("/update.cgi", isfdb.UpdateHandler)

	// Legacy CGI name aliases — redirect to the canonical Go URLs so that
	// links copied from Wikipedia, external sites, or the live ISFDB still work.
	legacyRedirect := func(target string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			dest := target
			if q := r.URL.RawQuery; q != "" {
				dest += "?" + q
			}
			http.Redirect(w, r, dest, http.StatusMovedPermanently)
		}
	}
	http.HandleFunc("/ea.cgi", legacyRedirect("/author.cgi"))
	http.HandleFunc("/ae.cgi", legacyRedirect("/author_alpha.cgi"))
	http.HandleFunc("/pl.cgi", legacyRedirect("/pub.cgi"))
	http.HandleFunc("/ch.cgi", legacyRedirect("/author_chrono.cgi"))
	http.HandleFunc("/eaw.cgi", legacyRedirect("/author_awards.cgi"))

	http.Handle("/", http.FileServer(http.Dir("./static")))

	log.Printf("Starting server at %s://%s\n", isfdb.PROTOCOL, isfdb.HTMLHOST)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
