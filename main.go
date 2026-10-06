// autoindex is a tiny directory-listing server styled after nginx's autoindex.
package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
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

	"github.com/spf13/pflag"
)

//go:embed web/index.html
var indexHTML string

//go:embed web/style.css
var styleCSS string

//go:embed web/script.js
var scriptJS string

//go:embed web/icons
var iconsFS embed.FS

var tmpl = template.Must(template.New("index").Parse(indexHTML))

// Path the page's CSS and JS are served from. Handled before any user files,
// so it can't collide with something in the served directory; a real file at
// this exact path would just be shadowed.
const assetPath = "/_autoindex/"

const maxName = 50 // nginx truncates displayed names at 50 columns

var (
	port  = pflag.IntP("port", "p", 8080, "port to listen on")
	all   = pflag.BoolP("all", "a", false, "show dotfiles")
	tree  = pflag.BoolP("tree", "t", false, "enable the folder tree sidebar")
	icons = pflag.BoolP("icons", "i", false, "show file-type icons in the listing")
	human = pflag.BoolP("human-readable", "h", false, `show modification times as relative ("3 hours ago") instead of an absolute timestamp`)
	host  = pflag.StringP("hostname", "H", "", "hostname to display (default: the request's Host header)")

	root string // directory to serve; the positional arg, defaulting to "."
)

func main() {
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [directory]\n\n", os.Args[0])
		pflag.PrintDefaults()
	}
	pflag.Parse()
	root = "."
	if pflag.NArg() > 0 {
		root = pflag.Arg(0)
	}
	abs, err := filepath.Abs(root)
	if err == nil {
		abs, err = filepath.EvalSymlinks(abs)
	}
	if err != nil {
		log.Fatal(err)
	}
	root = abs
	log.Printf("serving %s at http://localhost:%d/", abs, *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), http.HandlerFunc(serve)))
}

