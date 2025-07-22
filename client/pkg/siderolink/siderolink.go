// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package siderolink provides the function for encoding machine join configs and kernel args.
package siderolink

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"

	"github.com/siderolabs/siderolink/pkg/wireguard"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	siderolinkmachinery "github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/constants"

	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// JoinConfigOptions is the struct with all optional args for the RenderJoinConfig function.
type JoinConfigOptions struct {
	extraTokenData              map[string]string
	joinToken                   string
	machineAPIURL               string
	version                     string
	useGRPCTunnel               bool
	withoutSiderolinkJoinConfig bool
	logServerPort               int
	eventSinkPort               int
}

// JoinConfigOption is the additional options for the RenderJoinConfig function.
type JoinConfigOption func(*JoinConfigOptions)

// WithGRPCTunnel overrides the gRPC tunnel config.
func WithGRPCTunnel(value bool) JoinConfigOption {
	return func(opts *JoinConfigOptions) {
		opts.useGRPCTunnel = value
	}
}

// WithoutMachineAPIURL allows generating the config without the SideroLinkAPI section.
func WithoutMachineAPIURL() JoinConfigOption {
	return func(opts *JoinConfigOptions) {
		opts.withoutSiderolinkJoinConfig = true
	}
}

// WithMachine renders the connection for the particular machine, it will take care of keeping
// the gRPC tunnel connection enabled, will populate the machine request ID if it's there.
func WithMachine(machine *omni.Machine) JoinConfigOption {
	return func(opts *JoinConfigOptions) {
		opts.useGRPCTunnel = machine.TypedSpec().Value.UseGrpcTunnel

		if opts.extraTokenData == nil {
			opts.extraTokenData = map[string]string{}
		}

		if machineRequest, ok := machine.Metadata().Labels().Get(omni.LabelMachineRequest); ok {
			opts.extraTokenData[omni.LabelMachineRequest] = machineRequest
		}

		if providerID, ok := machine.Metadata().Annotations().Get(omni.LabelInfraProviderID); ok {
			opts.extraTokenData[omni.LabelInfraProviderID] = providerID
		}
	}
}

// WithJoinTokenVersion sets the version of the join token.
//
// If not set, it will default to jointoken.Version2.
func WithJoinTokenVersion(version string) JoinConfigOption {
	return func(opts *JoinConfigOptions) {
		opts.version = version
	}
}

// WithMachineRequestID renders the connection with the machine request ID encoded into the join token.
func WithMachineRequestID(id string) JoinConfigOption {
	return func(opts *JoinConfigOptions) {
		if opts.extraTokenData == nil {
			opts.extraTokenData = map[string]string{}
		}

		opts.extraTokenData[omni.LabelMachineRequest] = id
	}
}

// WithMachineAPIURL sets the machine API URL.
// Without the API URL it will not render the SideroLink config section.
func WithMachineAPIURL(value string) JoinConfigOption {
	return func(opts *JoinConfigOptions) {
		opts.machineAPIURL = value
	}
}

// WithEventSinkPort specifies event sink port.
// Without the port it will not render the EventSink config section.
func WithEventSinkPort(value int) JoinConfigOption {
	return func(opts *JoinConfigOptions) {
		opts.eventSinkPort = value
	}
}

// WithLogServerPort specifies log server port.
// Without the port it will not render the KmsgLog config section.
func WithLogServerPort(value int) JoinConfigOption {
	return func(opts *JoinConfigOptions) {
		opts.logServerPort = value
	}
}

// WithProvider renders the join config for the provider.
func WithProvider(provider *infra.Provider) JoinConfigOption {
	return func(opts *JoinConfigOptions) {
		if opts.extraTokenData == nil {
			opts.extraTokenData = map[string]string{}
		}

		opts.extraTokenData[omni.LabelInfraProviderID] = provider.Metadata().ID()
	}
}

// WithJoinToken adds the join token to the machine API URL.
// Without the token it will try to use tokenless access to Omni.
func WithJoinToken(token string) JoinConfigOption {
	return func(opts *JoinConfigOptions) {
		opts.joinToken = token
	}
}

// JoinOptions is the intermediate struct used in the kernel args and the partial config generation.
type JoinOptions struct {
	apiURL            *url.URL
	logServerURL      *url.URL
	eventSinkEndpoint string
}

