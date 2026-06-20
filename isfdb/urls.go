// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
)

// ISFDBHostCorrection replaces isfdb.org wiki URLs with the local host equivalent.
// mode "start" only replaces at the start of the string; "all" replaces everywhere.
func ISFDBHostCorrection(value, mode string) string {
	if value == "" {
		return value
	}
	localWiki := fmt.Sprintf("%s://%s/", PROTOCOL, WIKILOC)
	localCGI := fmt.Sprintf("%s://%s/", PROTOCOL, HTMLHOST)

	if mode == "start" {
		if strings.HasPrefix(value, "http://www.isfdb.org/wiki/") {
			return localWiki + value[26:]
		}
		if strings.HasPrefix(value, "https://www.isfdb.org/wiki/") {
			return localWiki + value[27:]
		}
	} else if mode == "all" {
		value = strings.ReplaceAll(value, "http://www.isfdb.org/wiki/", localWiki)
		value = strings.ReplaceAll(value, "https://www.isfdb.org/wiki/", localWiki)
		value = strings.ReplaceAll(value, "http://www.isfdb.org/cgi-bin/", localCGI)
		value = strings.ReplaceAll(value, "https://www.isfdb.org/cgi-bin/", localCGI)
	}
	return value
}

// BuildDisplayedURL normalizes a webpage URL and returns:
// (correctedURL, displayName, homePage, linkedPage)
func BuildDisplayedURL(webpage string, domains []RecognizedDomain) (string, string, string, string) {
	linkedPage := ""

	webpage = ISFDBHostCorrection(webpage, "start")

	// Handle pipe-separated "url|linked_page" format
	if strings.Contains(webpage, "|") {
		parts := strings.SplitN(webpage, "|", 2)
		webpage = parts[0]
		linkedPage = parts[1]
	}

	parsed, err := url.Parse(webpage)
	if err != nil {
		return webpage, webpage, webpage, linkedPage
	}

	// Extract domain, drop port
	netloc := strings.ToLower(parsed.Host)
	domain := strings.Split(netloc, ":")[0]
	if domain == "" {
		domain = webpage
	}

	// If no scheme, pad with https to avoid local server security issues
	if parsed.Scheme == "" {
		webpage = "https://" + webpage
	}

	// Try matching the last 4, 3, then 2 parts of the domain against recognized domains
	display := ""
	homePage := ""
	for _, index := range []int{4, 3, 2} {
		parts := strings.Split(domain, ".")
		if len(parts) < index {
			continue
		}
		part := strings.Join(parts[len(parts)-index:], ".")

		for _, d := range domains {
			if part != d.DomainName {
				continue
			}
			// Check required URL segment if defined
			if d.RequiredSegment.Valid && d.RequiredSegment.String != "" {
				if !strings.Contains(parsed.Path, d.RequiredSegment.String) {
					continue
				}
			}
			display = d.SiteName
			if d.SiteURL.Valid {
				homePage = d.SiteURL.String
			}
			// For Wikipedia, append the language code
			if part == "wikipedia.org" {
				language := strings.Split(domain, ".")[0]
				display += "-" + strings.ToUpper(language)
			}
			// For ISFDB-hosted images, build a wiki image link
			if display == "ISFDB" {
				pathParts := strings.Split(parsed.Path, "/")
				imageName := pathParts[len(pathParts)-1]
				linkedPage = fmt.Sprintf("%s://%s/index.php/Image:%s", PROTOCOL, WIKILOC, imageName)
			}
			break
		}
		if display != "" {
			break
		}
	}

	// Unrecognized domain — use raw domain as display name, strip leading www.
	if display == "" {
		display = domain
		homePage = domain
		if strings.HasPrefix(display, "www.") {
			display = display[4:]
		}
	}

	return webpage, display, homePage, linkedPage
}

// PrintWebPages writes a formatted list of webpage links to w.
// prefix is prepended before the "Webpages:" label (e.g. "<br>" or "<li>").
func PrintWebPages(w io.Writer, urls []string, prefix string, domains []RecognizedDomain) {
	if len(urls) == 0 {
		return
	}

	// Group corrected URLs by display name
	printed := make(map[string][]string)
	for _, webpage := range urls {
		corrected, display, _, _ := BuildDisplayedURL(webpage, domains)
		printed[display] = append(printed[display], corrected)
	}

	// Sort display names case-insensitively
	displayNames := make([]string, 0, len(printed))
	for name := range printed {
		displayNames = append(displayNames, name)
	}
	sort.Slice(displayNames, func(i, j int) bool {
		return strings.ToLower(displayNames[i]) < strings.ToLower(displayNames[j])
	})

	output := ""
	total := 0
	for _, display := range displayNames {
		for count, webpage := range printed[display] {
			if total == 0 {
				output = fmt.Sprintf("%s<b>Webpages:</b> ", prefix)
			} else {
				output += ", "
			}
			qualifier := ""
			if len(printed[display]) > 1 {
				qualifier = fmt.Sprintf("-%d", count+1)
			}
			output += fmt.Sprintf("<a href=\"%s\" target=\"_blank\">%s%s</a>", webpage, display, qualifier)
			total++
		}
	}
	fmt.Fprintln(w, output)
}
