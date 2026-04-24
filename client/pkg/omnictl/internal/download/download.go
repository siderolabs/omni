// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package download provides shared helpers for downloading installation media.
package download

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-api-signature/pkg/message"
	pgpclient "github.com/siderolabs/go-api-signature/pkg/pgp/client"
	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/client/pkg/omnictl/config"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

const (
	FormatPXE   = "pxe"
	formatISO   = "iso"
	formatRaw   = "raw"
	formatQcow2 = "qcow2"

	extensionRawXZ = "raw.xz"
)

// ImageInfo describes an installation media image for download.
type ImageInfo struct {
	MediaID        string // resource ID for CreateSchematicRequest.MediaId (empty when using Overlay)
	Name           string // display name
	DestFilePrefix string // prefix for output filename
	Profile        string // platform profile (e.g., "metal", "aws")
	Overlay        string // SBC overlay name (empty for non-SBC)
	Architecture   string // amd64, arm64
	Extension      string // file extension (iso, raw.xz, qcow2)
	NoSecureBoot   bool   // true if secure boot is not supported
}

// generateFilename builds the filename portion of the image factory URL.
func (info *ImageInfo) generateFilename(legacy, secureBoot, withExtension bool) string {
	var builder strings.Builder

	if info.Overlay != "" {
		if legacy {
			builder.WriteString(talosconstants.PlatformMetal + "-" + info.Profile)
		} else {
			builder.WriteString(talosconstants.PlatformMetal)
		}
	} else {
		builder.WriteString(info.Profile)
	}

	builder.WriteString("-" + info.Architecture)

	if secureBoot {
		builder.WriteString("-secureboot")
	}

	if withExtension {
		builder.WriteString("." + info.Extension)
	}

	return builder.String()
}

// imageInfoFromResource constructs an ImageInfo from an InstallationMedia resource.
func imageInfoFromResource(media *omni.InstallationMedia) ImageInfo {
	spec := media.TypedSpec().Value

	return ImageInfo{
		MediaID:        media.Metadata().ID(),
		Name:           spec.Name,
		DestFilePrefix: spec.DestFilePrefix,
		Profile:        spec.Profile,
		Overlay:        spec.Overlay,
		Architecture:   spec.Architecture,
		Extension:      spec.Extension,
		NoSecureBoot:   spec.NoSecureBoot,
	}
}

// Params holds the parameters for downloading installation media.
type Params struct { //nolint:govet
	Overlay *management.CreateSchematicRequest_Overlay

	Architecture   string
	Output         string
	TalosVersion   string
	JoinToken      string
	GrpcTunnelMode management.CreateSchematicRequest_SiderolinkGRPCTunnelMode

	Labels          []string
	ExtraKernelArgs []string
	Extensions      []string

	Bootloader management.SchematicBootloader
	PXE        bool
	SecureBoot bool
}

// ParseArch converts an architecture string to the proto enum.
func ParseArch(arch string) (specs.PlatformConfigSpec_Arch, error) {
	switch strings.ToLower(arch) {
	case "amd64":
		return specs.PlatformConfigSpec_AMD64, nil
	case "arm64":
		return specs.PlatformConfigSpec_ARM64, nil
	default:
		return specs.PlatformConfigSpec_UNKNOWN_ARCH, fmt.Errorf("unsupported architecture %q, must be amd64 or arm64", arch)
	}
}

// ParseBootloader converts a bootloader string to the proto enum.
func ParseBootloader(bootloader string) management.SchematicBootloader {
	switch strings.ToLower(bootloader) {
	case "uefi":
		return management.SchematicBootloader_BOOT_SD
	case "bios":
		return management.SchematicBootloader_BOOT_GRUB
	case "dual":
		return management.SchematicBootloader_BOOT_DUAL
	default:
		return management.SchematicBootloader_BOOT_AUTO
	}
}

