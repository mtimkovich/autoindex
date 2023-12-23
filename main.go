package main

import (
	"cmp"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"slices"
	"time"

	"github.com/dustin/go-humanize"
	sf "github.com/sa-/slicefunk"
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

func sortFiles(a, b *FileItem) int {
	if a.isDir && !b.isDir {
		return -1
	} else if !a.isDir && b.isDir {
		return 1
	}

	return cmp.Compare(a.name, b.name)
}

func readDir(dir string) ([]*FileItem, error) {
	var items []*FileItem
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return nil, err
		}

		items = append(items, &FileItem{
			name:    e.Name(),
			isDir:   e.IsDir(),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}

	slices.SortFunc(items, sortFiles)
	return items, nil
}

// Ready item for display.
func prettify(item *FileItem) *PrettyFileItem {
	var bytes string
	name := item.name

	if item.isDir {
		bytes = "-"
		name += "/"
	} else {
		bytes = humanize.Bytes(uint64(item.size))
	}

	return &PrettyFileItem{
		Name:    name,
		ModTime: humanize.Time(item.modTime),
		Size:    bytes,
	}
}

func renderDir(w http.ResponseWriter, r *http.Request, dir string, webPath string) {
	fullPath := path.Join(dir, webPath)
	items, err := readDir(fullPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	webPath = "/" + webPath
	pretty := sf.Map(items, prettify)

	data := struct {
		Path  string
		Items []*PrettyFileItem
	}{
		Path:  webPath,
		Items: pretty,
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
				http.ServeFile(w, r, fullPath)
				return
			}
		}

		http.NotFound(w, r)
	}
}

func main() {
	port := flag.Int("port", 3333, "Port to run on")
	dir := flag.String("dir", ".", "Directory to serve")
	flag.Parse()

	http.HandleFunc("/", index(*dir))

	fmt.Printf("Listening on http://localhost:%v/\n", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%v", *port), nil)
	if err != nil {
		log.Fatal(err)
	}
}
