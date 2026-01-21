// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package compress defines HTTP compression handlers.
package compress

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

const (
	acceptEncoding = "Accept-Encoding"
	gzipEncoding   = "gzip"
	flateEncoding  = "deflate"
)

type writerFlusherCloser interface {
	io.WriteCloser
	Flush() error
}

// Handler is an adapted version of the middleware from gorilla/handlers.
func Handler(next http.Handler, level int) http.Handler {
	if level < gzip.DefaultCompression || level > gzip.BestCompression {
		panic("invalid compression level")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(w.Header().Get("Vary"), acceptEncoding) {
			w.Header().Add("Vary", acceptEncoding)
		}

		if r.Header.Get("Upgrade") != "" {
			next.ServeHTTP(w, r)

			return
		}

		encoding := encoding(r)
		if encoding == "" {
			next.ServeHTTP(w, r)

			return
		}

		var encWriter writerFlusherCloser

		switch encoding {
		case gzipEncoding:
			encWriter, _ = gzip.NewWriterLevel(w, level) //nolint:errcheck
		case flateEncoding:
			encWriter, _ = flate.NewWriter(w, level) //nolint:errcheck
		}

		defer encWriter.Close() //nolint:errcheck

		w.Header().Set("Content-Encoding", encoding)
		r.Header.Del(acceptEncoding)

		next.ServeHTTP(&compressResponseWriter{w: w, c: encWriter}, r)
	})
}

func encoding(r *http.Request) string {
	for encPart := range strings.SplitSeq(r.Header.Get(acceptEncoding), ",") {
		encPart = strings.TrimSpace(encPart)

		if encPart == gzipEncoding || encPart == flateEncoding {
			return encPart
		}
	}

	return ""
}

type compressResponseWriter struct {
	w http.ResponseWriter
	c writerFlusherCloser
}

func (cw *compressResponseWriter) Header() http.Header { return cw.w.Header() }

func (cw *compressResponseWriter) WriteHeader(c int) {
	cw.w.Header().Del("Content-Length")
	cw.w.WriteHeader(c)
}

func (cw *compressResponseWriter) Write(b []byte) (int, error) {
	h := cw.w.Header()

	h.Del("Content-Length")

	if h.Get("Content-Type") == "" {
		h.Set("Content-Type", http.DetectContentType(b))
	}

	return cw.c.Write(b)
}

func (cw *compressResponseWriter) Flush() {
	cw.c.Flush() //nolint:errcheck

	// Flush HTTP response.
	if f, ok := cw.w.(http.Flusher); ok {
		f.Flush()
	}
}
