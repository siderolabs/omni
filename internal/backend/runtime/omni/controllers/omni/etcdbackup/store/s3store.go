// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package store

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store/internal/s3middleware"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/etcdbackup/crypt"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/etcdbackup/s3store"
)

// S3StoreFactory is a factory for S3 stores.
type S3StoreFactory struct { //nolint:govet
	mx             sync.Mutex
	store          etcdbackup.Store
	err            error
	bucket         string
	upThroughput   uint64
	downThroughput uint64
}

// NewS3StoreFactory returns a new S3 store factory.
func NewS3StoreFactory() Factory {
	return &S3StoreFactory{}
}

// ErrS3NotInitialized is returned when the store is not initialized.
var ErrS3NotInitialized = errors.New("s3 store is not initialized")

// GetStore returns the store and error. If the store is not initialized, it returns [ErrS3NotInitialized].
func (sf *S3StoreFactory) GetStore() (etcdbackup.Store, error) {
	sf.mx.Lock()
	defer sf.mx.Unlock()

	if sf.store == nil && sf.err == nil {
		return nil, ErrS3NotInitialized
	}

	return sf.store, sf.err
}

// Start starts watching the state for [*omni.EtcdBackupS3Conf] updates and updates the store and error accordingly.
func (sf *S3StoreFactory) Start(ctx context.Context, st state.State, logger *zap.Logger) error {
	if err := setStatus(ctx, st, "s3", "initializing"); err != nil {
		return err
	}

	eventCh := make(chan safe.WrappedStateEvent[*omni.EtcdBackupS3Conf])

	err := safe.StateWatch(ctx, st, omni.NewEtcdBackupS3Conf().Metadata(), eventCh)
	if err != nil {
		return fmt.Errorf("failed to watch EtcdBackupS3Conf: %w", err)
	}

	logger.Debug("s3 store factory started")

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				logger.Debug("s3 store factory context canceled")

				return nil
			}

			return fmt.Errorf("store factory context error: %w", ctx.Err())
		case ev := <-eventCh:
			if err = ev.Error(); err != nil {
				logger.Error("s3 store factory error", zap.Error(err))

				return fmt.Errorf("s3 store factory error: %w", err)
			}

			var resource *omni.EtcdBackupS3Conf

			if ev.Type() != state.Destroyed {
				resource, err = ev.Resource()
				if err != nil {
					logger.Error("s3 store: got incorrect resource", zap.Error(err))

					return fmt.Errorf("s3 store: got incorrect resource: %w", err)
				}
			}

			err = sf.updateStore(ctx, st, resource, logger)
			if err != nil {
				logger.Error("s3 store factory error", zap.Error(err))

				return fmt.Errorf("s3 store factory error: %w", err)
			}
		}
	}
}

func (sf *S3StoreFactory) updateStore(ctx context.Context, st state.State, resource *omni.EtcdBackupS3Conf, logger *zap.Logger) error {
	sf.mx.Lock()
	defer sf.mx.Unlock()

	sf.store = nil
	sf.err = nil
	sf.bucket = ""

	if IsEmptyS3Conf(resource) {
		logger.Debug("s3 store client is now nil")

		return updateS3Status(ctx, st, "not initialized")
	}

	client, bucket, err := S3ClientFromResource(ctx, resource)
	if err != nil {
		sf.err = err

		return updateS3Status(ctx, st, err.Error())
	}

	sf.store = crypt.NewStore(s3store.NewStore(client, bucket, sf.upThroughput, sf.downThroughput))
	sf.bucket = bucket

	logger.Debug("s3 store client is now set", zap.String("bucket", bucket))

	return updateS3Status(ctx, st, "")
}

// S3ClientFromResource returns an S3 client and a bucket name.
func S3ClientFromResource(ctx context.Context, s3Conf *omni.EtcdBackupS3Conf) (*s3.Client, string, error) {
	bucket := s3Conf.TypedSpec().Value.GetBucket()
	if bucket == "" {
		return nil, "", errors.New("bucket must be specified")
	}

	accessKey := s3Conf.TypedSpec().Value.GetAccessKeyId()
	secretKey := s3Conf.TypedSpec().Value.GetSecretAccessKey()
	sessionToken := s3Conf.TypedSpec().Value.GetSessionToken()

	if (accessKey == "" && secretKey != "") || (accessKey != "" && secretKey == "") {
		return nil, "", errors.New("access key and secret key must be specified together")
	}

	var opts []func(*awsConfig.LoadOptions) error

	if accessKey == "" && secretKey == "" {
		opts = append(opts, awsConfig.WithCredentialsProvider(ec2rolecreds.New()))
	} else {
		opts = append(opts, awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken)))
	}

	region := s3Conf.TypedSpec().Value.GetRegion()
	if region != "" {
		opts = append(opts, awsConfig.WithRegion(region))
	}

	var baseEndpoint string

	if endpoint := s3Conf.TypedSpec().Value.GetEndpoint(); endpoint != "" {
		if strings.Contains(endpoint, "storage.googleapis.com") {
			opts = append(opts, func(o *awsConfig.LoadOptions) error {
				s3middleware.IgnoreSigningHeaders(o, []string{"Accept-Encoding"})

				return nil
			})
		}

		if strings.HasPrefix(endpoint, "http://") {
			opts = append(opts, awsConfig.WithHTTPClient(&http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			}))
		}

		baseEndpoint = endpoint
	}

	loadedCfg, err := awsConfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load aws config: %w", err)
	}

	client := s3.NewFromConfig(loadedCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(baseEndpoint)
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.DisableLogOutputChecksumValidationSkipped = true
	})

	_, err = client.ListObjects(ctx, &s3.ListObjectsInput{Bucket: new(bucket)})
	if err != nil {
		return nil, "", fmt.Errorf("failed to list objects in bucket %q: %w", bucket, err)
	}

	return client, bucket, nil
}

// Description returns a description of the store.
func (sf *S3StoreFactory) Description() string {
	sf.mx.Lock()
	defer sf.mx.Unlock()

	return fmt.Sprintf("s3 store: bucket %q", sf.bucket)
}

// SetThroughputs sets the download and upload throughput for the store.
func (sf *S3StoreFactory) SetThroughputs(up, down uint64) {
	sf.mx.Lock()
	defer sf.mx.Unlock()

	sf.upThroughput = up
	sf.downThroughput = down
}

// IsEmptyS3Conf returns true if the resource is empty.
func IsEmptyS3Conf(res *omni.EtcdBackupS3Conf) bool {
	if res == nil {
		return true
	}

	value := res.TypedSpec().Value

	return value.GetAccessKeyId() == "" &&
		value.GetSecretAccessKey() == "" &&
		value.GetSessionToken() == "" &&
		value.GetBucket() == "" &&
		value.GetRegion() == "" &&
		value.GetEndpoint() == ""
}
