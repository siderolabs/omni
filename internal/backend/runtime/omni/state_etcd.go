// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
	"github.com/cosi-project/runtime/pkg/state/impl/store/compression"
	"github.com/cosi-project/runtime/pkg/state/impl/store/encryption"
	"github.com/cosi-project/state-etcd/pkg/state/impl/etcd"
	"github.com/siderolabs/gen/xslices"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/runtime/keyprovider"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// compressionThresholdBytes is the minimum marshaled size of the data to be considered for compression.
const compressionThresholdBytes = 2048

func buildEtcdPersistentState(ctx context.Context, params *config.Params, logger *zap.Logger, f func(context.Context, namespaced.StateBuilder) error) error {
	prefix := fmt.Sprintf("/omni/%s", url.PathEscape(params.AccountID))

	return getEtcdClient(ctx, &params.Storage.Etcd, logger, func(ctx context.Context, etcdClient *clientv3.Client) error {
		return etcdElections(ctx, etcdClient, prefix, logger, func(ctx context.Context, _ *clientv3.Client) error {
			cipher, err := makeCipher(params.AccountID, params.Storage.Etcd, etcdClient, logger) //nolint:contextcheck
			if err != nil {
				return err
			}

			salt := sha256.Sum256([]byte(params.AccountID))

			etcdState := etcd.NewState(
				etcdClient,
				encryption.NewMarshaler(
					compression.NewMarshaler(
						store.ProtobufMarshaler{},
						compression.ZStd(),
						compressionThresholdBytes,
					),
					cipher,
				),
				etcd.WithKeyPrefix(prefix),
				etcd.WithSalt(salt[:]),
			)

			stateBuilder := func(resource.Namespace) state.CoreState {
				// etcdState handles all namespaces in a single instance
				return etcdState
			}

			return f(ctx, stateBuilder)
		})
	})
}

func makeCipher(name string, etcdParams config.EtcdParams, etcdClient etcd.Client, logger *zap.Logger) (*encryption.Cipher, error) {
	publicKeys, err := loadPublicKeys(etcdParams)
	if err != nil {
		return nil, err
	}

	loader, err := NewLoader(etcdParams.PrivateKeySource, logger)
	if err != nil {
		return nil, err
	}

	privateKey, err := loader.PrivateKey()
	if err != nil {
		return nil, err
	}

	provider, err := keyprovider.New(etcdClient, hexHash(name), privateKey, publicKeys, logger)
	if err != nil {
		return nil, err
	}

	return encryption.NewCipher(provider), nil
}

func hexHash(name string) string {
	result := sha256.Sum256([]byte(name))

	return hex.EncodeToString(result[:])
}

func loadPublicKeys(params config.EtcdParams) ([]keyprovider.PublicKeyData, error) {
	publicKeys := make([]keyprovider.PublicKeyData, 0, len(params.PublicKeyFiles))

	for _, source := range params.PublicKeyFiles {
		fileData, err := os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("failed to read public key file '%s': %w", source, err)
		}

		data, err := keyprovider.MakePublicKeyData(string(fileData))
		if err != nil {
			return nil, fmt.Errorf("incorrect public key data '%s': %w", source, err)
		}

		publicKeys = append(publicKeys, data)
	}

	return publicKeys, nil
}

func getEtcdClient(ctx context.Context, params *config.EtcdParams, logger *zap.Logger, f func(context.Context, *clientv3.Client) error) error {
	logger = logger.With(logging.Component("server"))

	if params.Embedded {
		return getEmbeddedEtcdClient(ctx, params, logger, f)
	}

	return getExternalEtcdClient(ctx, params, logger, f)
}