// NewJoinOptions creates the new JoinOptions.
func NewJoinOptions(opts ...JoinConfigOption) (*JoinOptions, error) {
	var (
		options JoinConfigOptions
		res     JoinOptions
	)

	for _, o := range opts {
		o(&options)
	}

	if options.version == "" {
		options.version = jointoken.Version2
	}

	if options.logServerPort == 0 {
		return nil, errors.New("no log server port")
	}

	if options.eventSinkPort == 0 {
		return nil, errors.New("no events server port")
	}

	if options.machineAPIURL == "" && !options.withoutSiderolinkJoinConfig {
		return nil, errors.New("no machine API URL")
	}

	if options.machineAPIURL != "" {
		if options.joinToken == "" {
			return nil, errors.New("no join token")
		}

		url, err := url.Parse(options.machineAPIURL)
		if err != nil {
			return nil, err
		}

		query := url.Query()

		token, err := encodeToken(options)
		if err != nil {
			return nil, err
		}

		query.Add("jointoken", token)

		if options.useGRPCTunnel {
			query.Add("grpc_tunnel", "true")
		}

		url.RawQuery = query.Encode()

		res.apiURL = url
	}

	listenHost := GetListenHost()

	res.eventSinkEndpoint = net.JoinHostPort(listenHost, strconv.Itoa(options.eventSinkPort))

	kmsgLogURL, err := url.Parse("tcp://" + net.JoinHostPort(listenHost, strconv.Itoa(options.logServerPort)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse kmsg log URL: %w", err)
	}

	res.logServerURL = kmsgLogURL

	return &res, nil
}

// GetKernelArgs creates the Talos kernel arguments from the JoinOptions.
func (opts *JoinOptions) GetKernelArgs() []string {
	args := make([]string, 0, 3)

	if opts.apiURL != nil {
		args = append(args, fmt.Sprintf("%s=%s", constants.KernelParamSideroLink, opts.apiURL.String()))
	}

	if opts.eventSinkEndpoint != "" {
		args = append(args, fmt.Sprintf("%s=%s", constants.KernelParamEventsSink, opts.eventSinkEndpoint))
	}

	if opts.logServerURL != nil {
		args = append(args, fmt.Sprintf("%s=%s", constants.KernelParamLoggingKernel, opts.logServerURL.String()))
	}

	return args
}

// RenderJoinConfig creates the raw join config from the JoinOptions.
func (opts *JoinOptions) RenderJoinConfig() ([]byte, error) {
	var docs []config.Document

	if opts.apiURL != nil {
		siderolinkConfig := siderolinkmachinery.NewConfigV1Alpha1()
		siderolinkConfig.APIUrlConfig.URL = opts.apiURL

		docs = append(docs, siderolinkConfig)
	}

	if opts.eventSinkEndpoint != "" {
		eventSinkConfig := runtime.NewEventSinkV1Alpha1()
		eventSinkConfig.Endpoint = opts.eventSinkEndpoint

		docs = append(docs, eventSinkConfig)
	}

	if opts.logServerURL != nil {
		kmsgLogConfig := runtime.NewKmsgLogV1Alpha1()
		kmsgLogConfig.MetaName = "omni-kmsg"
		kmsgLogConfig.KmsgLogURL = meta.URL{
			URL: opts.logServerURL,
		}

		docs = append(docs, kmsgLogConfig)
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents were added, the args should have either of the following options: WithMachineAPIURL, WithEventSinkPort, WithLogServerPort")
	}

	configContainer, err := container.New(docs...)
	if err != nil {
		return nil, fmt.Errorf("failed to create config container: %w", err)
	}

	return configContainer.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
}

func encodeToken(options JoinConfigOptions) (string, error) {
	token, err := jointoken.Parse(options.joinToken)
	if err != nil {
		return "", err
	}

	// if the token is already encoded do nothing
	if token.Version != jointoken.VersionPlain {
		return options.joinToken, nil
	}

	if len(options.extraTokenData) != 0 {
		jt, err := jointoken.NewWithExtraData(options.joinToken, options.version, options.extraTokenData)
		if err != nil {
			return "", err
		}

		return jt.Encode()
	}

	return options.joinToken, nil
}

// GetListenHost returns the default Omni wireguard peer endpoint inside the virtual network.
func GetListenHost() string {
	siderolinkNetworkPrefix := wireguard.NetworkPrefix("")

	return netip.PrefixFrom(siderolinkNetworkPrefix.Addr().Next(), siderolinkNetworkPrefix.Bits()).Addr().String()
}
