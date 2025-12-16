// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package frontend contains embedded web files and static handler implementation.
package frontend

import (
	"crypto/rand"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/jxskiss/base62"

	"github.com/siderolabs/omni/internal/pkg/config"
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

		defer file.Close() //nolint:errcheck

		if path != index {
			w.Header().Set("Vary", "Accept-Encoding, User-Agent")
			w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", handler.maxAgeSec))
			http.ServeContent(w, r, file.Name(), handler.modTime, file)
		} else {
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			b := make([]byte, 10)
			if _, err := rand.Read(b); err != nil {
				writeHTTPError(w, fmt.Errorf("failed to read random bytes: %w", err))

				return
			}

			nonce := base62.EncodeToString(b)

			// If at all possible, when updating the CSP, please also update it for the dev server.
			// This can be done in frontend/vite.config.ts and will ensure CSP issues are reproducible
			// on both dev and prod.
			w.Header().Set("Content-Security-Policy",
				"upgrade-insecure-requests"+
					";default-src 'self'"+
					fmt.Sprintf(";script-src 'self' 'nonce-%s' https://*.userpilot.io", nonce)+
					";media-src 'self' https://js.userpilot.io"+
					";img-src * data:"+
					fmt.Sprintf(";connect-src 'self' %s https://*.auth0.com https://*.userpilot.io wss://*.userpilot.io", config.Config.Registries.GetImageFactoryBaseURL())+
					";font-src 'self' data: https://fonts.googleapis.com https://fonts.gstatic.com https://fonts.userpilot.io"+
					// We are forced to use unsafe-inline for style-src due to monaco-editor https://github.com/microsoft/monaco-editor/issues/271
					";style-src 'self' 'unsafe-inline' data: https://fonts.googleapis.com"+
					";frame-src https://*.auth0.com",
			)

			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Permissions-Policy", "accelerometer=(), ambient-light-sensor=(), "+
				"autoplay=(self), battery=(), camera=(), cross-origin-isolated=(self), display-capture=(), "+
				"document-domain=(), encrypted-media=(), fullscreen=(self), geolocation=(), gyroscope=(), "+
				"magnetometer=(), microphone=(), midi=(), payment=(), picture-in-picture=(), publickey-credentials=(self),"+
				"screen-wake-lock=(), sync-xhr=(self), usb=(), web-share=(), xr-spatial-tracking=()",
			)

			// Read index.html content
			content, err := io.ReadAll(file)
			if err != nil {
				writeHTTPError(w, err)

				return
			}

			tmpl, err := template.New("index.html").Parse(string(content))
			if err != nil {
				writeHTTPError(w, err)

				return
			}

			// Inject nonce into index.html
			err = tmpl.Execute(w, struct{ Nonce string }{Nonce: nonce})
			if err != nil {
				writeHTTPError(w, err)

				return
			}
		}

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
