package main

import (
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// node is one folder in the sidebar tree.
type node struct {
	Name, Href string
	Open       bool // expanded
	Cur        bool // the directory being viewed
	Lazy       bool // children not loaded yet; the page fetches them on demand
	Kids       []node
}

// subdirs returns the sorted names of the visible subdirectories of full.
func subdirs(full string) []string {
	entries, _ := os.ReadDir(full)
	var names []string
	for _, e := range entries {
		if hidden(e.Name()) {
			continue
		}
		target, ok := resolve(filepath.Join(full, e.Name()))
		if !ok {
			continue
		}
		if fi, err := os.Stat(target); err == nil && fi.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	return names
}

// treeKids builds the child nodes of full. rel is full's path below the link
// prefix base, depth its number of segments, and segs the path being viewed:
// children along that path are expanded, all others are left to load lazily.
// query is the query string every link carries forward so the sidebar's
// shown/hidden state survives a click without JavaScript.
func treeKids(full, rel, base string, depth int, segs []string, query string) []node {
	var out []node
	for _, d := range subdirs(full) {
		r := d
		if rel != "" {
			r = rel + "/" + d
		}
		u := url.URL{Path: base + r + "/", RawQuery: query}
		n := node{Name: d, Href: u.String(), Lazy: true}
		if depth < len(segs) && d == segs[depth] {
			n.Open, n.Lazy = true, false
			n.Cur = depth+1 == len(segs)
			n.Kids = treeKids(filepath.Join(full, d), r, base, depth+1, segs, query)
		}
		out = append(out, n)
	}
	return out
}

// treeRoot builds the whole sidebar: the root labelled with host, expanded
// down to the directory at segs.
func treeRoot(host string, segs []string, query string) node {
	base := up(len(segs))
	u := url.URL{Path: base, RawQuery: query}
	return node{Name: host, Href: u.String(), Open: true, Cur: len(segs) == 0,
		Kids: treeKids(*root, "", base, 0, segs, query)}
}

// encodeShown builds the query string every link on the page carries
// forward: "show=1" when the sidebar itself should stay visible across a
// click, which without JavaScript is otherwise lost (script.js instead
// remembers it in localStorage, so JS pages don't rely on this at all and
// keep their URLs clean via a history.replaceState call). Returns "" when
// the sidebar isn't shown, so most links carry nothing extra at all.
func encodeShown(shown bool) string {
	if !shown {
		return ""
	}
	return "show=1"
}
