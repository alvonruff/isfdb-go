# ISFDB-Go — Project Background & Development Guide

## What This Is

A desktop, read-only port of the [Internet Speculative Fiction Database (ISFDB)](https://www.isfdb.org) from Python 2/3 + MySQL to Go + SQLite. The goal is a self-contained binary that anyone can run locally — no separate database server, no web server installation. Point a browser at `localhost:8080` and browse the full ISFDB.

**Motivation:** Waves of AI crawler attacks beginning in early 2026 made the live ISFDB site unusable. The site runs on a single server (database, web server, wiki, edit/moderation tools all co-located), so any attack brings down everything. An installable desktop version provides resilience. The Go rewrite also escapes the painful Python ecosystem installation process and achieves dramatically better performance than the Python/MySQL original.

## Architecture

- **Language:** Go (`net/http` standard library, no framework)
- **Database:** SQLite via `github.com/mattn/go-sqlite3`
- **Entry point:** `cmd/server/main.go` — registers all HTTP handlers and starts on `:8080`
- **Library package:** `isfdb/` — all handlers, SQL helpers, and rendering functions
- **Static assets:** `static/` — CSS, images served via `http.FileServer`
- **Database file:** `isfdb.db` (SQLite, placed in project root at runtime)

The server is read-only with respect to bibliographic data — `isfdb.db` is never written to. POST handlers exist (or will exist) only for stateless operations like search and user preferences (stored in cookies, not the database). There is no authentication.

## Source Reference

The Python source being ported lives at `/Users/alvonruff/OFFICIAL/p3/` (a separate repo):
- `/common/` — shared utilities used across all Python CGI scripts
- `/biblio/` — main bibliography CGI scripts (title.cgi, author.cgi, pub.cgi, etc.)
- `/edit/` — editing front-end and back-end scripts (not ported — read-only target)

**Important:** Python SQL helpers in `/common/SQLparsing.py` use MySQL syntax. The file `/Users/alvonruff/OFFICIAL/p3/SQLite.py` contains working SQLite equivalents and should be the reference for any SQL being ported.

## Key SQLite vs MySQL Differences

- **Year extraction:** Use `substr(field, 1, 4)` not `YEAR()` or `strftime('%Y', ...)`. ISFDB dates like `1956-00-00` are not valid ISO dates and return NULL from `strftime`.
- **UNION ORDER BY:** SQLite forbids CASE expressions in the ORDER BY of a compound SELECT. Wrap the UNION in a subquery: `SELECT * FROM (...UNION...) ORDER BY CASE WHEN ...`.
- **NULL dates:** Dates stored as `'0000-00-00'` are common; guard against them before date arithmetic.

## URL / Handler Convention

URL parameters use `+` as a separator (not `&key=value`). Use `ParseRawParams(r)` to split `r.URL.RawQuery` on `+`. Single-parameter URLs (most record detail pages) use `ParseID(r)` which calls `strconv.Atoi(r.URL.RawQuery)` directly.

Example:
- `/title.cgi?12345` → single ID
- `/ay.cgi?23+1956` → award type ID + year
- `/seriesgrid.cgi?456+1` → series ID + display order

## File Layout — `isfdb/` Package

### Data / SQL Layer

| File | Contents |
|------|----------|
| `db.go` | `DB` global, `DBopen`/`DBclose`, `DBPath` |
| `authors.go` | `Author` struct, `SQLloadAuthorData` |
| `titles.go` | `Title` struct, `SQLloadTitleData`, `SQLloadTitlesByAuthor`, etc. |
| `pubs.go` | `Pub` struct, pub SQL helpers, `SQLGetPubsForSeriesIDs` (batch grid query) |
| `pubcontents.go` | `PubContent` struct, pub contents SQL |
| `publishers.go` | `Publisher` struct, publisher SQL |
| `awards.go` | Award structs (`Award`, `AwardType`, `AwardCat`), all award SQL helpers |
| `biblio.go` | `BibliographyData`, `LoadBibliographyData`, `Series` struct, series SQL helpers, `SQLFindSeriesTitlesByID`, `SQLBatchVerificationStatus` |
| `notes.go` | `SQLgetNotes` |
| `webpages.go` | `SQLloadXxxWebpages` helpers (one per record type) |
| `identifiers.go` | External identifier SQL |
| `templates.go` | `Templates` map — MediaWiki template substitutions used in `FormatNote` |
| `urls.go` | URL/domain SQL for recognized external domains |
| `update.go` | Database update logic: `CheckForUpdate`, `StartUpdate`, `importSQL` (string-aware SQL scanner), atomic DB swap; `UpdateState` with mutex |

### Rendering / Utility Layer

| File | Contents |
|------|----------|
| `html.go` | `HTMLheader`, `HTMLtrailer`, `ISFDBText`, `ISFDBScan`, `ISFDBPubFormat`, `PrintWebPages`, `ContentBox*` helpers |
| `format.go` | `FormatNote` — formats raw note text: template substitution, `{{BREAK}}` handling, MediaWiki `== heading ==` → `<b>`, whitespace normalisation, optional `<div class="notes">` wrapper |
| `navbar.go` | `PrintNavbar`, `searchTypeOptions`, nav section renderers (`printNavSearchBox`, `printNavOtherPages`, `printNavHistory`, `printNavLicense`) |
| `history.go` | In-memory page history (cap 10, move-to-top): `RecordHistory`, `GetHistory`, `HistoryEntry` |
| `isbn.go` | ISBN formatting |

### Handlers

| File | Route(s) |
|------|---------|
| `handler_title.go` | `/title.cgi` |
| `handler_author.go` | `/author.cgi` (also used by alpha/chrono/awards pages) |
| `handler_author_alpha.go` | `/author_alpha.cgi` (`/ae.cgi` alias) |
| `handler_author_chrono.go` | `/author_chrono.cgi` (`/ch.cgi` alias) |
| `handler_author_awards.go` | `/author_awards.cgi` (`/eaw.cgi` alias) |
| `handler_pub.go` | `/pub.cgi` (`/pl.cgi` alias) |
| `handler_pubcontents.go` | pub contents rendering (called by pub handler) |
| `handler_pubstable.go` | `PrintPubsTable` — shared pub table renderer |
| `handler_publisher.go` | `/publisher.cgi` |
| `handler_publisheryear.go` | `/publisheryear.cgi` |
| `handler_publisher_authors.go` | `/publisher_authors.cgi` |
| `handler_publisher_one_author.go` | `/publisher_one_author.cgi` |
| `handler_pubseries.go` | `/pubseries.cgi` |
| `handler_pubs_not_in_series.go` | `/pubs_not_in_series.cgi` |
| `handler_pe.go` | `/pe.cgi` — series bibliography page |
| `handler_seriesgrid.go` | `/seriesgrid.cgi` — magazine issue grid |
| `handler_note.go` | `/note.cgi` — full note display |
| `handler_awards.go` | Shared award rendering (`PrintAwardTable`, `printAwardRow`, etc.) |
| `handler_award_details.go` | `/award_details.cgi` |
| `handler_ay.go` | `/ay.cgi` — awards for a type+year |
| `handler_award_category.go` | `/award_category.cgi` |
| `handler_award_category_year.go` | `/award_category_year.cgi` |
| `handler_awardtype.go` | `/awardtype.cgi` |
| `handler_biblio.go` | `PrintBibliography` — renders the bibliography ContentBox for author pages |
| `handler_reviews.go` | Review rendering helpers |
| `handler_index.go` | `/index.cgi` — front page with search bar + type dropdown, birthday calendar, update notice |
| `handler_search.go` | `/se.cgi` — main search page (Name, Fiction Titles, All Titles, Year, Month, Series, Publisher, ISBN, Award, …) |
| `handler_directory.go` | `/directory.cgi` — author/publisher/magazine directory listings |
| `handler_award_directory.go` | `/award_directory.cgi` — list of all award types, non-ASCII names sorted first |
| `handler_calendar.go` | `/calendar_menu.cgi` (4×3 month grid), `/calendar_day.cgi` (born/died author columns) |
| `handler_adv_search_menu.go` | `/adv_search_menu.cgi` — Advanced Search landing page |
| `handler_adv_search_selection.go` | `/adv_search_selection.cgi` — field selector for a given record type |
| `handler_adv_search_results.go` | `/adv_search_results.cgi` — runs the advanced search and renders results |
| `handler_update.go` | `/update.cgi` — database update status page (check / install) |
| `handler_setup.go` | `/setup.cgi` — first-run setup page; auto-starts DB download when no `isfdb.db` present |

## Page Layout Conventions

- Most pages: `<div id="content">` wrapping one or more `<div class="ContentBox">` sections
- Award pages: `<div id="main">` (no ContentBox — matches Python's `PrintNavbar('award', ...)`)
- `note.cgi`: `<div id="main">` (single full-text note, no ContentBox)

## Key Shared Functions

- `ParseRawParams(r)` — splits raw query string on `+`, returns `[]string`
- `ParseID(r)` — parses a single integer from raw query
- `ISFDBText(s)` — HTML-escapes a string for safe output
- `ISFDBScan(pubID, imageURL)` — renders a cover thumbnail `<img>` wrapped in a pub link; image URL may be `|`-separated (takes first segment)
- `FormatNote(note, noteType, mode, id, recordType, div)` — `mode` is `"short"` (truncate at `{{BREAK}}`), `"full"` (remove marker), or `"edit"` (leave as-is); `div=true` wraps in `<div class="notes"><b>NoteType:</b> ...</div>`
- `PrintWebPages(w, urls, prefix, domains)` — renders external links with recognized domain labels
- `PrintPubsTable(w, pubs, displayType)` — shared publication table renderer
- `SQLBatchVerificationStatus(db, pubIDs)` — returns map of pub_id → 0/1/2 (unverified/primary/secondary)
- `SQLGetPubsForSeriesIDs(db, seriesIDs)` — fetches all pubs for a set of series in 3 queries (used by seriesgrid for performance)

## Award Display Pipeline

`Award` struct → `LoadAwardDisplay` / `LoadAwardDisplayBatch` → `AwardDisplay` struct → rendering functions (`PrintAwardTable`, `printAwardRow`, `printAwardLevel`, `printAwardTitle`, `printAwardAuthors`).

Award levels: 1–9 = win tiers, 10–89 = nomination tiers, 90–98 = special (mapped via `specialAwards` in `handler_awards.go`), 99 = poll (displayed as `<i>Poll Place</i>: n`).

## Development Log

| Date | Work |
|------|------|
| 6/5/26 | First Go prototype: Author struct, SQLite connection, `SQLloadAuthorData` |
| 6/6/26 | Title + Pub structs; split into packages; pub list table; CSS; static file serving |
| 6/7/26 | title.cgi: author/editor links, publisher, ISBN, pages, type, cover artist, metadata, webpages, note, synopsis |
| 6/8/26 | title.cgi: Awards section, Reviews section; major performance work (28s → 1.3s) |
| 6/9/26 | pub.cgi complete; author.cgi + biblio.go; publisher.cgi; publisheryear.cgi started |
| 6/10/26 | publisheryear.cgi; pubseries.cgi; publisher_authors.cgi; publisher_one_author.cgi; pe.cgi; ae.cgi; ch.cgi; author_awards.cgi (eaw.cgi); award_details.cgi started (~8,535 LOC) |
| 6/11/26 | award_details.cgi; ay.cgi; award_category_year.cgi; award_category.cgi; awardtype.cgi; pubs_not_in_series.cgi; note.cgi; seriesgrid.cgi; performance logging removed (~10,556 LOC) |
| 6/12/26 | se.cgi (Name/Title/Series/Publisher/ISBN/Award search); navbar with search-type dropdown; directory.cgi |
| 6/13/26 | index.cgi (front page: search bar with type dropdown, birthday calendar, update notice) |
| 6/14/26 | Page history in navbar (10 entries, move-to-top, 20-char truncation); award_directory.cgi (non-ASCII sort); calendar_menu.cgi + calendar_day.cgi |
| 6/15/26 | Database update system (update.go + handler_update.go): Google Drive download, gzip streaming, string-aware SQL import, atomic DB swap; first-run setup page (handler_setup.go) |
| 6/16/26 | adv_search_menu.cgi; adv_search_selection.cgi; adv_search_results.cgi |
| 6/20/26 | Copyright headers added to all Go files; README Installation section rewritten; initial GitHub push |

## What's Next

The core read-only bibliography, award, search, and navigation pages are complete. Planned next work:

- **User data database** (`user_data.db`) — a second SQLite file for per-user state
- **Book collection / reading list** — track personal reading against ISFDB records
- **User preferences** — theme, display options stored in the user DB

## Development Approach

Work is done in small, focused chunks (50–150 LOC at a time) with visual browser verification after each step. Claude Code cannot see rendered output, so the human provides visual feedback by comparing against the live site at `www.isfdb.org`. The Python source at `/Users/alvonruff/OFFICIAL/p3/` is the authoritative reference for behavior.

## Running the Server

```bash
cd /Users/alvonruff/isfdb-go
./RUN.sh          # or: go run ./cmd/server
# then open http://localhost:8080/title.cgi?12345
```