// ArchToString converts a proto arch enum to a string.
func ArchToString(arch specs.PlatformConfigSpec_Arch) string {
	switch arch { //nolint:exhaustive
	case specs.PlatformConfigSpec_AMD64:
		return "amd64"
	case specs.PlatformConfigSpec_ARM64:
		return "arm64"
	default:
		return "unknown"
	}
}

// GRPCTunnelModeFromFlag returns the gRPC tunnel mode based on whether the flag was explicitly
// set and its value. Returns AUTO if the flag was not changed.
func GRPCTunnelModeFromFlag(cmd *cobra.Command, flagName string, enabled bool) management.CreateSchematicRequest_SiderolinkGRPCTunnelMode {
	if !cmd.Flags().Changed(flagName) {
		return management.CreateSchematicRequest_AUTO
	}

	if enabled {
		return management.CreateSchematicRequest_ENABLED
	}

	return management.CreateSchematicRequest_DISABLED
}

// ResolveJoinToken resolves a join token ID, validates it is active, and returns the ID.
// If tokenID is empty, the default join token is used.
func ResolveJoinToken(ctx context.Context, client *client.Client, tokenID string) (string, error) {
	if tokenID == "" {
		defaultJoinToken, err := safe.ReaderGetByID[*siderolink.DefaultJoinToken](ctx, client.Omni().State(), siderolink.DefaultJoinTokenID)
		if err != nil {
			return "", fmt.Errorf("failed to get default join token: %w", err)
		}

		tokenID = defaultJoinToken.TypedSpec().Value.TokenId
	}

	if err := validateJoinToken(ctx, client, tokenID); err != nil {
		return "", err
	}

	return tokenID, nil
}

func validateJoinToken(ctx context.Context, client *client.Client, tokenID string) error {
	status, err := safe.ReaderGetByID[*siderolink.JoinTokenStatus](ctx, client.Omni().State(), tokenID)
	if err != nil {
		return fmt.Errorf("join token %q not found: %w", tokenID, err)
	}

	switch status.TypedSpec().Value.State {
	case specs.JoinTokenStatusSpec_REVOKED:
		return fmt.Errorf("join token %q has been revoked", tokenID)
	case specs.JoinTokenStatusSpec_EXPIRED:
		return fmt.Errorf("join token %q has expired", tokenID)
	case specs.JoinTokenStatusSpec_ACTIVE:
		return nil
	case specs.JoinTokenStatusSpec_UNKNOWN:
		fallthrough
	default:
		return fmt.Errorf("join token %q is not active (state: %s)", tokenID, status.TypedSpec().Value.State)
	}
}

// ResolveCloudPlatform validates that a cloud platform name exists.
func ResolveCloudPlatform(ctx context.Context, client *client.Client, platform string) (*virtual.CloudPlatformConfig, error) {
	cfg, err := safe.ReaderGetByID[*virtual.CloudPlatformConfig](ctx, client.Omni().State(), platform)
	if err != nil {
		return nil, fmt.Errorf("failed to get cloud platform config for %q: %w", platform, err)
	}

	return cfg, nil
}

// ResolveOverlay resolves an overlay name to the CreateSchematicRequest_Overlay.
func ResolveOverlay(ctx context.Context, client *client.Client, overlayName, overlayOptions string) (*management.CreateSchematicRequest_Overlay, error) {
	sbcConfig, err := safe.ReaderGetByID[*virtual.SBCConfig](ctx, client.Omni().State(), overlayName)
	if err != nil {
		return nil, fmt.Errorf("failed to get SBC config for overlay %q: %w", overlayName, err)
	}

	return &management.CreateSchematicRequest_Overlay{
		Name:    sbcConfig.TypedSpec().Value.OverlayName,
		Image:   sbcConfig.TypedSpec().Value.OverlayImage,
		Options: overlayOptions,
	}, nil
}

// MediaBuildOptions holds the effective platform/overlay/format values for building ImageInfo from a preset.
type MediaBuildOptions struct {
	Format         string
	Platform       string
	Overlay        string
	OverlayOptions string
}

