/*
 * Copyright (c) 2020 Zachariah Knight <aeros.storkpk@gmail.com>
 *
 * Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted, provided that the above copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 *
 */

package website

import (
	"html/template"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spkaeros/rscgo/pkg/log"
	"github.com/spkaeros/rscgo/pkg/db"
)

var muxCtx = http.NewServeMux()

type InformationData struct {
	PageTitle       string
	Title           string
	Owner, OwnerBio string
	Copyright       string
}

var Information = InformationData{
	PageTitle: "",
	Title:     "RSCGo",
	OwnerBio:  "https://github.com/spkaeros/",
	Owner:     "Zach Knight",
	Copyright: "2019-2020",
}

func (s InformationData) ToLower(s2 string) string {
	return strings.ToLower(s2)
}

func (s InformationData) OnlineCount() int {
	return db.DefaultPlayerService.OnlineCount()
}

//writeContent is a helper function to write to a http.ResponseWriter easily with error handling
// returns true on success, otherwise false
func writeContent(w http.ResponseWriter, content []byte) bool {
	_, err := w.Write(content)
	if err != nil {
		log.Warning.Println("Error writing template to client:", err)
		return false
	}
	return true
}

type webpages map[string]*template.Template

var pageTemplates = make(webpages)

/*
// Load templates on program initialisation
func init() {
	layouts, err := filepath.Glob("website/* /*.html")
	if err != nil {
		log.Error.Fatal(err)
	}
	layouts2, err := filepath.Glob("website/*.html")
	if err != nil {
		log.Error.Fatal(err)
	}

	// Generate our templates map from our layouts/ and includes/ directories
	for _, layout := range append(layouts, layouts2...) {
		templates[layout[8:]] = template.Must(template.ParseFiles("website/layouts/layout.html", layout))
	}
}
*/

func render(w http.ResponseWriter, r *http.Request) {
	name := strings.Replace(filepath.Clean(r.URL.Path), ".ws", ".html", -1)
	file := filepath.Join("website", name)
	if strings.HasSuffix(file, "/game") {
		file += "/index.html"
	}
	if strings.HasSuffix(file, "/game/") {
		file += "index.html"
	}

	if strings.HasSuffix(file, "wasm") || strings.HasSuffix(file, "png") || strings.HasSuffix(file, "js") {
		http.ServeFile(w, r, file)
		return
	}

	// check template files cache
	tmpl, ok := pageTemplates[name]
	if !ok {
		// Return a 404 if the template doesn't exist or the request is for a directory
		info, err := os.Stat(file)
		if err != nil && os.IsNotExist(err) {
			
			log.Errorf("Website error at '%v' (exists:%v; directory:%v):\t%v\n", file, os.IsNotExist(err), info != nil && info.IsDir(), err)
			http.NotFound(w, r)
			return
		}

		tmpl, err = template.ParseFiles(filepath.Join("website/layouts", "layout.html"), file)
		if err != nil {
			// Log the detailed error
			log.Warn(err.Error())
			// Return a generic "Internal Server Error" message
			http.Error(w, http.StatusText(500), 500)
			return
		}

		// Cache the template in RAM for future requests to the same URL.
		// This results in faster execution times, after the very first request to a templated URL
		pageTemplates[name] = tmpl
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := tmpl.ExecuteTemplate(w, "layout", Information)
	if err != nil {
		log.Warn("Problem encountered executing a webpage template:", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

//var controlPage = template.Must(template.ParseFiles("./website/layouts/layout.html", "./website/control.html"))

//Start Binds to the web port 8080 and serves HTTP template to it.
// Note: This is a blocking call, it will not return to caller.
func Start() {
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".wasm", "application/wasm")
	muxCtx.HandleFunc("/", render)
	muxCtx.HandleFunc("/game/", render)
	// muxCtx.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./data/client"))))
	muxCtx.Handle("/game/static/", http.StripPrefix("/game/static/", http.FileServer(http.Dir("./website/game"))))
	bindGameProcManager()
	if err := http.ListenAndServe(":8080", muxCtx); err != nil {
		log.Error.Println("Could not bind to website port:", err)
		os.Exit(99)
	}
}
