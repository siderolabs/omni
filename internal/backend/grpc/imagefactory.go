// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	gateway "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/hashicorp/golang-lru/v2/expirable"
	factoryclient "github.com/siderolabs/image-factory/pkg/client"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	imagefactorypb "github.com/siderolabs/omni/client/api/omni/imagefactory"
	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

// schematicIDPattern matches a factory schematic ID: the hex-encoded SHA-256 of the schematic.
var schematicIDPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// scanReportFilenames is the report filename the factory serves each format under.
var scanReportFilenames = map[imagefactorypb.VulnerabilityReportFormat]string{
	imagefactorypb.VulnerabilityReportFormat_JSON:      "report.json",
	imagefactorypb.VulnerabilityReportFormat_SARIF:     "report.sarif",
	imagefactorypb.VulnerabilityReportFormat_CYCLONEDX: "report.cdx",
	imagefactorypb.VulnerabilityReportFormat_TABLE:     "report.table",
}

// archNames is the name the factory knows each architecture by.
var archNames = map[imagefactorypb.Arch]string{
	imagefactorypb.Arch_AMD64: "amd64",
	imagefactorypb.Arch_ARM64: "arm64",
}

const (
	// artifactCacheSize is the number of artifacts kept in the cache. Reports and SPDX bundles run to
	// megabytes each, so this bounds what the cache can hold onto rather than aiming to fit every
	// artifact a deployment might ever serve.
	artifactCacheSize = 64

	// artifactCacheTTL is how long a fetched artifact is served from the cache.
	//
	// A scan report is not immutable - the factory rescans as vulnerabilities are published - but it
	// does not change often, and the security views fan out one request per (schematic, arch) across
	// the current Talos version and every upgrade target. Without a cache each visit re-pulls all of
	// that through Omni.
	artifactCacheTTL = 5 * time.Minute
)

// imageFactoryServer proxies image factory artifacts - vulnerability scan reports, SPDX bundles and
// VEX documents - through Omni, so the frontend never talks to the image factory directly.
type imageFactoryServer struct {
	imagefactorypb.UnimplementedImageFactoryServiceServer

	clients   *imagefactory.Clients
	artifacts *expirable.LRU[string, []byte]
	logger    *zap.Logger
}

func newImageFactoryServer(clients *imagefactory.Clients, logger *zap.Logger) *imageFactoryServer {
	return &imageFactoryServer{
		clients:   clients,
		artifacts: expirable.NewLRU[string, []byte](artifactCacheSize, nil, artifactCacheTTL),
		logger:    logger,
	}
}

func (s *imageFactoryServer) register(server grpc.ServiceRegistrar) {
	imagefactorypb.RegisterImageFactoryServiceServer(server, s)
}

func (s *imageFactoryServer) gateway(ctx context.Context, mux *gateway.ServeMux, address string, opts []grpc.DialOption) error {
	return imagefactorypb.RegisterImageFactoryServiceHandlerFromEndpoint(ctx, mux, address, opts)
}

// VulnerabilityReport implements imagefactorypb.ImageFactoryServiceServer.
func (s *imageFactoryServer) VulnerabilityReport(
	ctx context.Context, req *imagefactorypb.VulnerabilityReportRequest,
) (*imagefactorypb.VulnerabilityReportResponse, error) {
	if _, err := auth.CheckGRPC(ctx, auth.WithRole(role.Reader)); err != nil {
		return nil, err
	}

	err := validTalosVersion(req.TalosVersion)
	if err != nil {
		return nil, err
	}

	err = validSchematicID(req.SchematicId)
	if err != nil {
		return nil, err
	}

	err = validArch(req.Arch)
	if err != nil {
		return nil, err
	}

	filename, ok := scanReportFilenames[req.Format]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "invalid report format %s", req.Format)
	}

	key := strings.Join([]string{"scan", req.SchematicId, req.TalosVersion, archNames[req.Arch], filename}, "/")

	if data, ok := s.artifacts.Get(key); ok {
		return &imagefactorypb.VulnerabilityReportResponse{Data: data}, nil
	}

	data, err := s.fetch(ctx, "scan report", req.TalosVersion, func(ctx context.Context, client imagefactory.FactoryClient) ([]byte, error) {
		return client.ScanReport(ctx, req.SchematicId, req.TalosVersion, archNames[req.Arch], filename)
	})
	if err != nil {
		return nil, err
	}

	s.artifacts.Add(key, data)

	return &imagefactorypb.VulnerabilityReportResponse{Data: data}, nil
}