// BuildImageFromPreset constructs an ImageInfo from preset configuration.
// The opts contain effective values (after CLI override).
func BuildImageFromPreset(ctx context.Context, client *client.Client, name, arch string, opts MediaBuildOptions) (ImageInfo, error) {
	info := ImageInfo{
		Name:           name,
		DestFilePrefix: name,
		Architecture:   arch,
	}

	switch {
	case opts.Overlay != "":
		info.Profile = talosconstants.PlatformMetal
		info.Overlay = opts.Overlay
		info.Extension = extensionRawXZ
		info.NoSecureBoot = true

	case opts.Platform != "":
		info.Profile = opts.Platform
		info.Extension = extensionRawXZ

		cloudConfig, err := ResolveCloudPlatform(ctx, client, opts.Platform)
		if err != nil {
			return ImageInfo{}, err
		}

		if suffix := cloudConfig.TypedSpec().Value.DiskImageSuffix; suffix != "" {
			info.Extension = suffix
		}

	default:
		info.Profile = talosconstants.PlatformMetal

		switch opts.Format {
		case "", formatISO:
			info.Extension = formatISO
		case formatRaw:
			info.Extension = extensionRawXZ
		case formatQcow2:
			info.Extension = formatQcow2
		case FormatPXE:
			info.Extension = formatISO // PXE doesn't use extension for download
		default:
			return ImageInfo{}, fmt.Errorf("unsupported format %q, must be one of: %s, %s, %s, %s", opts.Format, formatISO, formatRaw, formatQcow2, FormatPXE)
		}
	}

	return info, nil
}

// BuildParamsFromPreset constructs Params from a preset spec.
func BuildParamsFromPreset(spec *specs.InstallationMediaConfigSpec, arch string) (Params, error) {
	params := Params{
		Architecture: arch,
		TalosVersion: spec.TalosVersion,
		SecureBoot:   spec.SecureBoot,
		JoinToken:    spec.JoinToken,
		Bootloader:   spec.Bootloader,
	}

	if spec.KernelArgs != "" {
		params.ExtraKernelArgs = strings.Fields(spec.KernelArgs)
	}

	if spec.InstallExtensions != nil {
		params.Extensions = spec.InstallExtensions
	}

	if len(spec.MachineLabels) > 0 {
		params.Labels = make([]string, 0, len(spec.MachineLabels))
		for k, v := range spec.MachineLabels {
			if v != "" {
				params.Labels = append(params.Labels, k+"="+v)
			} else {
				params.Labels = append(params.Labels, k)
			}
		}
	}

	return params, nil
}

// FindImage finds an installation media resource by name/profile/overlay and returns its ImageInfo.
func FindImage(ctx context.Context, client *client.Client, name string, params Params) (ImageInfo, error) {
	result, err := filterMedia(ctx, client, func(val *omni.InstallationMedia) (*omni.InstallationMedia, bool) {
		spec := val.TypedSpec().Value

		if strings.EqualFold(name, "iso") {
			return val, strings.Contains(strings.ToLower(spec.Name), strings.ToLower(name))
		}

		return val, strings.EqualFold(spec.Name, name) ||
			strings.EqualFold(spec.Profile, name) || strings.EqualFold(spec.Overlay, name)
	})
	if err != nil {
		return ImageInfo{}, err
	}

	if len(result) > 1 {
		result = xslices.FilterInPlace(result, func(val *omni.InstallationMedia) bool {
			return val.TypedSpec().Value.Architecture == params.Architecture
		})
	}

	if len(result) == 0 {
		return ImageInfo{}, fmt.Errorf("no image found for %q", name)
	} else if len(result) > 1 {
		names := xslices.Map(result, func(val *omni.InstallationMedia) string {
			return val.Metadata().ID()
		})

		return ImageInfo{}, fmt.Errorf("multiple images found:\n  %s", strings.Join(names, "\n  "))
	}

	installationMedia := result[0]

	minTalosVersion := installationMedia.TypedSpec().Value.MinTalosVersion
	if minTalosVersion != "" {
		minVersion, versionErr := semver.ParseTolerant(minTalosVersion)
		if versionErr != nil {
			return ImageInfo{}, fmt.Errorf("failed to parse min Talos version supported by the installation media: %w", versionErr)
		}

		requestedVersion, versionErr := semver.ParseTolerant(params.TalosVersion)
		if versionErr != nil {
			return ImageInfo{}, fmt.Errorf("failed to parse requested Talos version: %w", versionErr)
		}

		requestedVersion.Pre = nil

		if requestedVersion.LT(minVersion) {
			return ImageInfo{}, fmt.Errorf("%s supports only Talos version >= %s", installationMedia.TypedSpec().Value.Name, minTalosVersion)
		}
	}

	return imageInfoFromResource(installationMedia), nil
}

