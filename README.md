# autoindex

Silly little nginx autoindex clone.

<img src="https://github.com/mtimkovich/autoindex/blob/main/preview.png" width="400px">

## Features

My goal was to create a file server like nginx's autoindex but add some additional features like file path breadcrumbing and field sorting. But at the same time I wanted to keep the simple display of nginx's UI.


## Usage

```
Usage: autoindex [--port PORT] [--dir DIR]

Options:
  --port PORT, -p PORT   port to run on [default: 3333]
  --dir DIR, -d DIR      directory to serve [default: .]
  --help, -h             display this help and exit
```

## Roadmap

- [x] List filenames, sizes, and modTime
- [x] Create server, output to html
- [x] File path links
- [x] Sort directories first
- [x] File path breadcrumb
- [ ] Mobile display
- [ ] Sort by name, modTime, or size (JavaScript)
