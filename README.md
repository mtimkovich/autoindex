# autoindex

A small directory-listing server in Go, styled after nginx's `autoindex` but
friendlier: human-readable sizes and times, sortable columns, and mobile-friendly.
No dependencies and no external JavaScript.

## Build and run

```
go build -o autoindex .
./autoindex -root /path/to/files -addr 8080
```

Or without building: `go run . -root /path/to/files`.

Then open http://localhost:8080/.

## Flags

| Flag    | Default | Description                        |
|---------|---------|------------------------------------|
| `-root` | `.`     | Directory to serve                 |
| `-addr` | `8080`  | Port to listen on                  |
| `-all`  | `false` | Show dotfiles (hidden by default)  |

## Features

- **Sizes** like `684 KB` or `1.9 MB`; **times** like `3 hours ago`, or
  `Jan 2, 2026` for anything older than 30 days. Hover a date for the exact time.
- **Click a column header** (Name, Modified, Size) to sort; click again to
  reverse. Directories always stay on top. The choice is saved in the browser's
  `localStorage`. Without JavaScript the list is sorted by name.
- Files are served as-is; directories redirect to a trailing `/`.

## Security

- Paths are cleaned so `..` cannot climb above the root.
- Symlinks are followed, but any link that resolves outside the root is
  hidden from listings and returns 404.
- Dotfiles and dot-directories are hidden and return 404 unless `-all` is set.

## Files

- `main.go`: server and listing logic
- `index.html`: page template (with the small sorting script), embedded into
  the binary at build time, so the compiled binary is self-contained

## Behind a reverse proxy

All links are relative, so it works under a path prefix (e.g. `/files/`) as
long as the proxy strips the prefix before forwarding.
