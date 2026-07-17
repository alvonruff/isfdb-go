// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"net/http"
	"sync/atomic"
)

// ActiveHandler is the live HTTP handler used by the server. main() passes it
// to http.ListenAndServe so we can swap the underlying mux without restarting.
var ActiveHandler atomic.Value // stores http.Handler

// ServeHTTP implements http.Handler by delegating to the current ActiveHandler.
type SwappableHandler struct{}

func (SwappableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ActiveHandler.Load().(http.Handler).ServeHTTP(w, r)
}

// newAppMux builds and returns a fully-wired ServeMux for normal operation.
// Call after DBopen and UserDBOpen have succeeded.
func newAppMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/index.cgi", IndexHandler)
	mux.HandleFunc("/title.cgi", TitleHandler)
	mux.HandleFunc("/author.cgi", AuthorHandler)
	mux.HandleFunc("/pub.cgi", PubHandler)
	mux.HandleFunc("/publisher.cgi", PublisherHandler)
	mux.HandleFunc("/publisheryear.cgi", PublisherYearHandler)
	mux.HandleFunc("/pubseries.cgi", PubSeriesHandler)
	mux.HandleFunc("/publisher_authors.cgi", PublisherAuthorsHandler)
	mux.HandleFunc("/publisher_one_author.cgi", PublisherOneAuthorHandler)
	mux.HandleFunc("/pe.cgi", PeHandler)
	mux.HandleFunc("/author_alpha.cgi", AuthorAlphaHandler)
	mux.HandleFunc("/author_chrono.cgi", AuthorChronoHandler)
	mux.HandleFunc("/author_awards.cgi", AuthorAwardsHandler)
	mux.HandleFunc("/award_details.cgi", AwardDetailsHandler)
	mux.HandleFunc("/ay.cgi", AyHandler)
	mux.HandleFunc("/award_category_year.cgi", AwardCategoryYearHandler)
	mux.HandleFunc("/award_category.cgi", AwardCategoryHandler)
	mux.HandleFunc("/awardtype.cgi", AwardTypeHandler)
	mux.HandleFunc("/pubs_not_in_series.cgi", PubsNotInSeriesHandler)
	mux.HandleFunc("/note.cgi", NoteHandler)
	mux.HandleFunc("/seriesgrid.cgi", SeriesGridHandler)
	mux.HandleFunc("/se.cgi", SearchHandler)
	mux.HandleFunc("/directory.cgi", DirectoryHandler)
	mux.HandleFunc("/award_directory.cgi", AwardDirectoryHandler)
	mux.HandleFunc("/calendar_menu.cgi", CalendarMenuHandler)
	mux.HandleFunc("/calendar_day.cgi", CalendarDayHandler)
	mux.HandleFunc("/adv_search_menu.cgi", AdvSearchMenuHandler)
	mux.HandleFunc("/adv_search_selection.cgi", AdvSearchSelectionHandler)
	mux.HandleFunc("/adv_search_results.cgi", AdvSearchResultsHandler)
	mux.HandleFunc("/adv_search_result.cgi", AdvSearchResultsHandler)
	mux.HandleFunc("/collection_new.cgi", CollectionNewHandler)
	mux.HandleFunc("/collection_submitnew.cgi", CollectionSubmitNewHandler)
	mux.HandleFunc("/collection_list.cgi", CollectionListHandler)
	mux.HandleFunc("/collection_view.cgi", CollectionViewHandler)
	mux.HandleFunc("/collection_edit.cgi", CollectionEditHandler)
	mux.HandleFunc("/collection_submitedit.cgi", CollectionSubmitEditHandler)
	mux.HandleFunc("/collection_search.cgi", CollectionSearchHandler)
	mux.HandleFunc("/collection_slist.cgi", CollectionSlistHandler)
	mux.HandleFunc("/stats-and-tops.cgi", StatsHandler)
	mux.HandleFunc("/stats.cgi", StatsReportHandler)
	mux.HandleFunc("/authors_by_debut_year_table.cgi", AuthorsByDebutYearTableHandler)
	mux.HandleFunc("/authors_by_debut_year.cgi", AuthorsByDebutYearHandler)
	mux.HandleFunc("/popular_authors_table.cgi", PopularAuthorsTableHandler)
	mux.HandleFunc("/popular_authors.cgi", PopularAuthorsHandler)
	mux.HandleFunc("/most_popular_table.cgi", MostPopularTableHandler)
	mux.HandleFunc("/most_popular.cgi", MostPopularHandler)
	mux.HandleFunc("/most_reviewed_table.cgi", MostReviewedTableHandler)
	mux.HandleFunc("/most_reviewed.cgi", MostReviewedHandler)
	mux.HandleFunc("/update.cgi", UpdateHandler)

	legacyRedirect := func(target string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			dest := target
			if q := r.URL.RawQuery; q != "" {
				dest += "?" + q
			}
			http.Redirect(w, r, dest, http.StatusMovedPermanently)
		}
	}
	mux.HandleFunc("/ea.cgi", legacyRedirect("/author.cgi"))
	mux.HandleFunc("/ae.cgi", legacyRedirect("/author_alpha.cgi"))
	mux.HandleFunc("/pl.cgi", legacyRedirect("/pub.cgi"))
	mux.HandleFunc("/ch.cgi", legacyRedirect("/author_chrono.cgi"))
	mux.HandleFunc("/eaw.cgi", legacyRedirect("/author_awards.cgi"))

	// Redirect setup.cgi to index in case the browser auto-refreshes after
	// a first-run install that already swapped the mux.
	mux.HandleFunc("/setup.cgi", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/index.cgi", http.StatusSeeOther)
	})

	mux.Handle("/", http.FileServer(http.Dir("./static")))

	return mux
}

// ActivateAppRoutes opens the databases and swaps in the full application mux.
// Safe to call from a goroutine (e.g. after first-run install).
func ActivateAppRoutes() error {
	if err := DBopen(); err != nil {
		return err
	}
	if err := UserDBOpen(); err != nil {
		DBclose()
		return err
	}
	ActiveHandler.Store(newAppMux())
	return nil
}

// RegisterRoutes is kept for compatibility: used by cmd/server in normal
// (non-first-run) startup where the DB is already present.
func RegisterRoutes() {
	ActiveHandler.Store(newAppMux())
}
