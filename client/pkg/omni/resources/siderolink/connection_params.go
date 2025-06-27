// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"net/url"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/omni/client/api/omni/specs"
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
