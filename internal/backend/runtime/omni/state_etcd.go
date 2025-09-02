// Copyright (c) 2025 Sidero Labs, Inc.
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
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/state/impl/store"
	"github.com/cosi-project/runtime/pkg/state/impl/store/compression"
	"github.com/cosi-project/runtime/pkg/state/impl/store/encryption"
	"github.com/cosi-project/state-etcd/pkg/state/impl/etcd"
	"github.com/hashicorp/go-multierror"
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

func newEtcdPersistentState(ctx context.Context, params *config.Params, logger *zap.Logger) (state *PersistentState, err error) {
	prefix := fmt.Sprintf("/omni/%s", url.PathEscape(params.Account.ID))

	var etcdState EtcdState

	etcdState, err = getEtcdState(&params.Storage.Default.Etcd, logger)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			logger.Error("closing etcd state after the error")

			if e := etcdState.Close(); e != nil {
				logger.Error("failed to gracefully close etcd state", zap.Error(e))
			}
		}
	}()

	if params.Storage.Default.Etcd.RunElections || !params.Storage.Default.Etcd.Embedded {
		err = etcdState.RunElections(ctx, prefix, logger)
		if err != nil {
			return nil, err
		}
	} else {
		logger.Info("skipped elections",
			zap.Bool("embedded", params.Storage.Default.Etcd.Embedded),
			zap.Bool("force_elections", params.Storage.Default.Etcd.RunElections),
		)
	}

	var cipher *encryption.Cipher

	cipher, err = makeCipher(params.Account.ID, params.Storage.Default.Etcd, etcdState.Client(), logger) //nolint:contextcheck
	if err != nil {
		return nil, err
	}

	salt := sha256.Sum256([]byte(params.Account.ID))

	coreState := etcd.NewState(
		etcdState.Client(),
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

	return &PersistentState{
		State:  coreState,
		Close:  etcdState.Close,
		errors: etcdState.err(),
	}, nil
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

func getEtcdState(params *config.EtcdParams, logger *zap.Logger) (EtcdState, error) {
	logger = logger.With(logging.Component("server"))

	if params.Embedded {
		return getEmbeddedEtcdState(params, logger)
	}

	return getExternalEtcdState(params, logger)
}

// getEmbeddedEtcdState runs the embedded etcd and creates a client for it.
func getEmbeddedEtcdState(params *config.EtcdParams, logger *zap.Logger) (EtcdState, error) {
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
	cfg.WarningUnaryRequestDuration = embed.DefaultWarningUnaryRequestDuration

	peerURL, err := url.Parse("http://localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	cfg.ListenPeerUrls = []url.URL{*peerURL}

	clientURLs := make([]url.URL, 0, len(params.Endpoints))

	for _, endpoint := range params.Endpoints {
		clientURL, parseErr := url.Parse(endpoint)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse URL: %w", parseErr)
		}

		clientURLs = append(clientURLs, *clientURL)
	}

	cfg.ListenClientUrls = clientURLs

	embeddedServer, err := embed.StartEtcd(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to start embedded etcd: %w", err)
	}

	errs := make(chan error, 1)

	panichandler.Go(func() {
		for etcdErr := range embeddedServer.Err() {
			if etcdErr != nil {
				errs <- etcdErr

				return
			}
		}
	}, logger)

	// give etcd some time to start
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()

	select {
	case <-embeddedServer.Server.ReadyNotify():
	case <-embeddedServer.Err():
		embeddedServer.Close()

		return nil, errors.New("etcd failed to start")
	case <-time.After(15 * time.Second):
		embeddedServer.Close()

		return nil, errors.New("etcd failed to start")
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   xslices.Map(embeddedServer.Clients, func(l net.Listener) string { return l.Addr().String() }),
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(constants.GRPCMaxMessageSize)),
			grpc.WithSharedWriteBuffer(true),
		},
		Logger: logger.WithOptions(
			// never enable debug logs for etcd client, they are too chatty
			zap.IncreaseLevel(zap.InfoLevel),
		).With(logging.Component("etcd_client")),
	})
	if err != nil {
		embeddedServer.Close()

		return nil, err
	}

	return &embeddedEtcd{
		etcdState: etcdState{
			client:    cli,
			elections: map[string]*etcdElections{},
			errors:    errs,
		},
		embeddedServer: embeddedServer,
	}, nil
}

