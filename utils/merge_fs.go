package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
)

type MergeFS struct {
	base    http.FileSystem
	overlay http.FileSystem
}

func NewMergeFS(base, overlay http.FileSystem) http.FileSystem {
	return &MergeFS{
		base:    base,
		overlay: overlay,
	}
}

func (m MergeFS) Open(name string) (http.File, error) {
	f, err := m.overlay.Open(name)
	if err != nil {
		return m.base.Open(name)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return m.base.Open(name)
	}

	if !info.IsDir() {
		return f, nil
	}

	baseDir, _ := m.base.Open(name)
	defer func() {
		_ = baseDir.Close()
		_ = f.Close()
	}()
	return m.mergeDirs(name, info, baseDir, f)
}

func (m MergeFS) mergeDirs(name string, info os.FileInfo, baseDir http.File, overlayDir http.File) (http.File, error) {
	merged := map[string]os.FileInfo{}

	baseFiles, err := baseDir.Readdir(-1)
	if err != nil {
		return nil, err
	}
	sort.Slice(baseFiles, func(i, j int) bool { return baseFiles[i].Name() < baseFiles[j].Name() })

	overlayFiles, err := overlayDir.Readdir(-1)
	if err != nil {
		overlayFiles = nil
	}
	sort.Slice(overlayFiles, func(i, j int) bool { return overlayFiles[i].Name() < overlayFiles[j].Name() })

	for _, f := range baseFiles {
		merged[f.Name()] = f
	}
	for _, f := range overlayFiles {
		merged[f.Name()] = f
	}

	var entries []os.FileInfo
	for _, i := range merged {
		entries = append(entries, i)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return &mergedDir{
		name:    name,
		info:    info,
		entries: entries,
	}, nil
}

type mergedDir struct {
	name    string
	info    os.FileInfo
	entries []os.FileInfo
	pos     int
}

func (d mergedDir) Readdir(count int) ([]os.FileInfo, error) {
	if d.pos >= len(d.entries) && count > 0 {
		return nil, io.EOF
	}
	if count <= 0 || count > len(d.entries)-d.pos {
		count = len(d.entries) - d.pos
	}
	e := d.entries[d.pos : d.pos+count]
	d.pos += count
	return e, nil
}

func (d mergedDir) Close() error               { return nil }
func (d mergedDir) Stat() (os.FileInfo, error) { return d.info, nil }
func (d mergedDir) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("cannot Read from directory %s", d.name)
}
func (d mergedDir) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("unsupported Seek in directory %s", d.name)
}
