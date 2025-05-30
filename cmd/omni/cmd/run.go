// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/zapr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"

	"github.com/siderolabs/omni/client/pkg/compression"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/version"
)

// RunService starts the main Omni server.
func RunService(ctx context.Context, logger *zap.Logger, params *config.Params) error {
	cfg := config.InitDefault()

	raw, err := yaml.Marshal(params)
	if err != nil {
		return err
	}

	if err = yaml.Unmarshal(raw, &cfg); err != nil {
		return err
	}

	config.Config = cfg

	if err = compression.InitConfig(config.Config.ConfigDataCompression.Enabled); err != nil {
		return err
	}

	logger.Info("initialized resource compression config", zap.Bool("enabled", config.Config.ConfigDataCompression.Enabled))

	// set kubernetes logger to use warn log level and use zap
	klog.SetLogger(zapr.NewLogger(logger.WithOptions(zap.IncreaseLevel(zapcore.WarnLevel)).With(logging.Component("kubernetes"))))

	if constants.IsDebugBuild {
		logger.Warn("running debug build")
	}

	for _, registryMirror := range rootCmdArgs.registryMirrors {
		hostname, endpoint, ok := strings.Cut(registryMirror, "=")
		if !ok {
			return fmt.Errorf("invalid registry mirror spec: %q", registryMirror)
		}

		config.Config.DefaultConfigGenOptions = append(config.Config.DefaultConfigGenOptions, generate.WithRegistryMirror(hostname, endpoint))
	}

	logger.Info("starting Omni", zap.String("version", version.Tag))

	logger.Debug("using config", zap.Any("config", config.Config))

	if cfg.RunDebugServer {
		panichandler.Go(func() {
			runDebugServer(ctx, logger)
		}, logger)
	}

	// this global context propagates into all controllers and any other background activities
	ctx = actor.MarkContextAsInternalActor(ctx)

	err = omni.NewState(ctx, config.Config, logger, prometheus.DefaultRegisterer, runWithState(logger))
	if err != nil {
		return fmt.Errorf("failed to run Omni: %w", err)
	}

	return nil
}
