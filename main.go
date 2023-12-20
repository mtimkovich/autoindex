package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"text/template"
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

func readDir(dir string) ([]FileItem, error) {
	var items []FileItem
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return nil, err
		}

		items = append(items, FileItem{
			name:    e.Name(),
			isDir:   e.IsDir(),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}

	return items, nil
}

func prettyOutput(dir string) ([]PrettyFileItem, error) {
	items, err := readDir(dir)
	if err != nil {
		return nil, err
	}
	var pretties []PrettyFileItem

	for _, f := range items {
		var bytes string
		name := f.name

		if f.isDir {
			bytes = "-"
			name += "/"
		} else {
			bytes = humanize.Bytes(uint64(f.size))
		}

		pretties = append(pretties, PrettyFileItem{
			Name:    name,
			ModTime: humanize.Time(f.modTime),
			Size:    bytes,
		})
	}

	return pretties, nil
}

func renderDir(w http.ResponseWriter, r *http.Request, dir string, webPath string) {
	fullPath := path.Join(dir, webPath)
	items, err := prettyOutput(fullPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	data := struct {
		Path  string
		Items []PrettyFileItem
	}{
		Path:  webPath,
		Items: items,
	}

	err = templates.ExecuteTemplate(w, "autoindex.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func index(dir string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		webPath := r.URL.Path[len("/"):]
		fullPath := path.Join(dir, webPath)

		if f, err := os.Stat(fullPath); err == nil {
			if f.IsDir() {
				renderDir(w, r, dir, webPath)
				return
			} else {
				// TODO: May want to fall back on nginx here.
				http.ServeFile(w, r, fullPath)
				return
			}
		}

		http.NotFound(w, r)
	}
}

func main() {
	const testDir = "/mnt/chromeos/MyFiles/Downloads/uh"

	http.HandleFunc("/", index(testDir))
	port := ":3333"

	fmt.Printf("Listening on http://localhost%v/\n", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal(err)
	}

}
