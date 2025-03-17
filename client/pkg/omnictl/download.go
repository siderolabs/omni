// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"fmt"
	"io"
	"mime"
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
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/config"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

const useSiderolinkGRPCTunnelFlag = "use-siderolink-grpc-tunnel"

// downloadCmdFlags represents the `download` command flags.
var downloadCmdFlags struct {
	architecture string

	output                  string
	talosVersion            string
	labels                  []string
	extraKernelArgs         []string
	extensions              []string
	pxe                     bool
	secureBoot              bool
	useSiderolinkGRPCTunnel bool
}

func init() {
	downloadCmd.Flags().BoolVar(&downloadCmdFlags.pxe, "pxe", false, "Print PXE URL and exit")
	downloadCmd.Flags().BoolVar(&downloadCmdFlags.secureBoot, "secureboot", false, "Download SecureBoot enabled installation media")
	downloadCmd.Flags().StringVar(&downloadCmdFlags.architecture, "arch", "amd64", "Image architecture to download (amd64, arm64)")
	downloadCmd.Flags().StringVar(&downloadCmdFlags.output, "output", ".", "Output file or directory, defaults to current working directory")
	downloadCmd.Flags().StringVar(&downloadCmdFlags.talosVersion, "talos-version", constants.DefaultTalosVersion, "Talos version to be used in the generated installation media")
	downloadCmd.Flags().StringSliceVar(&downloadCmdFlags.labels, "initial-labels", nil, "Bake initial labels into the generated installation media")
	downloadCmd.Flags().StringArrayVar(&downloadCmdFlags.extraKernelArgs, "extra-kernel-args", nil, "Add extra kernel args to the generated installation media")
	downloadCmd.Flags().StringSliceVar(&downloadCmdFlags.extensions, "extensions", nil, "Generate installation media with extensions pre-installed")
	downloadCmd.Flags().BoolVar(&downloadCmdFlags.useSiderolinkGRPCTunnel, useSiderolinkGRPCTunnelFlag, false,
		"Configure Talos to use the SideroLink (WireGuard) gRPC tunnel over HTTP2 for Omni management traffic, instead of UDP. Note that this will add overhead to the traffic.")

	RootCmd.AddCommand(downloadCmd)
}