func createSchematic(ctx context.Context, client *client.Client, image ImageInfo, params Params) (*management.CreateSchematicResponse, error) {
	metaValues := map[uint32]string{}

	var extensions []string

	if params.Labels != nil {
		labels, err := getMachineLabels(params.Labels)
		if err != nil {
			return nil, fmt.Errorf("failed to gen image labels: %w", err)
		}

		metaValues[meta.LabelsMeta] = string(labels)
	}

	var err error

	if params.Extensions != nil {
		extensions, err = getExtensions(ctx, client, params.TalosVersion, params.Extensions)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup extensions: %w", err)
		}
	}

	req := &management.CreateSchematicRequest{
		MetaValues:               metaValues,
		ExtraKernelArgs:          params.ExtraKernelArgs,
		Extensions:               extensions,
		SecureBoot:               params.SecureBoot,
		TalosVersion:             params.TalosVersion,
		SiderolinkGrpcTunnelMode: params.GrpcTunnelMode,
		JoinToken:                params.JoinToken,
		Bootloader:               params.Bootloader,
	}

	if params.Overlay != nil {
		req.Overlay = params.Overlay
	} else {
		req.MediaId = image.MediaID
	}

	resp, err := client.Management().CreateSchematic(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create schematic: %w", err)
	}

	return resp, nil
}

// DownloadImageTo downloads an installation media image and saves it to the output path.
func DownloadImageTo(ctx context.Context, client *client.Client, image ImageInfo, params Params) error {
	if params.SecureBoot && image.NoSecureBoot {
		return fmt.Errorf("%q doesn't support secure boot", image.Name)
	}

	schematicResp, err := createSchematic(ctx, client, image, params)
	if err != nil {
		return err
	}

	switch {
	case params.GrpcTunnelMode == management.CreateSchematicRequest_AUTO:
		fmt.Fprintf(os.Stderr, "Using server's SideroLink gRPC tunnel setting: %v\n", schematicResp.GrpcTunnelEnabled)
	case params.GrpcTunnelMode == management.CreateSchematicRequest_DISABLED && schematicResp.GrpcTunnelEnabled:
		fmt.Fprintf(os.Stderr, "WARNING: requested setting \"--use-siderolink-grpc-tunnel\" is ignored because the server's SideroLink gRPC tunnel setting is enabled.\n")
	}

	features, err := safe.ReaderGetByID[*omni.FeaturesConfig](ctx, client.Omni().State(), omni.FeaturesConfigID)
	if err != nil {
		return err
	}

	legacy := !quirks.New(params.TalosVersion).SupportsOverlay()

	generatedFilename := image.generateFilename(legacy, params.SecureBoot, true)

	if params.PXE {
		pxeFilename := image.generateFilename(legacy, params.SecureBoot, false)

		var pxeURL *url.URL

		pxeURL, err = url.Parse(features.TypedSpec().Value.ImageFactoryPxeBaseUrl)
		if err != nil {
			return err
		}

		fmt.Println(pxeURL.JoinPath("pxe", schematicResp.SchematicId, params.TalosVersion, pxeFilename).String())

		return nil
	}

	req, err := createDirectRequest(ctx, features.TypedSpec().Value.ImageFactoryBaseUrl, schematicResp.SchematicId, params.TalosVersion, generatedFilename, params.SecureBoot)
	if err != nil {
		return err
	}

	dest := params.Output
	if filepath.Ext(params.Output) == "" {
		dest = filepath.Join(params.Output, fmt.Sprintf(
			"%s-%s-%s%s",
			image.DestFilePrefix,
			params.TalosVersion,
			schematicResp.SchematicId[:6],
			filepath.Ext(generatedFilename),
		))
	}

	return downloadToFile(req, client.KeyProvider(), dest)
}