// SBOM implements imagefactorypb.ImageFactoryServiceServer.
func (s *imageFactoryServer) SBOM(ctx context.Context, req *imagefactorypb.SBOMRequest) (*imagefactorypb.SBOMResponse, error) {
	if _, err := auth.CheckGRPC(ctx, auth.WithRole(role.Reader)); err != nil {
		return nil, err
	}

	err := validTalosVersion(req.TalosVersion)
	if err != nil {
		return nil, err
	}

	err = validSchematicID(req.SchematicId)
	if err != nil {
		return nil, err
	}

	err = validArch(req.Arch)
	if err != nil {
		return nil, err
	}

	key := strings.Join([]string{"spdx", req.SchematicId, req.TalosVersion, archNames[req.Arch]}, "/")

	if data, ok := s.artifacts.Get(key); ok {
		return &imagefactorypb.SBOMResponse{Data: data}, nil
	}

	data, err := s.fetch(ctx, "SBOM bundle", req.TalosVersion, func(ctx context.Context, client imagefactory.FactoryClient) ([]byte, error) {
		return client.SPDXBundle(ctx, req.SchematicId, req.TalosVersion, archNames[req.Arch])
	})
	if err != nil {
		return nil, err
	}

	s.artifacts.Add(key, data)

	return &imagefactorypb.SBOMResponse{Data: data}, nil
}

// VEXDocument implements imagefactorypb.ImageFactoryServiceServer.
func (s *imageFactoryServer) VEXDocument(ctx context.Context, req *imagefactorypb.VEXDocumentRequest) (*imagefactorypb.VEXDocumentResponse, error) {
	if _, err := auth.CheckGRPC(ctx, auth.WithRole(role.Reader)); err != nil {
		return nil, err
	}

	err := validTalosVersion(req.TalosVersion)
	if err != nil {
		return nil, err
	}

	key := strings.Join([]string{"vex", req.TalosVersion}, "/")

	if data, ok := s.artifacts.Get(key); ok {
		return &imagefactorypb.VEXDocumentResponse{Data: data}, nil
	}

	data, err := s.fetch(ctx, "VEX document", req.TalosVersion, func(ctx context.Context, client imagefactory.FactoryClient) ([]byte, error) {
		return client.VEXDocument(ctx, req.TalosVersion)
	})
	if err != nil {
		return nil, err
	}

	s.artifacts.Add(key, data)

	return &imagefactorypb.VEXDocumentResponse{Data: data}, nil
}

func (s *imageFactoryServer) DownloadToken(ctx context.Context, req *imagefactorypb.DownloadTokenRequest) (*imagefactorypb.DownloadTokenResponse, error) {
	if _, err := auth.CheckGRPC(ctx, auth.WithRole(role.Reader)); err != nil {
		return nil, err
	}

	if req.Duration <= 0 {
		return nil, status.Error(codes.InvalidArgument, "duration must be positive")
	}

	logger := s.logger.With(zap.String("factory_url", req.FactoryUrl), zap.Int32("duration", req.Duration))

	client := s.clients.ForURL(req.FactoryUrl)
	if client == nil {
		return nil, status.Errorf(codes.InvalidArgument, "no factory client found for %s", req.FactoryUrl)
	}

	token, err := client.DownloadToken(ctx, time.Duration(req.Duration)*time.Second)
	if err != nil {
		code := codeFromFactoryErr(err)

		logger.Log(logLevelForCode(code), "failed to get download token", zap.Error(err), zap.Stringer("code", code))

		return nil, status.Error(code, failureMessage("download token", code))
	}

	return &imagefactorypb.DownloadTokenResponse{Token: token}, nil
}

