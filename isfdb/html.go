// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"html"
	"io"
	"os"
	"strings"
)

const (
	UNICODE  = "utf-8"
	PROTOCOL = "http"
	HTMLHOST = "localhost:8080"
	WIKILOC  = "localhost:8080"
)

func ISFDBText(s string) string {
	return html.EscapeString(s)
}

// ISFDBLink returns an HTML anchor tag linking to a CGI script with an integer
// ID parameter.  name is HTML-escaped automatically.
func ISFDBLink(cgi string, id int, name string) string {
	return fmt.Sprintf("<a href=\"/%s?%d\">%s</a>", cgi, id, html.EscapeString(name))
}

// ISFDBLinkNoName is identical to ISFDBLink but the label is used verbatim
// (it may already contain HTML).  Use when the link text is not plain text.
func ISFDBLinkNoName(cgi string, id int, label string) string {
	return fmt.Sprintf("<a href=\"/%s?%d\">%s</a>", cgi, id, label)
}

func HTMLheader(w io.Writer, title string) {
	fmt.Fprintln(w, `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd">`)
	fmt.Fprintln(w, `<html lang="en-us">`)
	fmt.Fprintln(w, `<head>`)
	fmt.Fprintf(w, `<meta http-equiv="content-type" content="text/html; charset=%s" >`+"\n", UNICODE)
	fmt.Fprintf(w, `<link rel="shortcut icon" href="%s://%s/favicon.ico">`+"\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<title>%s</title>\n", ISFDBText(title))
	fmt.Fprintf(w, `<link href="%s://%s/biblio.css" rel="stylesheet" type="text/css" media="screen">`+"\n", PROTOCOL, HTMLHOST)
	fmt.Fprintln(w, `</head>`)
	fmt.Fprintln(w, `<body>`)
	fmt.Fprintln(w, `<div id="wrap">`)
	fmt.Fprintf(w, `<a class="topbanner" href="%s://%s/index.cgi">`+"\n", PROTOCOL, HTMLHOST)
	fmt.Fprintln(w, `<span>`)
        fmt.Fprintf(w, "<img src=\"%s://%s/dsfdb_banner.png\" alt=\"ISFDB Banner\">\n", PROTOCOL, HTMLHOST)
        fmt.Fprintln(w, `</span>`)
        fmt.Fprintln(w, `</a>`)
        fmt.Fprintln(w, `<div id="statusbar">`)
	fmt.Fprintf(w, "<h2>%s</h2>\n", ISFDBText(title))
        //fmt.Fprintln(w, `<script type="text/javascript" src="https://www.isfdb.org/isfdb_main.js"></script>`)
        fmt.Fprintln(w, `</div>`)
}

// BuildAuthorLink returns an HTML anchor tag linking to the author page.
func BuildAuthorLink(authorID int, name string) string {
	return fmt.Sprintf("<a href=\"/author.cgi?%d\">%s</a>", authorID, ISFDBText(name))
}

// DisplayPersonLabel writes the label (e.g. "Author:", "Editors:") for a list
// of persons, pluralizing if there is more than one.
func DisplayPersonLabel(w io.Writer, personType string, persons []AuthorRef) {
	many := ""
	if len(persons) > 1 {
		many = "s"
	}
	fmt.Fprintf(w, "<b>%s%s:</b>", personType, many)
}

// DisplayPersons writes the linked author names joined by " and ".
func DisplayPersons(w io.Writer, persons []AuthorRef) {
	for i, person := range persons {
		if i > 0 {
			fmt.Fprint(w, " <b>and</b> ")
		}
		fmt.Fprint(w, BuildAuthorLink(person.AuthorID, person.Canonical))
	}
}

// PrintISBNCatalog writes a <td> cell containing the formatted ISBN and/or
// catalog ID for a publication. If neither is present, a non-breaking space
// is used to keep the cell from collapsing.
func PrintISBNCatalog(w io.Writer, p *Pub) {
	value := ""
	if p.PubISBN.Valid && p.PubISBN.String != "" {
		formatted, err := ConvertISBN(DB, p.PubISBN.String)
		if err != nil {
			formatted = p.PubISBN.String
		}
		value = ISFDBText(formatted)
	}
	if p.PubCatalog.Valid && p.PubCatalog.String != "" {
		if value != "" {
			value += " / "
		}
		value += ISFDBText(p.PubCatalog.String)
	}
	if value == "" {
		value = "&nbsp;"
	}
	fmt.Fprintf(w, "<td dir=\"ltr\">%s</td>\n", value)
}

// PrintPages writes a <td> cell containing the formatted page count.
// Page counts may be compound (e.g. "256+32") and are split onto separate
// lines with a '+' separator between them.
func PrintPages(w io.Writer, p *Pub) {
	if !p.PubPages.Valid || p.PubPages.String == "" {
		fmt.Fprintln(w, "<td>&nbsp;</td>")
		return
	}
	pageList := strings.Split(p.PubPages.String, "+")
	fmt.Fprintln(w, "<td>")
	output := ""
	for i, page := range pageList {
		if i > 0 {
			output += "+<br>"
		}
		output += ISFDBText(page)
	}
	fmt.Fprintln(w, output)
	fmt.Fprintln(w, "</td>")
}

// pubTypeShortNames maps publication content types to their abbreviated display names.
var pubTypeShortNames = map[string]string{
	"NOVEL":      "novel",
	"OMNIBUS":    "omni",
	"MAGAZINE":   "mag",
	"COLLECTION": "coll",
	"ANTHOLOGY":  "anth",
	"CHAPBOOK":   "chap",
	"FANZINE":    "fanzine",
	"NONFICTION": "non-fic",
}

// ISFDBPubFormat returns the display string for a publication format (pub_ptype).
func ISFDBPubFormat(ptype string) string {
	return ISFDBText(ptype)
}

// PrintPubType writes a <td> cell containing the abbreviated publication type.
func PrintPubType(w io.Writer, p *Pub) {
	if !p.PubCType.Valid || p.PubCType.String == "" {
		fmt.Fprintln(w, "<td>&nbsp;</td>")
		return
	}
	name, ok := pubTypeShortNames[p.PubCType.String]
	if !ok {
		name = p.PubCType.String
	}
	fmt.Fprintf(w, "<td>%s</td>\n", name)
}

func HTMLtrailer(w io.Writer) {
	fmt.Fprintln(w, `</div>`)
	fmt.Fprintln(w, `</body>`)
	fmt.Fprintln(w, `</html>`)
}

// ISFDBScan renders a cover-scan thumbnail linked to the pub page.
// It matches Python's ISFDBScan(pub_id, pub_image, css_class='scans').
// If pub_image contains "|"-separated URLs, only the first is used.
func ISFDBScan(pubID int, pubImage string) string {
	img := strings.SplitN(pubImage, "|", 2)[0]
	if img == "" {
		return ""
	}
	imgTag := fmt.Sprintf(`<img src="%s" alt="Image" class="scans">`, ISFDBText(img))
	if pubID != 0 {
		return fmt.Sprintf(`<a href="/pub.cgi?%d" dir="ltr">%s</a>`, pubID, imgTag)
	}
	return imgTag
}

// Stdout convenience wrappers for use by the cmd/ apps (CGI mode)
func HTMLheaderStdout(title string) {
	fmt.Fprintf(os.Stdout, "Content-type: text/html; charset=%s\n\n", UNICODE)
	HTMLheader(os.Stdout, title)
}
func HTMLtrailerStdout() { HTMLtrailer(os.Stdout) }
