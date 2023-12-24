package main

import (
	"cmp"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/dustin/go-humanize"
	sf "github.com/sa-/slicefunk"
)

var templates = template.Must(template.ParseFiles("autoindex.html"))

type FileItem struct {
	name    string
	link    string
	modTime time.Time
	size    int64
	isDir   bool
}

type PrettyFileItem struct {
	Name    string
	Link    string
	ModTime string
	Size    string
}

type Link struct {
	Text string
	Href string
}

func breadcrumb(webPath string) []*Link {
	split := strings.Split(webPath, "/")
	var crumbs []*Link

	link := "/"
	for i, p := range split {
		if p == "" {
			continue
		}

		if i > 0 {
			link = path.Join(link, p)
		}
		crumbs = append(crumbs, &Link{
			Text: p,
			Href: link,
		})
	}

	return crumbs
}

func sortFiles(a, b *FileItem) int {
	if a.isDir && !b.isDir {
		return -1
	} else if !a.isDir && b.isDir {
		return 1
	}

	return cmp.Compare(a.name, b.name)
}

func readDir(dir, webPath string) ([]*FileItem, error) {
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
			link:    path.Join("/", webPath, e.Name()),
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
		Link:    item.link,
		ModTime: humanize.Time(item.modTime),
		Size:    bytes,
	}
}

func renderDir(w http.ResponseWriter, r *http.Request, dir string, webPath string) {
	fullPath := path.Join(dir, webPath)
	items, err := readDir(fullPath, webPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	pretty := sf.Map(items, prettify)
	webPath = path.Join(filepath.Base(dir), webPath)
	pathCrumbs := breadcrumb(webPath)

	data := struct {
		Path       string
		Breadcrumb []*Link
		Items      []*PrettyFileItem
	}{
		Path:       webPath,
		Breadcrumb: pathCrumbs,
		Items:      pretty,
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
	var args struct {
		Port int    `arg:"-p" default:"3333" help:"port to run on"`
		Dir  string `arg:"-d" default:"." help:"directory to serve"`
	}

	arg.MustParse(&args)

	http.HandleFunc("/", index(args.Dir))
	port := fmt.Sprintf(":%v", args.Port)

	fmt.Printf("Running on http://localhost%v\n", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal(err)
	}
}
