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
	w.SetTitle("DSFDB")
	w.SetSize(1400, 900, webview.HintNone)

	// Bind a Go function that the JS intercept calls for external links.
	w.Bind("openExternal", func(href string) {
		if href != "" {
			exec.Command("open", href).Start()
		}
	})

	// Inject a click interceptor after every page load. Any link whose href
	// doesn't point at our local server is opened in the system browser instead.
	// Find-in-page bar — shown/hidden with Cmd+F / Escape.
	w.Init(`
		(function() {
			var bar = document.createElement('div');
			bar.id = '_findbar';
			bar.style.cssText = 'display:none;position:fixed;top:0;left:0;right:0;z-index:999999;' +
				'background:#f0f0f0;border-bottom:1px solid #999;padding:6px 10px;' +
				'font:14px/1.4 Arial,sans-serif;box-shadow:0 2px 6px rgba(0,0,0,.25)';

			var input = document.createElement('input');
			input.type = 'text';
			input.placeholder = 'Find on page…';
			input.style.cssText = 'width:260px;padding:3px 6px;font-size:14px;border:1px solid #999;border-radius:3px';

			var info = document.createElement('span');
			info.style.cssText = 'margin-left:8px;color:#555;font-size:13px';

			var btnPrev = document.createElement('button');
			btnPrev.textContent = '▲';
			btnPrev.style.cssText = 'margin-left:8px;padding:2px 8px';

			var btnNext = document.createElement('button');
			btnNext.textContent = '▼';
			btnNext.style.cssText = 'margin-left:4px;padding:2px 8px';

			var btnClose = document.createElement('button');
			btnClose.textContent = '✕';
			btnClose.style.cssText = 'margin-left:12px;padding:2px 8px';

			bar.appendChild(input);
			bar.appendChild(btnPrev);
			bar.appendChild(btnNext);
			bar.appendChild(info);
			bar.appendChild(btnClose);
			document.documentElement.appendChild(bar);

			var matches = [], current = -1, lastTerm = '';

			function clearHighlights() {
				matches.forEach(function(m) {
					var p = m.parentNode;
					if (p) { p.replaceChild(document.createTextNode(m.textContent), m); p.normalize(); }
				});
				matches = []; current = -1;
			}

			function highlight(term) {
				clearHighlights();
				if (!term) { info.textContent = ''; return; }
				var walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT, null, false);
				var nodes = [], node;
				while ((node = walker.nextNode())) {
					if (node.parentNode.id === '_findbar') continue;
					if (node.textContent.toLowerCase().indexOf(term.toLowerCase()) >= 0) nodes.push(node);
				}
				nodes.forEach(function(n) {
					var re = new RegExp(term.replace(/[.*+?^${}()|[\]\\]/g,'\\$&'), 'gi');
					var frag = document.createDocumentFragment(), last = 0, m;
					while ((m = re.exec(n.textContent)) !== null) {
						frag.appendChild(document.createTextNode(n.textContent.slice(last, m.index)));
						var span = document.createElement('mark');
						span.style.cssText = 'background:#ff0;color:#000;border-radius:2px';
						span.textContent = m[0];
						frag.appendChild(span);
						matches.push(span);
						last = re.lastIndex;
					}
					frag.appendChild(document.createTextNode(n.textContent.slice(last)));
					n.parentNode.replaceChild(frag, n);
				});
				if (matches.length) { current = 0; scrollTo(0); }
				info.textContent = matches.length ? (1 + ' / ' + matches.length) : 'Not found';
			}

			function scrollTo(idx) {
				matches.forEach(function(m, i) {
					m.style.background = i === idx ? '#f80' : '#ff0';
				});
				if (matches[idx]) matches[idx].scrollIntoView({block:'center'});
				info.textContent = matches.length ? ((idx+1) + ' / ' + matches.length) : 'Not found';
			}

			function search() {
				var term = input.value;
				if (term !== lastTerm) { lastTerm = term; highlight(term); }
				else if (matches.length) { scrollTo(current); }
			}

			function next() { if (!matches.length) return; current = (current+1) % matches.length; scrollTo(current); }
			function prev() { if (!matches.length) return; current = (current-1+matches.length) % matches.length; scrollTo(current); }

			function open() {
				bar.style.display = 'block';
				input.focus(); input.select();
			}
			function close() {
				bar.style.display = 'none';
				clearHighlights(); input.value = ''; lastTerm = ''; info.textContent = '';
			}

			input.addEventListener('input', search);
			input.addEventListener('keydown', function(e) {
				if (e.key === 'Enter') { e.shiftKey ? prev() : next(); }
				if (e.key === 'Escape') { close(); }
			});
			btnNext.addEventListener('click', next);
			btnPrev.addEventListener('click', prev);
			btnClose.addEventListener('click', close);

			document.addEventListener('keydown', function(e) {
				if ((e.metaKey || e.ctrlKey) && e.key === 'f') {
					e.preventDefault();
					if (bar.style.display === 'none') open(); else input.focus();
				}
				if (e.key === 'Escape' && bar.style.display !== 'none') close();
			});

			// On macOS the app has no NSMenu, so Cmd+C/X/V/A never reach a
			// native handler and the OS beeps. Intercept them here instead.
			document.addEventListener('keydown', function(e) {
				if (!e.metaKey) return;
				switch (e.key) {
					case 'a': document.execCommand('selectAll'); e.preventDefault(); break;
					case 'c': document.execCommand('copy');      e.preventDefault(); break;
					case 'x': document.execCommand('cut');       e.preventDefault(); break;
					case 'v': document.execCommand('paste');     e.preventDefault(); break;
				}
			});
		})();

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
