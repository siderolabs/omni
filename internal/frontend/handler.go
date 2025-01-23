// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package frontend contains embedded web files and static handler implementation.
package frontend

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const index = "/index.html"

// StaticHandler serves embedded frontend files.
type StaticHandler struct {
	modTime   time.Time
	maxAgeSec int
}

// NewStaticHandler creates new static handler.
func NewStaticHandler(maxAgeSec int) *StaticHandler {
	return &StaticHandler{
		modTime:   time.Now(),
		maxAgeSec: maxAgeSec,
	}
}

// ServeHTTP implements http.Handler.
func (handler *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const options = http.MethodOptions + ", " + http.MethodGet + ", " + http.MethodHead

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		if !strings.HasPrefix(r.URL.Path, "/") {
			r.URL.Path = "/" + r.URL.Path
		}

		handler.serveFile(w, r, r.URL.Path)

	case http.MethodOptions:
		w.Header().Set("Allow", options)

	default:
		w.Header().Set("Allow", options)
		http.Error(w, "read-only", http.StatusMethodNotAllowed)
	}
}

type fileInfo struct {
	io.ReadSeekCloser
	fs.FileInfo
}

func (handler *StaticHandler) openFile(name string) (*fileInfo, error) {
	f, err := Dist.Open(path.Clean(filepath.Join("dist", name)))
	if err != nil {
		return nil, err
	}

	d, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if !d.Mode().IsRegular() {
		f.Close() //nolint:errcheck

		return nil, os.ErrNotExist
	}

	file, ok := f.(io.ReadSeekCloser)
	if !ok {
		return nil, fmt.Errorf("file %s is not io.ReadSeekCloser", d.Name())
	}

	return &fileInfo{
		FileInfo:       d,
		ReadSeekCloser: file,
	}, nil
}

func (handler *StaticHandler) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	for _, path := range []string{name, index} {
		file, err := handler.openFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			writeHTTPError(w, err)

			return
		}

		if path != index {
			w.Header().Set("Vary", "Accept-Encoding, User-Agent")
			w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", handler.maxAgeSec))
		}

		defer file.Close() //nolint:errcheck

		http.ServeContent(w, r, file.Name(), handler.modTime, file)

		return
	}

	writeHTTPError(w, os.ErrNotExist)
}

func writeHTTPError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		http.Error(w, "404 Page Not Found", http.StatusNotFound)
	case errors.Is(err, fs.ErrPermission):
		http.Error(w, "403 Forbidden", http.StatusForbidden)
	default:
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
	}
}