func filterMedia[T any](ctx context.Context, client *client.Client, check func(value *omni.InstallationMedia) (T, bool)) ([]T, error) {
	media, err := safe.StateListAll[*omni.InstallationMedia](
		ctx,
		client.Omni().State(),
	)
	if err != nil {
		return nil, err
	}

	result := make([]T, 0, media.Len())

	for item := range media.All() {
		if val, ok := check(item); ok {
			result = append(result, val)
		}
	}

	return result, nil
}

func createDirectRequest(ctx context.Context, baseURL, schematic, talosVersion, filename string, secureBoot bool) (*http.Request, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	u.Scheme = "https"

	u.Path, err = url.JoinPath(u.Path, "image", schematic, talosVersion, filename)
	if err != nil {
		return nil, err
	}

	if secureBoot {
		query := u.Query()
		query.Add("secureboot", "true")

		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, err
}

func signRequest(req *http.Request, provider *pgpclient.KeyProvider) error {
	identity, signer, err := getSigner(provider)
	if err != nil {
		return err
	}

	msg, err := message.NewHTTP(req)
	if err != nil {
		return err
	}

	return msg.Sign(identity, signer)
}

// getSigner returns the identity and the signer to use for signing the request.
func getSigner(provider *pgpclient.KeyProvider) (identity string, signer message.Signer, err error) {
	envKey, valueBase64 := serviceaccount.GetFromEnv()
	if envKey != "" {
		sa, saErr := serviceaccount.Decode(valueBase64)
		if saErr != nil {
			return "", nil, saErr
		}

		return sa.Name, sa.Key, nil
	}

	contextName, configCtx, err := currentConfigCtx()
	if err != nil {
		return "", nil, err
	}

	key, keyErr := provider.ReadValidKey(contextName, configCtx.Auth.SideroV1.Identity)
	if keyErr != nil {
		return "", nil, fmt.Errorf("failed to read key: %w", keyErr)
	}

	return configCtx.Auth.SideroV1.Identity, key, nil
}

// ParseLabelPairs converts label key=value pairs into a map.
func ParseLabelPairs(labelPairs []string) (map[string]string, error) {
	labels := map[string]string{}

	for _, l := range labelPairs {
		parts := strings.Split(l, "=")

		switch len(parts) {
		case 1:
			labels[parts[0]] = ""
		case 2:
			labels[parts[0]] = parts[1]
		default:
			return nil, fmt.Errorf("invalid label format %q, expected key=value", l)
		}
	}

	return labels, nil
}

// getMachineLabels converts label key=value pairs into YAML-encoded meta bytes.
func getMachineLabels(labelPairs []string) ([]byte, error) {
	labels, err := ParseLabelPairs(labelPairs)
	if err != nil {
		return nil, err
	}

	return meta.ImageLabels{Labels: labels}.Encode()
}

// getExtensions resolves extension short names to full extension names for the given Talos version.
func getExtensions(ctx context.Context, client *client.Client, talosVersion string, extensions []string) ([]string, error) {
	talosExtensions, err := safe.StateGet[*omni.TalosExtensions](
		ctx,
		client.Omni().State(),
		omni.NewTalosExtensions(strings.TrimLeft(talosVersion, "v")).Metadata(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get extensions for talos version %q: %w", talosVersion, err)
	}

	allNames := xslices.Map(talosExtensions.TypedSpec().Value.Items, func(e *specs.TalosExtensionsSpec_Info) string {
		return e.Name
	})

	result := make([]string, 0, len(extensions))

	for _, extension := range extensions {
		matched := xslices.Filter(allNames, func(item string) bool {
			if strings.Contains(item, extension) {
				fmt.Printf("Install Extension: %s\n", item)

				return true
			}

			return false
		})

		if len(matched) == 0 {
			return nil, fmt.Errorf("failed to find extension with name %q for talos version %q", extension, talosVersion)
		}

		result = append(result, matched...)
	}

	return result, nil
}

func currentConfigCtx() (name string, ctx *config.Context, err error) {
	conf, err := config.Current()
	if err != nil {
		return "", nil, err
	}

	contextName := conf.Context
	if access.CmdFlags.Context != "" {
		contextName = access.CmdFlags.Context
	}

	configCtx, err := conf.GetContext(contextName)
	if err != nil {
		return "", nil, err
	}

	return contextName, configCtx, nil
}

// downloadToFile signs and executes an HTTP request, then saves the response to dest.
// If dest is a directory, filename is used as the base name.
func downloadToFile(req *http.Request, provider *pgpclient.KeyProvider, dest string) error {
	err := signRequest(req, provider)
	if err != nil {
		return err
	}

	httpClient, err := newHTTPClient()
	if err != nil {
		return err
	}

	fmt.Println("Generating the image...")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	defer checkCloser(resp.Body)

	if resp.StatusCode >= http.StatusBadRequest {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return readErr
		}

		return fmt.Errorf("failed to download the installation media, error code: %d, message: %s", resp.StatusCode, body)
	}

	fmt.Printf("Downloading to %s\n", dest)

	err = downloadResponseTo(dest, resp)
	if err != nil {
		return err
	}

	fmt.Printf("Downloaded to %s\n", dest)

	return nil
}

func downloadResponseTo(dest string, resp *http.Response) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}

	defer checkCloser(f)

	_, err = io.Copy(f, resp.Body)

	return err
}