// getExternalEtcdState creates a client for the external etcd.
func getExternalEtcdState(params *config.EtcdParams, logger *zap.Logger) (EtcdState, error) {
	if len(params.Endpoints) == 0 {
		return nil, errors.New("no etcd endpoints provided")
	}

	logger.Info("starting etcd client",
		zap.Strings("endpoints", params.Endpoints),
		zap.String("cert_path", params.CertFile),
		zap.String("key_path", params.KeyFile),
		zap.String("ca_path", params.CAFile),
	)

	tlsInfo := transport.TLSInfo{
		CertFile:      params.CertFile,
		KeyFile:       params.KeyFile,
		TrustedCAFile: params.CAFile,
	}

	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error building etcd client TLS config: %w", err)
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
		return nil, err
	}

	return &externalEtcd{
		etcdState: etcdState{
			elections: map[string]*etcdElections{},
			client:    cli,
		},
	}, nil
}

// EtcdState starts etcd backed COSI state.
type EtcdState interface {
	Client() *clientv3.Client
	Close() error
	RunElections(context.Context, string, *zap.Logger) error
	StopElections(string) error

	err() <-chan error
}

type etcdState struct {
	client      *clientv3.Client
	elections   map[string]*etcdElections
	errors      chan error
	electionsMu sync.Mutex
}

// RunElections allows using global concurrency locks in Omni.
func (e *etcdState) RunElections(ctx context.Context, prefix string, logger *zap.Logger) error {
	elections := newEtcdElections(logger)

	e.electionsMu.Lock()
	e.elections[prefix] = elections
	e.electionsMu.Unlock()

	return elections.run(ctx, e.client, prefix, e.errors)
}

// StopElections unlocks the used prefix.
func (e *etcdState) StopElections(prefix string) error {
	e.electionsMu.Lock()
	defer e.electionsMu.Unlock()

	ee, ok := e.elections[prefix]
	if !ok {
		return nil
	}

	if err := ee.stop(); err != nil {
		return err
	}

	delete(e.elections, prefix)

	return nil
}

func (e *etcdState) stopAllElections() error {
	e.electionsMu.Lock()
	defer e.electionsMu.Unlock()

	for _, ee := range e.elections {
		if err := ee.stop(); err != nil {
			return err
		}
	}

	return nil
}

func (e *etcdState) err() <-chan error {
	return e.errors
}

type embeddedEtcd struct {
	embeddedServer *embed.Etcd
	etcdState
}

func (e *embeddedEtcd) Client() *clientv3.Client {
	return e.client
}

func (e *embeddedEtcd) Close() error {
	var errs error

	if err := e.stopAllElections(); err != nil && !errors.Is(err, context.Canceled) {
		errs = multierror.Append(errs, err)
	}

	if err := e.client.Close(); err != nil && !errors.Is(err, context.Canceled) {
		errs = multierror.Append(errs, err)
	}

	e.embeddedServer.Close()

	select {
	case <-e.embeddedServer.Server.StopNotify():
	case <-time.After(15 * time.Second):
		errs = multierror.Append(errs, errors.New("timeout stopping etcd server"))
	}

	return errs
}

type externalEtcd struct {
	etcdState
}

func (e *externalEtcd) Client() *clientv3.Client {
	return e.client
}

func (e *externalEtcd) Close() error {
	if err := e.stopAllElections(); err != nil {
		return err
	}

	return e.client.Close()
}
