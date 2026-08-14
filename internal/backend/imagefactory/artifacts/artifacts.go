// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package artifacts proxies image factory artifacts - vulnerability scan reports, SPDX bundles
// and VEX documents - through Omni, so the frontend never talks to the image factory directly.
package artifacts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/blang/semver/v4"
	factoryclient "github.com/siderolabs/image-factory/pkg/client"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

// scanReportContentTypes are the report filenames the factory serves, mapped to the content type each
// one carries. It doubles as the allowlist for the "filename" path value.
var scanReportContentTypes = map[string]string{
	"report.json":  "application/json",
	"report.sarif": "application/sarif+json",
	"report.cdx":   "application/vnd.cyclonedx+json",
	"report.table": "text/plain; charset=utf-8",
}

// schematicIDPattern matches a factory schematic ID: the hex-encoded SHA-256 of the schematic.
var schematicIDPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// NewScanHandler returns a handler serving the vulnerability scan report of a schematic, in
// whichever format the requested filename selects.
func NewScanHandler(clients *imagefactory.Clients, logger *zap.Logger) http.Handler {
	return artifact{
		name: "scan report",
		path: []pathValue{
			{name: "schematicID", label: "schematic ID", validate: validSchematicID},
			{name: "talosVersion", label: "Talos version", validate: validTalosVersion},
			{name: "arch", label: "architecture", validate: validArch},
			{name: "filename", label: "report filename", validate: validReportFilename},
		},
		contentType: func(r *http.Request) string {
			return scanReportContentTypes[r.PathValue("filename")]
		},
		fetch: func(ctx context.Context, client imagefactory.FactoryClient, r *http.Request) ([]byte, error) {
			return client.ScanReport(ctx, r.PathValue("schematicID"), r.PathValue("talosVersion"), r.PathValue("arch"), r.PathValue("filename"))
		},
	}.handler(clients, logger)
}

// NewSbomHandler returns a handler serving the SBOM of a schematic.
func NewSbomHandler(clients *imagefactory.Clients, logger *zap.Logger) http.Handler {
	return artifact{
		name: "SBOM bundle",
		path: []pathValue{
			{name: "schematicID", label: "schematic ID", validate: validSchematicID},
			{name: "talosVersion", label: "Talos version", validate: validTalosVersion},
			{name: "arch", label: "architecture", validate: validArch},
		},
		contentType: staticContentType("application/spdx+json"),
		fetch: func(ctx context.Context, client imagefactory.FactoryClient, r *http.Request) ([]byte, error) {
			return client.SPDXBundle(ctx, r.PathValue("schematicID"), r.PathValue("talosVersion"), r.PathValue("arch"))
		},
	}.handler(clients, logger)
}

// NewVEXHandler returns a handler serving the VEX document of a Talos version.
func NewVEXHandler(clients *imagefactory.Clients, logger *zap.Logger) http.Handler {
	return artifact{
		name: "VEX document",
		path: []pathValue{
			{name: "talosVersion", label: "Talos version", validate: validTalosVersion},
		},
		contentType: staticContentType("application/json"),
		fetch: func(ctx context.Context, client imagefactory.FactoryClient, r *http.Request) ([]byte, error) {
			return client.VEXDocument(ctx, r.PathValue("talosVersion"))
		},
	}.handler(clients, logger)
}

// pathValue is one path value a route's pattern provides: the wildcard name it is matched under, the
// label it is reported by, and the shape it must have.
type pathValue struct {
	validate func(string) error
	name     string
	label    string
}

// artifact describes one artifact route. name appears in the log and error messages, e.g. "failed to
// get scan report".
type artifact struct {
	contentType func(*http.Request) string
	fetch       func(context.Context, imagefactory.FactoryClient, *http.Request) ([]byte, error)
	name        string

	// path is every path value this route reads, in the order its pattern declares them, and each has
	// to pass its check before the request is forwarded.
	//
	// Each of these ends up in the URL Omni asks the factory for, and Go's router hands a wildcard its
	// *unescaped* segment: an encoded separator in an unchecked one would traverse out of the artifact
	// path and point Omni's authenticated factory client at some other factory endpoint entirely.
	path []pathValue
}