func checkCloser(c io.Closer) {
	if err := c.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error closing: %v", err)
	}
}

func newHTTPClient() (*http.Client, error) {
	httpTransport := http.DefaultTransport

	if access.CmdFlags.InsecureSkipTLSVerify {
		defaultTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, fmt.Errorf("unexpected default transport type: %T", http.DefaultTransport)
		}

		defaultTransportClone := defaultTransport.Clone()
		if defaultTransportClone.TLSClientConfig == nil {
			defaultTransportClone.TLSClientConfig = &tls.Config{}
		}

		defaultTransportClone.TLSClientConfig.InsecureSkipVerify = true

		httpTransport = defaultTransportClone
	}

	return &http.Client{
		Transport: httpTransport,
	}, nil
}

// MakePath ensures the output path exists.
func MakePath(path string) error {
	if filepath.Ext(path) != "" {
		ok, err := checkPath(path)
		if err != nil {
			return err
		}

		if ok {
			return fmt.Errorf("destination %s already exists", path)
		}

		path = filepath.Dir(path)
	}

	return os.MkdirAll(path, 0o755)
}

func checkPath(path string) (bool, error) {
	_, err := os.Stat(path)

	switch {
	case os.IsNotExist(err):
		return false, nil
	case err != nil:
		return false, err
	default:
		return true, nil
	}
}

// ImageCompletion provides shell completion for image names.
func ImageCompletion(architecture string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var results []string

	err := access.WithClient(
		func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
			res, err := filterMedia(ctx, client, func(value *omni.InstallationMedia) (string, bool) {
				spec := value.TypedSpec().Value
				if architecture != spec.Architecture {
					return "", false
				}

				name := spec.Name
				if toComplete == "" || strings.Contains(name, toComplete) {
					return name, true
				}

				return "", false
			})
			if err != nil {
				return err
			}

			results = res

			return nil
		},
	)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	return xslices.Deduplicate(results, func(s string) string { return s }), cobra.ShellCompDirectiveNoFileComp
}
