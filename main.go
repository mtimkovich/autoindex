// autoindex is a tiny directory-listing server styled after nginx's autoindex.
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

//go:embed index.html
var indexHTML string

var tmpl = template.Must(template.New("index").Parse(indexHTML))

const maxName = 50 // nginx truncates displayed names at 50 columns

var (
	root = flag.String("root", ".", "directory to serve")
	addr = flag.Int("addr", 8080, "port to listen on")
	all  = flag.Bool("all", false, "show dotfiles")
	tree = flag.Bool("tree", false, "enable the folder tree sidebar")
	host = flag.String("hostname", "", "hostname to display (default: the request's Host header)")
)

func main() {
	flag.Parse()
	abs, err := filepath.Abs(*root)
	if err == nil {
		abs, err = filepath.EvalSymlinks(abs)
	}
	if err != nil {
		log.Fatal(err)
	}
	*root = abs
	log.Printf("serving %s at http://localhost:%d/", abs, *addr)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *addr), http.HandlerFunc(serve)))
}

func hidden(p string) bool {
	if *all {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

// resolve follows symlinks and reports false if the result leaves root.
func resolve(p string) (string, bool) {
	full, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(*root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}

func serve(w http.ResponseWriter, r *http.Request) {
	// Clean against a rooted path so ".." can never climb above root.
	p := path.Clean("/" + r.URL.Path)
	if hidden(p) {
		http.NotFound(w, r)
		return
	}
	dir := filepath.Join(*root, filepath.FromSlash(p))
	full, ok := resolve(dir)
	if !ok {
		http.NotFound(w, r)
		return
	}
	fi, err := os.Stat(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !fi.IsDir() {
		http.ServeFile(w, r, full)
		return
	}
	if !strings.HasSuffix(r.URL.Path, "/") {
		http.Redirect(w, r, path.Base(p)+"/", http.StatusMovedPermanently)
		return
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}
	if *tree && r.URL.Query().Get("tree") == "1" {
		// Sub-folders of this directory, for the sidebar to load on demand.
		if err := tmpl.ExecuteTemplate(w, "list", treeKids(full, "", "./", 0, nil)); err != nil {
			log.Print(err)
		}
		return
	}
	listing(w, r, p, full, entries)
}

type row struct {
	Name, Href, Date, Size string
	Full                   string // exact timestamp, shown on hover
	NamePad, DatePad       string
	SizePad                string
	Key                    string // lowercase name, for client-side sorting
	Dir, Unix              int64
	Bytes                  int64
}

type crumb struct{ Pre, Name, Href string } // Pre is the plain-text separator before the link

// up returns the relative link n directories above the current one.
func up(n int) string {
	if n == 0 {
		return "./"
	}
	return strings.Repeat("../", n)
}

func pad(s string, w int) string { return strings.Repeat(" ", w-utf8.RuneCountInString(s)) }

func listing(w http.ResponseWriter, req *http.Request, p, full string, entries []os.DirEntry) {
	now := time.Now()
	var rows []row
	for _, e := range entries {
		if hidden(e.Name()) {
			continue
		}
		target, ok := resolve(filepath.Join(full, e.Name()))
		if !ok {
			continue
		}
		fi, err := os.Stat(target)
		if err != nil {
			continue
		}
		rw := row{Name: e.Name(), Key: strings.ToLower(e.Name()), Unix: fi.ModTime().Unix(),
			Bytes: fi.Size(), Date: humanTime(fi.ModTime(), now), Size: "-",
			Full: fi.ModTime().Format("2006-01-02 15:04:05 MST")}
		u := url.URL{Path: "./" + e.Name()}
		rw.Href = u.String()
		if fi.IsDir() {
			rw.Dir, rw.Bytes = 1, 0
			rw.Name += "/"
			rw.Href += "/"
		} else {
			rw.Size = humanSize(fi.Size())
		}
		rows = append(rows, rw)
	}
	// Initial order (also the no-JS order); the page script re-sorts on demand.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Dir != rows[j].Dir {
			return rows[i].Dir > rows[j].Dir
		}
		return rows[i].Key < rows[j].Key
	})

	// Column widths: fit the content, capped like nginx. Headers carry two
	// extra columns for the sort arrow the script draws.
	nameW, dateW, sizeW := len("Name")+2, len("Modified")+2, len("Size")+2
	for i := range rows {
		rows[i].Name = truncate(rows[i].Name, maxName)
		nameW = max(nameW, utf8.RuneCountInString(rows[i].Name))
		dateW = max(dateW, len(rows[i].Date))
		sizeW = max(sizeW, len(rows[i].Size))
	}
	for i := range rows {
		r := &rows[i]
		r.NamePad, r.DatePad, r.SizePad = pad(r.Name, nameW), pad(r.Date, dateW), pad(r.Size, sizeW)
	}

	// Breadcrumbs for the heading: the host (root), then one link per path segment.
	host := *host
	if host == "" {
		host = displayHost(req.Host)
	}
	segs := []string{}
	if p != "/" {
		segs = strings.Split(strings.Trim(p, "/"), "/")
	}
	crumbs := []crumb{{"", host, up(len(segs))}}
	trail := host
	for i, seg := range segs {
		crumbs = append(crumbs, crumb{" > ", seg, up(len(segs) - 1 - i)})
		trail += " > " + seg
	}
	// Page title: "current folder - host > folder > folder".
	title := trail
	if len(segs) > 0 {
		title = segs[len(segs)-1] + " - " + trail
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]any{"Title": title, "Crumbs": crumbs, "Parent": p != "/", "Rows": rows,
		"NamePad": pad("Name  ", nameW), "DatePad": pad("Modified  ", dateW), "SizePad": pad("Size  ", sizeW)}
	if *tree {
		data["Tree"] = treeRoot(host, segs)
	}
	err := tmpl.Execute(w, data)
	if err != nil {
		log.Print(err)
	}
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-3]) + "..>"
}

func humanSize(n int64) string {
	const units = "KMGTPE"
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	f, i := float64(n)/1024, 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if f < 10 {
		return fmt.Sprintf("%.1f %cB", f, units[i])
	}
	return fmt.Sprintf("%.0f %cB", f, units[i])
}

func humanTime(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d/time.Minute), "minute")
	case d < 24*time.Hour:
		return plural(int(d/time.Hour), "hour")
	case d < 30*24*time.Hour:
		return plural(int(d/(24*time.Hour)), "day")
	}
	return t.Format("Jan 2, 2006")
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit + " ago"
	}
	return fmt.Sprintf("%d %ss ago", n, unit)
}

// displayHost drops the port unless the host is an IP address.
func displayHost(h string) string {
	name, _, err := net.SplitHostPort(h)
	if err != nil {
		name = strings.Trim(h, "[]")
	}
	if net.ParseIP(name) != nil {
		return h
	}
	return name
}
