// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package access

import (
	"context"
	"fmt"
	"os"

	"github.com/blang/semver"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/omnictl/config"
	"github.com/siderolabs/omni/client/pkg/version"
)

const (
	// EndpointEnvVar is the name of the environment variable that contains the Omni endpoint.
	EndpointEnvVar = "OMNI_ENDPOINT"
)

type clientOptions struct {
	skipAuth bool
}

// ClientOption is a functional option for the client.
type ClientOption func(*clientOptions)

// WithSkipAuth configures the client to skip the authentication interception.
func WithSkipAuth(skipAuth bool) ClientOption {
	return func(o *clientOptions) {
		o.skipAuth = skipAuth
	}
}

// WithClient initializes the Omni API client.
//
//nolint:gocognit
func WithClient(f func(ctx context.Context, client *client.Client) error, clientOpts ...ClientOption) error {
	_, err := config.Init(CmdFlags.Omniconfig, true)
	if err != nil {
		return err
	}

	cliOpts := clientOptions{}

	for _, opt := range clientOpts {
		opt(&cliOpts)
	}

	return WithContext(func(ctx context.Context) error {
		opts := []client.Option{
			client.WithInsecureSkipTLSVerify(CmdFlags.InsecureSkipTLSVerify),
		}

		var (
			serviceAccount string
			url            string
		)

		envKey, valueBase64 := serviceaccount.GetFromEnv()
		if envKey != "" {
			sa, saErr := serviceaccount.Decode(valueBase64)
			if saErr != nil {
				return saErr
			}

			serviceAccount = sa.Name

			opts = append(opts, client.WithServiceAccount(valueBase64))
		}

		if serviceAccount == "" {
			conf, err := config.Current()
			if err != nil {
				return err
			}

			contextName := conf.Context
			if CmdFlags.Context != "" {
				contextName = CmdFlags.Context
			}

			configCtx, err := conf.GetContext(CmdFlags.Context)
			if err != nil {
				return err
			}

			if configCtx.Auth.Basic != "" { //nolint:staticcheck
				fmt.Fprintf(os.Stderr, "[WARN] basic auth is deprecated and has no effect\n")
			}

			opts = append(opts, client.WithUserAccount(contextName, configCtx.Auth.SideroV1.Identity), client.WithCustomKeysDir(CmdFlags.SideroV1KeysDir))

			if configCtx.URL == config.PlaceholderURL {
				return fmt.Errorf("context %q has not been configured, you will need to set it manually", contextName)
			}

			url = configCtx.URL
		}

		endpointEnv := os.Getenv(EndpointEnvVar)
		if endpointEnv != "" {
			url = endpointEnv
		}

		loggerCfg := zap.NewDevelopmentConfig()
		loggerCfg.Development = false

		logger, err := loggerCfg.Build()
		if err != nil {
			return err
		}

		opts = append(opts, client.WithOmniClientOptions(omni.WithRetryLogger(logger)))

		client, err := client.New(url, opts...)
		if err != nil {
			return err
		}

		defer client.Close() //nolint:errcheck

		if !cliOpts.skipAuth {
			// bootstrap the client, and perform auth/re-auth if needed via the unary call
			// stream interceptor can't catch the auth error, as it comes async
			_, err = client.Omni().State().Get(ctx, resource.NewMetadata(resources.EphemeralNamespace, system.SysVersionType, system.SysVersionID, resource.VersionUndefined))
			if err != nil {
				return err
			}
		}

		if err = checkVersion(ctx, client.Omni().State()); err != nil {
			return err
		}

		return f(ctx, client)
	})
}

func checkVersion(ctx context.Context, state state.State) error {
	if version.API == 0 && !version.SuppressVersionWarning {
		fmt.Fprintln(os.Stderr, `[WARN] github.com/siderolabs/omni/client/pkg/version.API is not set, client-server version validation is disabled.
If you want to enable the version validation and disable this warning, set github.com/siderolabs/omni/client/pkg/version.SuppressVersionWarning to true.`)

		return nil
	}

	sysVersion, err := safe.StateGet[*system.SysVersion](ctx, state, system.NewSysVersion(resources.EphemeralNamespace, system.SysVersionID).Metadata())
	if err != nil {
		return err
	}

	checkVersionWarning(sysVersion)

	if sysVersion.TypedSpec().Value.BackendApiVersion == 0 { // API versions are not supported (yet) on backend, i.e., the client is newer than the backend
		return fmt.Errorf("server API does not support API versions, i.e., the server is older than the client, "+
			"please upgrade the server to have the same API version as the client: client API version %v, "+
			"client version %v, server version %v", version.API, version.Tag, sysVersion.TypedSpec().Value.BackendVersion)
	}

	// compare the API versions
	if sysVersion.TypedSpec().Value.BackendApiVersion != version.API {
		return fmt.Errorf("client API version mismatch: backend API version %v, client API version %v", sysVersion.TypedSpec().Value.BackendApiVersion, version.API)
	}

	return nil
}

func checkVersionWarning(sysVersion *system.SysVersion) {
	backendVersion, err := semver.ParseTolerant(sysVersion.TypedSpec().Value.BackendVersion)
	if err != nil {
		return
	}

	clientVersion, err := semver.ParseTolerant(version.Tag)
	if err != nil {
		return
	}

	if clientVersion.Major != backendVersion.Major || clientVersion.Minor != backendVersion.Minor {
		fmt.Fprintf(os.Stderr, "[WARN] omnictl version differs from the backend version: %q vs %q.\n", clientVersion.String(), backendVersion.String())
	}
}
