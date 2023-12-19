package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dustin/go-humanize"
)

var templates = template.Must(template.ParseFiles("autoindex.html"))

type FileItem struct {
	name    string
	modTime time.Time
	size    int64
	isDir   bool
}

type PrettyFileItem struct {
	Name    string
	ModTime string
	Size    string
}

func listDir(dir string) []FileItem {
	var items []FileItem
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			log.Fatal(err)
		}

		items = append(items, FileItem{
			name:    e.Name(),
			isDir:   e.IsDir(),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}

	return items
}

func prettyOutput(dir string) []PrettyFileItem {
	items := listDir(dir)
	var pretties []PrettyFileItem

	for _, f := range items {
		var bytes string

		if f.isDir {
			bytes = "-"
		} else {
			bytes = humanize.Bytes(uint64(f.size))
		}

		pretties = append(pretties, PrettyFileItem{
			Name:    f.name,
			ModTime: humanize.Time(f.modTime),
			Size:    bytes,
		})
	}

	return pretties
}

func handler(w http.ResponseWriter, r *http.Request) {
	webPath := r.URL.Path[len("/"):]
	if webPath != "" {
		http.Error(w, "HTTP 403 - Forbidden", http.StatusForbidden)
		return
	}

	const testDir = "/mnt/chromeos/MyFiles/Downloads/uh"
	if webPath == "" {
		webPath = filepath.Base(testDir)
	}

	data := struct {
		Path  string
		Items []PrettyFileItem
	}{
		Path:  webPath,
		Items: prettyOutput(testDir),
	}

	err := templates.ExecuteTemplate(w, "autoindex.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/", handler)
	port := ":3333"

	fmt.Printf("Listening on http://localhost%v/\n", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal(err)
	}
}
