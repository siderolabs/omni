// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import (
	"errors"
	"time"
)

// EtcdBackup defines etcd backup configs.
type EtcdBackup struct {
	LocalPath         string        `yaml:"localPath" validate:"excluded_with=S3Enabled"`
	S3Enabled         bool          `yaml:"s3Enabled" validate:"excluded_with=LocalPath"`
	TickInterval      time.Duration `yaml:"tickInterval"`
	MinInterval       time.Duration `yaml:"minInterval"`
	MaxInterval       time.Duration `yaml:"maxInterval"`
	UploadLimitMbps   uint64        `yaml:"uploadLimitMbps"`
	DownloadLimitMbps uint64        `yaml:"downloadLimitMbps"`
	Jitter            time.Duration `yaml:"jitter"`
}

// GetStorageType returns the storage type.
func (ebp EtcdBackup) GetStorageType() (EtcdBackupStorage, error) {
	if ebp.LocalPath != "" && ebp.S3Enabled {
		return "", errors.New("both localPath and s3 are set")
	}

	switch {
	case ebp.LocalPath == "" && !ebp.S3Enabled:
		return EtcdBackupTypeS3, nil
	case ebp.LocalPath != "":
		return EtcdBackupTypeFS, nil
	case ebp.S3Enabled:
		return EtcdBackupTypeS3, nil
	default:
		return "", errors.New("unknown backup storage type")
	}
}