// fetch implements the shape every artifact method shares: resolve the image factory client for the
// Talos version and fetch the artifact through it, reporting a factory failure as the code and
// message Omni answers with. Checking the caller's role, validating the request and consulting the
// cache are the caller's, and have all happened by the time it gets here.
//
// name appears in the log and error messages, e.g. "failed to get scan report".
func (s *imageFactoryServer) fetch(
	ctx context.Context,
	name, talosVersion string,
	get func(context.Context, imagefactory.FactoryClient) ([]byte, error),
) ([]byte, error) {
	logger := s.logger.With(zap.String("artifact", name), zap.String("talos_version", talosVersion))

	ctx = actor.MarkContextAsInternalActor(ctx)

	client, err := s.clients.ForTalosVersion(ctx, talosVersion)
	if err != nil {
		logger.Error("failed to get image factory client", zap.Error(err))

		return nil, status.Error(codes.Internal, "failed to get image factory client")
	}

	data, err := get(ctx, client)
	if err != nil {
		code := codeFromFactoryErr(err)

		// A factory 404 or 400 is the caller asking for something that is not there, not an Omni
		// fault, so it does not belong in the log at error level.
		logger.Log(logLevelForCode(code), "failed to get "+name, zap.Error(err), zap.Stringer("code", code))

		return nil, status.Error(code, failureMessage(name, code))
	}

	return data, nil
}

// failureMessage is the message reported for a failure to fetch an artifact, phrased for the code it
// is reported under.
func failureMessage(name string, code codes.Code) string {
	switch code { //nolint:exhaustive // every other code is reported by the default message.
	case codes.NotFound:
		return name + " not found"
	case codes.InvalidArgument:
		return "image factory rejected the request for the " + name + " as invalid"
	case codes.Unavailable:
		return "image factory rejected Omni's credentials"
	default:
		return "failed to get " + name
	}
}

// codeFromFactoryErr maps a factory failure onto the code Omni reports for it.
//
// A scan that has not been produced yet, an SBOM for a version that has none, an unknown schematic:
// all of those are the factory answering 404, and reporting them as Internal would read as an Omni
// fault and hide the one thing the caller can act on.
func codeFromFactoryErr(err error) codes.Code {
	if factoryclient.IsInvalidSchematicError(err) {
		return codes.InvalidArgument
	}

	var httpErr *factoryclient.HTTPError

	if !errors.As(err, &httpErr) {
		return codes.Internal
	}

	switch httpErr.Code {
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusTooManyRequests:
		return codes.ResourceExhausted
	case http.StatusUnauthorized, http.StatusForbidden:
		// Omni's own credentials for the factory were rejected. That is a misconfiguration on this
		// side, and must not reach the caller as Unauthenticated/PermissionDenied - nothing about how
		// *they* authenticated is wrong, and the frontend invalidates the user's keys on a 401.
		return codes.Unavailable
	default:
		return codes.Internal
	}
}

func logLevelForCode(code codes.Code) zapcore.Level {
	if code == codes.Internal {
		return zapcore.ErrorLevel
	}

	return zapcore.InfoLevel
}

// validTalosVersion rejects a value that is not a Talos version, or that is not safe to place in the
// path of the URL Omni asks the factory for.
func validTalosVersion(value string) error {
	if err := noPathSeparator("Talos version", value); err != nil {
		return err
	}

	if _, err := semver.ParseTolerant(value); err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid Talos version: %s", err)
	}

	return nil
}

// validSchematicID rejects a value that is not the hex-encoded SHA-256 of a schematic. That shape
// rules out a path separator by construction.
func validSchematicID(value string) error {
	if !schematicIDPattern.MatchString(value) {
		return status.Error(codes.InvalidArgument, "invalid schematic ID: expected the hex-encoded SHA-256 of a schematic")
	}

	return nil
}

// validArch rejects an architecture the factory has no name for. archNames holds that name, which
// the caller builds the request path from.
func validArch(value imagefactorypb.Arch) error {
	if _, ok := archNames[value]; !ok {
		return status.Errorf(codes.InvalidArgument, "invalid architecture %s", value)
	}

	return nil
}

// noPathSeparator rejects a value that would traverse out of the artifact path of the URL Omni asks
// the factory for, and point Omni's authenticated factory client at some other factory endpoint
// entirely.
func noPathSeparator(label, value string) error {
	if strings.ContainsAny(value, "/\\") || strings.Contains(value, "..") {
		return status.Errorf(codes.InvalidArgument, "invalid %s: must not contain a path separator", label)
	}

	return nil
}