// serveAsset serves the page's embedded CSS and JS. name is the request path
// with assetPath already stripped.
func serveAsset(w http.ResponseWriter, name string) {
	if iconName, ok := strings.CutPrefix(name, "icons/"); ok {
		data, err := iconsFS.ReadFile("web/icons/" + iconName)
		if err != nil {
			http.NotFound(w, nil)
			return
		}
		ctype := "image/png"
		if strings.HasSuffix(iconName, ".svg") {
			ctype = "image/svg+xml" // browsers won't render an SVG served as image/png
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(data)
		return
	}
	var body, ctype string
	switch name {
	case "style.css":
		body, ctype = styleCSS, "text/css; charset=utf-8"
	case "script.js":
		body, ctype = scriptJS, "text/javascript; charset=utf-8"
	default:
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	io.WriteString(w, body)
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
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}

func serve(w http.ResponseWriter, r *http.Request) {
	if asset, ok := strings.CutPrefix(r.URL.Path, assetPath); ok {
		serveAsset(w, asset)
		return
	}
	// Reject ".." outright rather than silently cleaning it away: cleaning
	// "/files/.." would otherwise just serve "/" instead of 404ing like a
	// request for a path that doesn't exist should.
	for _, seg := range strings.Split(r.URL.Path, "/") {
		if seg == ".." {
			http.NotFound(w, r)
			return
		}
	}
	// Clean against a rooted path so stray ".."/"." segments that survived
	// the check above (there shouldn't be any) still can't climb above root.
	p := path.Clean("/" + r.URL.Path)
	if hidden(p) {
		http.NotFound(w, r)
		return
	}
	dir := filepath.Join(root, filepath.FromSlash(p))
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
		// Not http.ServeFile: it redirects any ".../index.html" request to the
		// directory itself, so a file named index.html could never be opened.
		f, err := os.Open(full)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		// Show HTML as text rather than rendering it: this is a file
		// browser, and rendering a served page on the app's own origin
		// isn't what anyone opening a file here is after.
		switch strings.ToLower(path.Ext(p)) {
		case ".html", ".htm":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
		}
		http.ServeContent(w, r, path.Base(p), fi.ModTime(), f)
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
		if err := tmpl.ExecuteTemplate(w, "list", treeKids(full, "", "./", 0, nil, "")); err != nil {
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
	Icon                   string // ico-<Icon> class: folder, or by extension
	Hidden                 bool   // dotfile, only ever true alongside -all
	Dir, Unix              int64
	Bytes                  int64
}

var iconsByExt = map[string][]string{
	"image":   {".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp", ".ico", ".tiff", ".heic"},
	"audio":   {".mp3", ".wav", ".flac", ".ogg", ".m4a", ".aac", ".wma"},
	"video":   {".mp4", ".mkv", ".avi", ".mov", ".webm", ".wmv", ".flv", ".m4v"},
	"archive": {".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".rar", ".7z", ".iso"},
	"pdf":     {".pdf"},
	"code":    {".go", ".js", ".ts", ".py", ".java", ".c", ".h", ".cpp", ".rs", ".rb", ".php", ".sh", ".html", ".htm", ".css", ".json", ".xml", ".sql", ".swift", ".kt", ".exe"},
}

var extIcon = func() map[string]string {
	m := make(map[string]string)
	for icon, exts := range iconsByExt {
		for _, ext := range exts {
			m[ext] = icon
		}
	}
	return m
}()

func iconOf(name string) string {
	if k, ok := extIcon[strings.ToLower(filepath.Ext(name))]; ok {
		return k
	}
	return "text"
}

type crumb struct {
	Pre        template.HTML // the separator icon before the link (empty for the first crumb)
	Name, Href string
}

// crumbSep is the separator icon between breadcrumbs. It's a fixed, trusted
// string, never built from user input, so rendering it unescaped is safe.
const crumbSep = template.HTML(`<svg class="sep" viewBox="0 0 24 24"><path d="M10 6l-1.4 1.4 4.6 4.6-4.6 4.6 1.4 1.4 6-6z"/></svg>`)

// attribution is an HTML comment crediting the project. html/template strips
// literal <!-- --> comments out of the template source at parse time, so
// this has to be injected as a template.HTML value (trusted, like crumbSep)
// rather than written directly into index.html.
const attribution = template.HTML(`<!--
     autoindex by Max Timkovich

     https://github.com/mtimkovich/autoindex
-->`)

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

	// The directory being viewed, split into segments: used for breadcrumbs,
	// the tree's auto-expanded ancestors, and the title.
	segs := []string{}
	if p != "/" {
		segs = strings.Split(strings.Trim(p, "/"), "/")
	}
	base := up(len(segs)) // relative path back to the root, for the CSS/JS links

	// Whether the sidebar itself is shown, carried forward as a query string
	// on every link on the page so it survives a click without JavaScript
	// (which otherwise remembers this in localStorage - see script.js).
	var navQuery, toggleHref string
	treeShown := false
	if *tree {
		treeShown = req.URL.Query().Get("show") != "0" // shown by default
		navQuery = encodeShown(treeShown)
		toggleHref = "?" + encodeShown(!treeShown)
	}
	suffix := ""
	if navQuery != "" {
		suffix = "?" + navQuery
	}

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
			Bytes: fi.Size(), Date: formatModTime(fi.ModTime(), now), Size: "-",
			Hidden: strings.HasPrefix(e.Name(), ".")}
		if *human {
			// Only meaningful as a hover tooltip when the displayed date is
			// the abbreviated relative form; with -h off, .Date is already
			// the full timestamp, so showing this too would be redundant.
			rw.Full = fi.ModTime().Format("2006-01-02 15:04:05 MST")
		}
		u := url.URL{Path: "./" + e.Name()}
		if fi.IsDir() {
			rw.Dir, rw.Bytes = 1, 0
			rw.Icon = "folder"
			if !*icons {
				rw.Name += "/" // the icon implies it otherwise
			}
			u.Path += "/"
			u.RawQuery = navQuery
		} else {
			rw.Size = humanSize(fi.Size())
			rw.Icon = iconOf(e.Name())
		}
		rw.Href = u.String()
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
	crumbs := []crumb{{"", host, base + suffix}}
	trail := host
	for i, seg := range segs {
		crumbs = append(crumbs, crumb{crumbSep, seg, up(len(segs)-1-i) + suffix})
		trail += " > " + seg
	}
	// Page title: "current folder - host > folder > folder".
	title := trail
	if len(segs) > 0 {
		title = segs[len(segs)-1] + " - " + trail
	}
	parentHref := ""
	if p != "/" {
		parentHref = "../" + suffix
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]any{"Title": title, "Crumbs": crumbs, "ParentHref": parentHref, "Rows": rows, "Base": base,
		"NamePad": pad("Name  ", nameW), "DatePad": pad("Modified  ", dateW), "SizePad": pad("Size  ", sizeW),
		"Icons": *icons, "Attribution": attribution}
	if *tree {
		data["Tree"] = treeRoot(host, segs, navQuery)
		data["TreeShown"] = treeShown
		data["ToggleHref"] = toggleHref
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

// formatModTime renders a modification time per -human-time: relative
// ("3 hours ago") when on, or a fixed absolute timestamp when off.
func formatModTime(t, now time.Time) string {
	if *human {
		return humanTime(t, now)
	}
	return t.Format("Jan _2, 2006 15:04") // "_2" space-pads the day to 2 digits
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
	return t.Format("Jan _2, 2006") // "_2" space-pads the day to 2 digits
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