// downloadCmd represents the download command.
var downloadCmd = &cobra.Command{
	Use:   "download <image name>",
	Short: "Download installer media",
	Long: `This command downloads installer media from the server

It accepts one argument, which is the name of the image to download. Name can be one of the following:

     * iso - downloads the latest ISO image
     * AWS AMI (amd64), Vultr (arm64), Raspberry Pi 4 Model B - full image name
     * oracle, aws, vmware - platform name
     * rpi_generic, rockpi_4c, rock64 - board name

To get the full list of available images, look at the output of the following command:
    omnictl get installationmedia -o yaml

The download command tries to match the passed string in this order:

    * name
    * profile

By default it will download amd64 image if there are multiple images available for the same name.

For example, to download the latest ISO image for arm64, run:

    omnictl download iso --arch amd64

To download the same ISO with two extensions added, the --extensions argument gets repeated to produce a stringArray:

    omnictl download iso --arch amd64 --extensions intel-ucode --extensions qemu-guest-agent

To download the latest Vultr image, run:

    omnictl download "vultr"

To download the latest Radxa ROCK PI 4 image, run:

    omnictl download "rpi_generic"
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return access.WithClient(func(ctx context.Context, client *client.Client) error {
			if args[0] == "" {
				return fmt.Errorf("image name is required")
			}

			output, err := filepath.Abs(downloadCmdFlags.output)
			if err != nil {
				return err
			}

			err = makePath(output)
			if err != nil {
				return err
			}

			image, err := findImage(ctx, client, args[0], downloadCmdFlags.architecture)
			if err != nil {
				return err
			}

			grpcTunnelMode := management.CreateSchematicRequest_AUTO

			if cmd.Flags().Changed(useSiderolinkGRPCTunnelFlag) {
				if downloadCmdFlags.useSiderolinkGRPCTunnel {
					grpcTunnelMode = management.CreateSchematicRequest_ENABLED
				} else {
					grpcTunnelMode = management.CreateSchematicRequest_DISABLED
				}
			}

			return downloadImageTo(ctx, client, image, output, grpcTunnelMode)
		})
	},
	ValidArgsFunction: downloadCompletion,
}

func findImage(ctx context.Context, client *client.Client, name, arch string) (*omni.InstallationMedia, error) {
	result, err := filterMedia(ctx, client, func(val *omni.InstallationMedia) (*omni.InstallationMedia, bool) {
		spec := val.TypedSpec().Value

		if strings.EqualFold(name, "iso") {
			return val, strings.Contains(strings.ToLower(spec.Name), strings.ToLower(name))
		}

		return val, strings.EqualFold(spec.Name, name) ||
			strings.EqualFold(spec.Profile, name)
	})
	if err != nil {
		return nil, err
	}

	if len(result) > 1 {
		result = xslices.FilterInPlace(result, func(val *omni.InstallationMedia) bool {
			return val.TypedSpec().Value.Architecture == arch
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no image found for %q", name)
	} else if len(result) > 1 {
		names := xslices.Map(result, func(val *omni.InstallationMedia) string {
			return val.Metadata().ID()
		})

		return nil, fmt.Errorf("multiple images found:\n  %s", strings.Join(names, "\n  "))
	}

	minTalosVersion := result[0].TypedSpec().Value.MinTalosVersion
	if minTalosVersion != "" {
		minVersion, err := semver.ParseTolerant(minTalosVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse min Talos version supported by the installation media: %w", err)
		}

		requestedVersion, err := semver.ParseTolerant(downloadCmdFlags.talosVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse requested Talos version: %w", err)
		}

		requestedVersion.Pre = nil

		if requestedVersion.LT(minVersion) {
			return nil, fmt.Errorf("%s supports only Talos version >= %s", result[0].TypedSpec().Value.Name, minTalosVersion)
		}
	}

	return result[0], nil
}

func createSchematic(ctx context.Context, client *client.Client, media *omni.InstallationMedia,
	grpcTunnelMode management.CreateSchematicRequest_SiderolinkGRPCTunnelMode,
) (*management.CreateSchematicResponse, error) {
	metaValues := map[uint32]string{}

	var extensions []string

	if downloadCmdFlags.labels != nil {
		labels, err := getMachineLabels()
		if err != nil {
			return nil, fmt.Errorf("failed to gen image labels: %w", err)
		}

		metaValues[meta.LabelsMeta] = string(labels)
	}

	var err error

	if downloadCmdFlags.extensions != nil {
		extensions, err = getExtensions(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup extensions: %w", err)
		}
	}

	resp, err := client.Management().CreateSchematic(ctx, &management.CreateSchematicRequest{
		MetaValues:               metaValues,
		ExtraKernelArgs:          downloadCmdFlags.extraKernelArgs,
		Extensions:               extensions,
		MediaId:                  media.Metadata().ID(),
		SecureBoot:               downloadCmdFlags.secureBoot,
		TalosVersion:             downloadCmdFlags.talosVersion,
		SiderolinkGrpcTunnelMode: grpcTunnelMode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create schematic: %w", err)
	}

	return resp, nil
}

func downloadImageTo(ctx context.Context, client *client.Client, media *omni.InstallationMedia, output string, grpcTunnelMode management.CreateSchematicRequest_SiderolinkGRPCTunnelMode) error {
	schematicResp, err := createSchematic(ctx, client, media, grpcTunnelMode)
	if err != nil {
		return err
	}

	switch {
	case grpcTunnelMode == management.CreateSchematicRequest_AUTO:
		fmt.Fprintf(os.Stderr, "Using server's SideroLink gRPC tunnel setting: %v\n", schematicResp.GrpcTunnelEnabled)
	case grpcTunnelMode == management.CreateSchematicRequest_DISABLED && schematicResp.GrpcTunnelEnabled:
		fmt.Fprintf(os.Stderr, `WARNING: requested setting "--%s" is ignored because the server's SideroLink gRPC tunnel setting is enabled.\n`, useSiderolinkGRPCTunnelFlag)
	}

	if media.TypedSpec().Value.NoSecureBoot && downloadCmdFlags.secureBoot {
		return fmt.Errorf("%q doesn't support secure boot", media.TypedSpec().Value.Name)
	}

	if downloadCmdFlags.pxe {
		fmt.Println(schematicResp.PxeUrl)

		return nil
	}

	req, err := createRequest(ctx, client, schematicResp.SchematicId, media)
	if err != nil {
		return err
	}

	err = signRequest(req)
	if err != nil {
		return err
	}

	httpTransport := http.DefaultTransport

	if access.CmdFlags.InsecureSkipTLSVerify {
		defaultTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return fmt.Errorf("unexpected default transport type: %T", http.DefaultTransport)
		}

		defaultTransportClone := defaultTransport.Clone()
		defaultTransportClone.TLSClientConfig.InsecureSkipVerify = true

		httpTransport = defaultTransportClone
	}

	httpClient := &http.Client{
		Transport: httpTransport,
	}

	fmt.Println("Generating the image...")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	defer checkCloser(resp.Body)

	if resp.StatusCode >= http.StatusBadRequest {
		var message []byte

		message, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		return fmt.Errorf("failed to download the installation media, error code: %d, message: %s", resp.StatusCode, message)
	}

	dest := output

	if filepath.Ext(output) == "" {
		disposition := resp.Header.Get("Content-Disposition")

		if disposition == "" {
			return fmt.Errorf("no content disposition header in the server response")
		}

		var params map[string]string

		_, params, err = mime.ParseMediaType(disposition)
		if err != nil {
			return fmt.Errorf("failed to parse content disposition header: %w", err)
		}

		filename, ok := params["filename"]
		if !ok {
			return fmt.Errorf("failed to auto-detect filename from the response headers, filename is not present in the content disposition header")
		}

		dest = filepath.Join(output, filename)
	}

	fmt.Printf("Downloading %s to %s\n", media.Metadata().ID(), dest)

	err = downloadResponseTo(dest, resp)
	if err != nil {
		return err
	}

	fmt.Printf("Downloaded %s to %s\n", media.Metadata().ID(), dest)

	return nil
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

