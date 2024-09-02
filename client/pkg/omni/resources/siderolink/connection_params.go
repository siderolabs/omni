// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/talos/pkg/machinery/constants"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

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
func APIURL(cfg *ConnectionParams, useTunnel bool) (string, error) {
	var url *url.URL

	url, err := url.Parse(cfg.TypedSpec().Value.ApiEndpoint)
	if err != nil {
		return "", err
	}

	query := url.Query()
	query.Set("jointoken", cfg.TypedSpec().Value.JoinToken)
	query.Set("grpc_tunnel", fmt.Sprintf("%t", useTunnel))
	url.RawQuery = query.Encode()

	return url.String(), nil
}

// GetConnectionArgsForProvider composes connection args for the specific provider.
func GetConnectionArgsForProvider(connectionParams *ConnectionParams, providerID string) (string, error) {
	params := KernelArgs(connectionParams)
	if len(params) == 0 {
		return "", errors.New("failed to get the connection params")
	}

	index := slices.IndexFunc(params, func(p string) bool {
		return strings.HasPrefix(p, constants.KernelParamSideroLink)
	})

	if index == -1 {
		return "", errors.New("malformed connection params string: doesn't contain siderolink api arg")
	}

	key, value, found := strings.Cut(params[index], "=")
	if !found {
		return "", errors.New("failed to parse siderolink connection param")
	}

	url, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("failed to parse siderolink connection param: %w", err)
	}

	token, err := jointoken.NewWithExtraData(connectionParams.TypedSpec().Value.JoinToken, map[string]string{
		omni.LabelInfraProviderID: providerID,
	})
	if err != nil {
		return "", err
	}

	data, err := token.Encode()
	if err != nil {
		return "", fmt.Errorf("failed to encode the siderolink token")
	}

	query := url.Query()
	query.Set("jointoken", data)

	url.RawQuery = query.Encode()

	params[index] = key + "=" + url.String()

	return strings.Join(params, " "), nil
}
