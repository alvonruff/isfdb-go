// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	webview "github.com/webview/webview_go"

	"myproject/isfdb"
)

const addr = "127.0.0.1:8080"

func main() {
	isfdb.DBPath = "./isfdb.db"

	mux := http.NewServeMux()

	startURL := "http://" + addr + "/index.cgi"

	if _, err := os.Stat(isfdb.DBPath); os.IsNotExist(err) {
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
		startURL = "http://" + addr + "/setup.cgi"
	} else {
		if err := isfdb.DBopen(); err != nil {
			log.Fatal(err)
		}
		if err := isfdb.UserDBOpen(); err != nil {
			log.Fatal(err)
		}
		go isfdb.CheckForUpdate()
		isfdb.RegisterRoutes()
	}

	go func() { log.Fatal(http.ListenAndServe(addr, isfdb.SwappableHandler{})) }()
	waitForServer(addr)
	openWindow(startURL)
}

func openWindow(url string) {
	w := webview.New(true)
	defer w.Destroy()
	w.SetTitle("ISFDB")
	w.SetSize(1400, 900, webview.HintNone)

	// Bind a Go function that the JS intercept calls for external links.
	w.Bind("openExternal", func(href string) {
		if href != "" {
			exec.Command("open", href).Start()
		}
	})

	// Inject a click interceptor after every page load. Any link whose href
	// doesn't point at our local server is opened in the system browser instead.
	w.Init(`
		document.addEventListener('click', function(e) {
			var a = e.target.closest('a');
			if (!a) return;
			var href = a.getAttribute('href');
			if (!href) return;
			// Leave internal links, anchors, and javascript: alone.
			if (href.startsWith('/') || href.startsWith('#') ||
			    href.startsWith('javascript:') ||
			    href.startsWith('http://127.0.0.1') ||
			    href.startsWith('http://localhost')) return;
			// Everything else is external — open in system browser.
			if (href.startsWith('http://') || href.startsWith('https://')) {
				e.preventDefault();
				openExternal(href);
			}
		}, true);
	`)

	w.Navigate(url)
	w.Run()
}

func waitForServer(addr string) {
	for i := 0; i < 50; i++ {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
