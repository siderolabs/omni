// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

const (
	// grpcTunnelQueryParam is the query parameter key for enabling SideroLink gRPC tunnel.
	grpcTunnelQueryParam = "grpc_tunnel"
)

// APIURLOptions provides extra args to the APIURL method.
type APIURLOptions struct {
	grpcTunnel *bool
	token      string
}

// APIURLOption provides extra arg to the APIURL method.
type APIURLOption func(*APIURLOptions)

// WithJoinToken overrides token value from the ConnectionParams with the custom one.
func WithJoinToken(token string) APIURLOption {
	return func(a *APIURLOptions) {
		a.token = token
	}
}

// WithGRPCTunnel overrides default value for the grpc tunnel.
func WithGRPCTunnel(enabled bool) APIURLOption {
	return func(a *APIURLOptions) {
		a.grpcTunnel = pointer.To(enabled)
	}
}

// NewConnectionParams creates new ConnectionParams state.
func NewConnectionParams(ns, id string) *ConnectionParams {
	return typed.NewResource[ConnectionParamsSpec, ConnectionParamsExtension](
		resource.NewMetadata(ns, ConnectionParamsType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(&specs.ConnectionParamsSpec{}),
	)
}

// ConnectionParamsType is the type of ConnectionParams resource.
//
// tsgen:ConnectionParamsType
const ConnectionParamsType = resource.Type("ConnectionParams.omni.sidero.dev")

// ConnectionParams resource keeps generated kernel arguments as a resource.
//
// ConnectionParams resource ID is a machine UUID.
type ConnectionParams = typed.Resource[ConnectionParamsSpec, ConnectionParamsExtension]

// ConnectionParamsSpec wraps specs.ConnectionParamsSpec.
type ConnectionParamsSpec = protobuf.ResourceSpec[specs.ConnectionParamsSpec, *specs.ConnectionParamsSpec]

// ConnectionParamsExtension providers auxiliary methods for ConnectionParams resource.
type ConnectionParamsExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ConnectionParamsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ConnectionParamsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: Namespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "JoinToken",
				JSONPath: "{.jointoken}",
			},
			{
				Name:     "API",
				JSONPath: "{.apiendpoint}",
			},
			{
				Name:     "Wireguard",
				JSONPath: "{.wireguardendpoint}",
			},
			{
				Name:     "GRPC Tunnel",
				JSONPath: "{.usegrpctunnel}",
			},
		},
	}
}

// KernelArgs returns the kernel args for the given ConnectionParams resource.
func KernelArgs(res *ConnectionParams) []string {
	if res == nil {
		return nil
	}

	if res.TypedSpec().Value.Args == "" {
		return nil
	}

	return strings.Split(res.TypedSpec().Value.Args, " ")
}

// APIURL generates siderolink API URL from the connection params.
func APIURL(cfg *ConnectionParams, options ...APIURLOption) (string, error) {
	apiURL, err := url.Parse(cfg.TypedSpec().Value.ApiEndpoint)
	if err != nil {
		return "", err
	}

	opts := APIURLOptions{
		token: cfg.TypedSpec().Value.JoinToken,
	}

	for _, o := range options {
		o(&opts)
	}

	query := apiURL.Query()
	query.Set("jointoken", opts.token)

	// Enable the GRPC tunnel only when:
	// - It is explicitly set in the options, and it true, or
	// - It is not explicitly set in the options, but it is enabled in the connection params.
	if (opts.grpcTunnel != nil && *opts.grpcTunnel) ||
		(opts.grpcTunnel == nil && cfg.TypedSpec().Value.UseGrpcTunnel) {
		query.Set(grpcTunnelQueryParam, "true")
	}

	apiURL.RawQuery = query.Encode()

	return apiURL.String(), nil
}

// GetConnectionArgsForProvider composes connection args for the specific provider.
func GetConnectionArgsForProvider(connectionParams *ConnectionParams,
	providerID string, grpcTunnel specs.GrpcTunnelMode, opts ...JoinConfigOption,
) (string, error) {
	params := KernelArgs(connectionParams)
	if len(params) == 0 {
		return "", errors.New("failed to get the connection params")
	}

	token, err := getJoinTokenWithExtraData(connectionParams.TypedSpec().Value.JoinToken, providerID, opts...)
	if err != nil {
		return "", err
	}

	data, err := token.Encode()
	if err != nil {
		return "", fmt.Errorf("failed to encode the siderolink token")
	}

	if err = replaceQuerySiderolinkAPIURLQueryValue(params, "jointoken", data); err != nil {
		return "", err
	}

	if grpcTunnel != specs.GrpcTunnelMode_UNSET {
		if err = replaceQuerySiderolinkAPIURLQueryValue(params, "grpc_tunnel", strconv.FormatBool(grpcTunnel == specs.GrpcTunnelMode_ENABLED)); err != nil {
			return "", err
		}
	}

	return strings.Join(params, " "), nil
}

