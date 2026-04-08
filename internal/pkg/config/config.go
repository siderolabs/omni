// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package config contains the application config loading functions.
package config

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xyaml"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/compression"
	"github.com/siderolabs/omni/client/pkg/omni/resources/common"
	"github.com/siderolabs/omni/internal/pkg/config/validations"
	"github.com/siderolabs/omni/internal/pkg/jsonschema"
)

const wireguardDefaultPort = "50180"

//go:embed schema.json
var schemaData string

// ParseSchema parses the embedded JSON schema for the Omni config.
func ParseSchema() (*jsonschema.Schema, error) {
	return jsonschema.Parse("omni", schemaData)
}

// ParseOption describes an additional optional arg to the parseConfig function.
type ParseOption func(*ParseOptions)

// ParseOptions describes additional options for parsing the Omni config.
type ParseOptions struct {
	ignoreUnknownFields bool
}

// WithIgnoreUnknownFields ignores the unknown fields present in the config file.
func WithIgnoreUnknownFields() ParseOption {
	return func(po *ParseOptions) {
		po.ignoreUnknownFields = true
	}
}

// FromBytes loads the config from bytes.
func FromBytes(data []byte) (*Params, error) {
	return parseConfig(bytes.NewBuffer(data))
}

// LoadFromFile loads the config from the file.
func LoadFromFile(path string, opts ...ParseOption) (*Params, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close() //nolint:errcheck

	return parseConfig(f, opts...)
}

// LoadDefault creates the new default configuration by loading static defaults from the schema and applying dynamic defaults on top.
func LoadDefault() (*Params, error) {
	p, err := defaultsFromSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to load defaults from schema: %w", err)
	}

	// Dynamic defaults that depend on runtime state and cannot be expressed in the schema.
	p.Services.Siderolink.WireGuard.SetAdvertisedEndpoint(net.JoinHostPort(localIP, wireguardDefaultPort))
	p.Services.MachineAPI.SetEndpoint(net.JoinHostPort(localIP, "8090"))
	p.Logs.ResourceLogger.Types = common.UserManagedResourceTypes

	return p, nil
}

// Default creates the new default configuration by loading static defaults from the schema and applying dynamic defaults on top.
//
// If it fails, it panics.
func Default() *Params {
	p, err := LoadDefault()
	if err != nil {
		panic(err)
	}

	return p
}

// Init the config using defaults, merge with overrides, populate fallbacks and validate.
func Init(logger *zap.Logger, schema *jsonschema.Schema, params ...*Params) (*Params, error) {
	config, err := LoadDefault()
	if err != nil {
		return nil, err
	}

	for _, override := range params {
		if err := merge.Merge(config, override); err != nil {
			return nil, err
		}
	}

	config.PopulateFallbacks()

	if err := config.Validate(schema); err != nil {
		return nil, err
	}

	enableCompression := config.Features.GetEnableConfigDataCompression()
	if err := compression.InitConfig(enableCompression); err != nil {
		return nil, err
	}

	logger.Info("initialized resource compression config", zap.Bool("enabled", enableCompression))

	return config, nil
}

func parseConfig(r io.Reader, opts ...ParseOption) (*Params, error) {
	var options ParseOptions

	for _, o := range opts {
		o(&options)
	}

	if options.ignoreUnknownFields {
		var config Params

		return &config, yaml.NewDecoder(r).Decode(&config)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var config Params

	if err := xyaml.UnmarshalStrict(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ValidateState validate Omni params against the current state of Omni instance.
// Add any hooks that would need to validate the config against the state here.
func (p *Params) ValidateState(ctx context.Context, st state.State) error {
	if p.Services.Siderolink.GetJoinTokensMode() == SiderolinkServiceJoinTokensModeStrict {
		if err := validations.EnsureAllMachinesSupportStrictTokens(ctx, st); err != nil {
			return err
		}
	}

	return nil
}

// Validate Omni params.
func (p *Params) Validate(schema *jsonschema.Schema) error {
	if schema == nil {
		return errors.New("schema is nil")
	}

	var sb strings.Builder

	encoder := yaml.NewEncoder(&sb)
	encoder.SetIndent(2)

	if err := encoder.Encode(p); err != nil {
		return fmt.Errorf("failed to encode config to YAML for validation: %w", err)
	}

	configYAML := sb.String()

	if err := schema.Validate(configYAML); err != nil {
		return fmt.Errorf("failed to validate config against JSON schema: %w", err)
	}

	return nil
}

var localIP = getLocalIPOrEmpty()

// GetImageFactoryPXEBaseURL reads image factory PXE address from the args.
func (p *Params) GetImageFactoryPXEBaseURL() (*url.URL, error) {
	pxeBaseURL := p.Registries.GetImageFactoryPXEBaseURL()
	if pxeBaseURL != "" {
		return url.Parse(pxeBaseURL)
	}

	factoryBaseURL := p.Registries.GetImageFactoryBaseURL()

	url, err := url.Parse(factoryBaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL specified for the image factory: %w", err)
	}

	url.Host = fmt.Sprintf("pxe.%s", url.Host)

	return url, nil
}

// GetOIDCIssuerEndpoint returns the OIDC issuer endpoint.
func (p *Params) GetOIDCIssuerEndpoint() (string, error) {
	u, err := url.Parse(p.Services.Api.URL())
	if err != nil {
		return "", err
	}

	u.Path, err = url.JoinPath(u.Path, "/oidc")
	if err != nil {
		return "", err
	}

	return u.String(), nil
}

// PopulateFallbacks in the config file.
func (p *Params) PopulateFallbacks() {
	// copy the keys from the main API server if kubernetes proxy doesn't have certs defined explicitly.
	if !p.Services.KubernetesProxy.IsSecure() {
		p.Services.KubernetesProxy.SetCertFile(p.Services.Api.GetCertFile())
		p.Services.KubernetesProxy.SetKeyFile(p.Services.Api.GetKeyFile())
	}

	// copy the keys from the main API server if dev server proxy doesn't have certs defined explicitly.
	if !p.Services.DevServerProxy.IsSecure() {
		p.Services.DevServerProxy.SetCertFile(p.Services.Api.GetCertFile())
		p.Services.DevServerProxy.SetKeyFile(p.Services.Api.GetKeyFile())
	}

	if p.Auth.Auth0.InitialUsers != nil && p.Auth.InitialUsers == nil { //nolint:staticcheck
		p.Auth.InitialUsers = p.Auth.Auth0.InitialUsers //nolint:staticcheck
	}
}

func getLocalIPOrEmpty() string {
	ip, _ := getLocalIP() //nolint:errcheck

	return ip
}

// getLocalIP returns the non-loopback local IP of the host, preferring IPv4 over IPv6.
func getLocalIP() (string, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	var firstIPV6 string

	for _, address := range addresses {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}

			if firstIPV6 == "" {
				firstIPV6 = ipnet.IP.String()
			}
		}
	}

	if firstIPV6 != "" {
		return firstIPV6, nil
	}

	return "", errors.New("could not determine local IP address")
}

// EtcdBackupStorage defines etcd backup storage type.
type EtcdBackupStorage string

const (
	// EtcdBackupTypeNone is the no backup storage type.
	EtcdBackupTypeNone EtcdBackupStorage = "none"
	// EtcdBackupTypeS3 is the S3 backup storage type.
	EtcdBackupTypeS3 EtcdBackupStorage = "s3"
	// EtcdBackupTypeFS is the filesystem backup storage type.
	EtcdBackupTypeFS EtcdBackupStorage = "local"
)