// handler implements the shape every artifact route shares: validate the path, resolve the image
// factory client for the requested Talos version, fetch the artifact and write it back.
func (a artifact) handler(clients *imagefactory.Clients, logger *zap.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The request logging middleware wrapping this route logs the raw URI of every request; this
		// is what ties the lines below to the request that produced them, whatever else is in flight.
		reqLogger := logger.With(zap.String("path", r.URL.Path))

		if err := a.validatePath(r); err != nil {
			reqLogger.Info("invalid "+a.name+" request", zap.Error(err))
			WriteError(w, reqLogger, err.Error(), http.StatusBadRequest)

			return
		}

		ctx := actor.MarkContextAsInternalActor(r.Context())

		client, err := clients.ForTalosVersion(ctx, r.PathValue("talosVersion"))
		if err != nil {
			reqLogger.Error("failed to get image factory client", zap.Error(err))
			WriteError(w, reqLogger, "failed to get image factory client", http.StatusInternalServerError)

			return
		}

		data, err := a.fetch(ctx, client, r)
		if err != nil {
			code := statusFromFactoryErr(err)

			// A factory 404 or 400 is the caller asking for something that is not there, not an Omni
			// fault, so it does not belong in the log at error level.
			reqLogger.Log(logLevelForStatus(code), "failed to get "+a.name, zap.Error(err), zap.Int("status", code))
			WriteError(w, reqLogger, a.failureStatus(code), code)

			return
		}

		w.Header().Set("Content-Type", a.contentType(r))
		w.WriteHeader(http.StatusOK)

		if _, err := w.Write(data); err != nil {
			reqLogger.Error("failed to write "+a.name, zap.Error(err))
		}
	})
}

// validatePath checks every path value the route declares, and reports the first one that fails.
func (a artifact) validatePath(r *http.Request) error {
	for _, pv := range a.path {
		value := r.PathValue(pv.name)

		// Checked ahead of the value's own shape, and for every value whatever that shape is: what
		// makes traversal possible is a separator reaching the factory URL, and that is not something
		// each individual check should have to remember to rule out.
		if strings.ContainsAny(value, "/\\") || strings.Contains(value, "..") {
			return fmt.Errorf("invalid %s: must not contain a path separator", pv.label)
		}

		if err := pv.validate(value); err != nil {
			return fmt.Errorf("invalid %s: %w", pv.label, err)
		}
	}

	return nil
}

// failureStatus is the message reported for a failure to fetch this artifact, phrased for the status
// it is reported under.
func (a artifact) failureStatus(code int) string {
	switch code {
	case http.StatusNotFound:
		return a.name + " not found"
	case http.StatusBadRequest:
		return "image factory rejected the request for the " + a.name + " as invalid"
	case http.StatusBadGateway:
		return "image factory rejected Omni's credentials"
	default:
		return "failed to get " + a.name
	}
}

// statusFromFactoryErr maps a factory failure onto the status Omni reports for it.
//
// A scan that has not been produced yet, an SBOM for a version that has none, an unknown schematic:
// all of those are the factory answering 404, and reporting them as 500 would read as an Omni fault
// and hide the one thing the caller can act on.
func statusFromFactoryErr(err error) int {
	if factoryclient.IsInvalidSchematicError(err) {
		return http.StatusBadRequest
	}

	var httpErr *factoryclient.HTTPError

	if !errors.As(err, &httpErr) {
		return http.StatusInternalServerError
	}

	switch httpErr.Code {
	case http.StatusNotFound:
		return http.StatusNotFound
	case http.StatusTooManyRequests:
		return http.StatusTooManyRequests
	case http.StatusUnauthorized, http.StatusForbidden:
		// Omni's own credentials for the factory were rejected. That is a misconfiguration on this
		// side, and must not reach the caller as a 401/403 - nothing about how *they* authenticated
		// is wrong, and the frontend invalidates the user's keys on a 401.
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func logLevelForStatus(code int) zapcore.Level {
	if code < http.StatusInternalServerError {
		return zapcore.InfoLevel
	}

	return zapcore.ErrorLevel
}

// WriteError writes the JSON error body that every artifact route returns, so the frontend can
// parse a failure the same way regardless of which stage produced it - including a rejection by
// the auth middleware wrapping the route, before any handler here runs.
func WriteError(w http.ResponseWriter, logger *zap.Logger, status string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(struct {
		Status string `json:"status"`
	}{Status: status}); err != nil {
		logger.Error("failed to encode error response", zap.Error(err))
	}
}

func validTalosVersion(value string) error {
	_, err := semver.ParseTolerant(value)

	return err
}

func validSchematicID(value string) error {
	if !schematicIDPattern.MatchString(value) {
		return errors.New("expected the hex-encoded SHA-256 of a schematic")
	}

	return nil
}

func validArch(value string) error {
	switch value {
	case "amd64", "arm64":
		return nil
	default:
		return errors.New(`expected "amd64" or "arm64"`)
	}
}

func validReportFilename(value string) error {
	if _, ok := scanReportContentTypes[value]; !ok {
		return fmt.Errorf("expected one of %v", slices.Sorted(maps.Keys(scanReportContentTypes)))
	}

	return nil
}

func staticContentType(contentType string) func(*http.Request) string {
	return func(*http.Request) string {
		return contentType
	}
}
