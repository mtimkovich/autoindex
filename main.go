package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
)

const testDir = "/mnt/chromeos/MyFiles/Downloads/uh"

type FileItem struct {
	name    string
	modTime time.Time
	size    int64
	isDir   bool
}

func (f FileItem) String() string {
	var bytes string

	if f.isDir {
		bytes = "-"
	} else {
		bytes = humanize.Bytes(uint64(f.size))
	}

	modTime := humanize.Time(f.modTime)
	return fmt.Sprintf("%v\t%v\t%v", f.name, modTime, bytes)
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

func output(dir string) {
	base := filepath.Base(testDir)
	fmt.Printf("Index of / %v /\n", base)

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	for _, e := range listDir(testDir) {
		fmt.Fprintln(w, e)
		w.Flush()
	}
}

func main() {
	output(testDir)
}