func createRequest(ctx context.Context, client *client.Client, schematic string, image *omni.InstallationMedia) (*http.Request, error) {
	u, err := url.Parse(client.Endpoint())
	if err != nil {
		return nil, err
	}

	u.Scheme = "https"

	u.Path, err = url.JoinPath(u.Path, "image", schematic, downloadCmdFlags.talosVersion, image.Metadata().ID())
	if err != nil {
		return nil, err
	}

	if downloadCmdFlags.secureBoot {
		query := u.Query()
		query.Add(constants.SecureBoot, "true")

		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, err
}

func signRequest(req *http.Request) error {
	identity, signer, err := getSigner()
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
//
// It can be a service account or a user key.
func getSigner() (identity string, signer message.Signer, err error) {
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

	provider := pgpclient.NewKeyProvider("omni/keys")

	key, keyErr := provider.ReadValidKey(contextName, configCtx.Auth.SideroV1.Identity)
	if keyErr != nil {
		return "", nil, fmt.Errorf("failed to read key: %w", err)
	}

	return configCtx.Auth.SideroV1.Identity, key, nil
}

func getMachineLabels() ([]byte, error) {
	labels := map[string]string{}

	for _, l := range downloadCmdFlags.labels {
		parts := strings.Split(l, "=")
		if len(parts) > 2 {
			return nil, fmt.Errorf("malformed label %s", l)
		}

		value := ""

		if len(parts) > 1 {
			value = parts[1]
		}

		labels[parts[0]] = value
	}

	return yaml.Marshal(meta.ImageLabels{
		Labels: labels,
	})
}

func getExtensions(ctx context.Context, client *client.Client) ([]string, error) {
	extensions, err := safe.StateGet[*omni.TalosExtensions](
		ctx,
		client.Omni().State(),
		omni.NewTalosExtensions(resources.DefaultNamespace, strings.TrimLeft(downloadCmdFlags.talosVersion, "v")).Metadata(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get extensions for talos version %q: %w", downloadCmdFlags.talosVersion, err)
	}

	result := make([]string, 0, len(downloadCmdFlags.extensions))

	for _, extension := range downloadCmdFlags.extensions {
		items := xslices.Map(extensions.TypedSpec().Value.Items, func(e *specs.TalosExtensionsSpec_Info) string {
			return e.Name
		})

		items = xslices.FilterInPlace(items, func(item string) bool {
			if strings.Contains(item, extension) {
				fmt.Printf("Install Extension: %s\n", item)

				return true
			}

			return false
		})

		if len(items) == 0 {
			return nil, fmt.Errorf("failed to find extension with name %q for talos version %q", extension, downloadCmdFlags.talosVersion)
		}

		result = append(result, items...)
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
		fmt.Printf("error closing: %v", err)
	}
}

func downloadCompletion(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var results []string

	err := access.WithClient(
		func(ctx context.Context, client *client.Client) error {
			res, err := filterMedia(ctx, client, func(value *omni.InstallationMedia) (string, bool) {
				spec := value.TypedSpec().Value
				if downloadCmdFlags.architecture != spec.Architecture {
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

	return dedupInplace(results), cobra.ShellCompDirectiveNoFileComp
}

func dedupInplace(results []string) []string {
	seen := make(map[string]struct{}, len(results))
	j := 0

	for _, r := range results {
		if _, ok := seen[r]; !ok {
			seen[r] = struct{}{}
			results[j] = r
			j++
		}
	}

	return results[:j]
}

func makePath(path string) error {
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

	ok, err := checkPath(path)
	if err != nil {
		return err
	}

	if !ok {
		if dirErr := os.MkdirAll(path, 0o755); dirErr != nil {
			return dirErr
		}
	}

	return nil
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