// JoinConfigOption represents a single optional arg to the GetJoinConfigForProvider function.
type JoinConfigOption func(*JoinConfigOptions)

// JoinConfigOptions is the additional options to the GetJoinConfigForProvider function.
type JoinConfigOptions struct {
	requestID string
}

// WithEncodeRequestID additionally encodes the request ID right in the join token.
// NOTE: use only when the join tokens are passed to the nodes though the config and not baked into the schematics.
// Otherwise it will blow up the schematics count in the factory.
func WithEncodeRequestID(requestID string) JoinConfigOption {
	return func(opts *JoinConfigOptions) {
		opts.requestID = requestID
	}
}

func getJoinTokenWithExtraData(token, providerID string, opts ...JoinConfigOption) (jointoken.JoinToken, error) {
	var options JoinConfigOptions

	for _, opt := range opts {
		opt(&options)
	}

	extraData := map[string]string{
		omni.LabelInfraProviderID: providerID,
	}

	if options.requestID != "" {
		extraData[omni.LabelMachineRequest] = options.requestID
	}

	return jointoken.NewWithExtraData(token, extraData)
}

// GetJoinConfigForProvider composes Omni join config.
func GetJoinConfigForProvider(connectionParams *ConnectionParams, providerID string, grpcTunnel specs.GrpcTunnelMode, joinTokenOpts ...JoinConfigOption) (string, error) {
	token, err := getJoinTokenWithExtraData(connectionParams.TypedSpec().Value.JoinToken, providerID, joinTokenOpts...)
	if err != nil {
		return "", err
	}

	tokenString, err := token.Encode()
	if err != nil {
		return "", err
	}

	opts := []APIURLOption{
		WithJoinToken(tokenString),
	}

	if grpcTunnel != specs.GrpcTunnelMode_UNSET {
		opts = append(opts, WithGRPCTunnel(grpcTunnel == specs.GrpcTunnelMode_ENABLED))
	}

	apiURL, err := APIURL(connectionParams, opts...)
	if err != nil {
		return "", err
	}

	siderolink := siderolink.NewConfigV1Alpha1()
	siderolink.APIUrlConfig.URL = ensure.Value(url.Parse(apiURL))

	events := runtime.NewEventSinkV1Alpha1()
	events.Endpoint = fmt.Sprintf("[fdae:41e4:649b:9303::1]:%d", connectionParams.TypedSpec().Value.EventsPort)

	logs := runtime.NewKmsgLogV1Alpha1()
	logs.KmsgLogURL.URL = ensure.Value(url.Parse(fmt.Sprintf("tcp://[fdae:41e4:649b:9303::1]:%d", connectionParams.TypedSpec().Value.LogsPort)))
	logs.MetaName = "omni-kmsg"

	var buf bytes.Buffer

	encoder := yaml.NewEncoder(&buf)

	for _, cfg := range []any{
		siderolink,
		events,
		logs,
	} {
		if err = encoder.Encode(cfg); err != nil {
			return "", err
		}
	}

	return buf.String(), nil
}

// KernelArgsWithGRPCRTunnelMode returns kernel args from the given connection params, overwriting the gRPC tunnel mode with the provided value in SideroLink API URL.
func KernelArgsWithGRPCRTunnelMode(connectionParams *ConnectionParams, enabled bool) ([]string, error) {
	params := KernelArgs(connectionParams)
	if len(params) == 0 {
		return nil, errors.New("failed to get the connection params")
	}

	if err := replaceQuerySiderolinkAPIURLQueryValue(params, grpcTunnelQueryParam, strconv.FormatBool(enabled)); err != nil {
		return nil, err
	}

	return params, nil
}

func replaceQuerySiderolinkAPIURLQueryValue(params []string, queryKey, queryValue string) error {
	if len(params) == 0 {
		return errors.New("failed to get the connection params")
	}

	index := slices.IndexFunc(params, func(p string) bool {
		return strings.HasPrefix(p, constants.KernelParamSideroLink)
	})

	if index == -1 {
		return errors.New("malformed connection params string: doesn't contain siderolink api arg")
	}

	key, value, found := strings.Cut(params[index], "=")
	if !found {
		return errors.New("failed to parse siderolink connection param")
	}

	siderolinkURL, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("failed to parse siderolink connection param: %w", err)
	}

	query := siderolinkURL.Query()

	query.Set(queryKey, queryValue)

	siderolinkURL.RawQuery = query.Encode()

	params[index] = key + "=" + siderolinkURL.String()

	return nil
}