func getEmbeddedEtcdClient(ctx context.Context, params *config.EtcdParams, logger *zap.Logger, f func(context.Context, *clientv3.Client) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger = logger.WithOptions(
		// never enable debug logs for etcd, they are too chatty
		zap.IncreaseLevel(zap.InfoLevel),
	).With(logging.Component("embedded_etcd"))

	logger.Info("starting embedded etcd server", zap.String("data_dir", params.EmbeddedDBPath))

	cfg := embed.NewConfig()
	cfg.Dir = params.EmbeddedDBPath
	cfg.EnableGRPCGateway = false
	cfg.LogLevel = "info"
	cfg.ZapLoggerBuilder = embed.NewZapLoggerBuilder(logger)
	cfg.AuthToken = ""
	cfg.AutoCompactionMode = "periodic"
	cfg.AutoCompactionRetention = "5h"
	cfg.ExperimentalCompactHashCheckEnabled = true
	cfg.ExperimentalInitialCorruptCheck = true
	cfg.UnsafeNoFsync = params.EmbeddedUnsafeFsync

	peerURL, err := url.Parse("http://localhost:0")
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	cfg.ListenPeerUrls = []url.URL{*peerURL}

	clientURLs := make([]url.URL, 0, len(params.Endpoints))

	for _, endpoint := range params.Endpoints {
		clientURL, parseErr := url.Parse(endpoint)
		if parseErr != nil {
			return fmt.Errorf("failed to parse URL: %w", parseErr)
		}

		clientURLs = append(clientURLs, *clientURL)
	}

	cfg.ListenClientUrls = clientURLs

	embeddedServer, err := embed.StartEtcd(cfg)
	if err != nil {
		return fmt.Errorf("failed to start embedded etcd: %w", err)
	}

	panichandler.Go(func() {
		for etcdErr := range embeddedServer.Err() {
			if etcdErr != nil {
				logger.Error("embedded etcd error", zap.Error(etcdErr))

				cancel()
			}
		}
	}, logger)

	// give etcd some time to start
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()

	select {
	case <-embeddedServer.Server.ReadyNotify():
	case <-time.After(15 * time.Second):
		embeddedServer.Close()

		return errors.New("etcd failed to start")
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   xslices.Map(embeddedServer.Clients, func(l net.Listener) string { return l.Addr().String() }),
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{
			grpc.WithBlock(), //nolint:staticcheck
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(constants.GRPCMaxMessageSize)),
			grpc.WithSharedWriteBuffer(true),
		},
		Logger: logger.WithOptions(
			// never enable debug logs for etcd client, they are too chatty
			zap.IncreaseLevel(zap.InfoLevel),
		).With(logging.Component("etcd_client")),
	})
	if err != nil {
		return err
	}

	closer := func() error {
		if err = cli.Close(); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("error closing client: %w", err)
		}

		embeddedServer.Close()

		select {
		case <-embeddedServer.Server.StopNotify():
		case <-time.After(15 * time.Second):
			return errors.New("timeout stopping etcd server")
		}

		return nil
	}

	defer func() {
		if err = closer(); err != nil {
			logger.Error("error stopping embedded etcd", zap.Error(err))
		}
	}()

	return f(ctx, cli)
}

func getExternalEtcdClient(ctx context.Context, params *config.EtcdParams, logger *zap.Logger, f func(context.Context, *clientv3.Client) error) error {
	if len(params.Endpoints) == 0 {
		return errors.New("no etcd endpoints provided")
	}

	logger.Info("starting etcd client",
		zap.Strings("endpoints", params.Endpoints),
		zap.String("cert_path", params.CertPath),
		zap.String("key_path", params.KeyPath),
		zap.String("ca_path", params.CAPath),
	)

	tlsInfo := transport.TLSInfo{
		CertFile:      params.CertPath,
		KeyFile:       params.KeyPath,
		TrustedCAFile: params.CAPath,
	}

	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return fmt.Errorf("error building etcd client TLS config: %w", err)
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:            params.Endpoints,
		DialKeepAliveTime:    params.DialKeepAliveTime,
		DialKeepAliveTimeout: params.DialKeepAliveTimeout,
		DialTimeout:          5 * time.Second,
		DialOptions: []grpc.DialOption{
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(constants.GRPCMaxMessageSize)),
			grpc.WithSharedWriteBuffer(true),
		},
		TLS: tlsConfig,
		Logger: logger.WithOptions(
			// never enable debug logs for etcd client, they are too chatty
			zap.IncreaseLevel(zap.InfoLevel),
		).With(logging.Component("etcd_client")),
	})
	if err != nil {
		return err
	}

	closer := func() error {
		if err = cli.Close(); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("error closing client: %w", err)
		}

		return nil
	}

	defer func() {
		if err = closer(); err != nil {
			logger.Error("error stopping embedded etcd", zap.Error(err))
		}
	}()

	return f(ctx, cli)
}
