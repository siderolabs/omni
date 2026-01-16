// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import (
	"errors"
)

// GetStorageType returns the storage type.
func (s *EtcdBackup) GetStorageType() (EtcdBackupStorage, error) {
	localPath := s.GetLocalPath()
	s3Enabled := s.GetS3Enabled()

	if localPath != "" && s3Enabled {
		return "", errors.New("both localPath and s3 are set")
	}

	switch {
	case localPath == "" && !s3Enabled:
		return EtcdBackupTypeS3, nil
	case localPath != "":
		return EtcdBackupTypeFS, nil
	default:
		return EtcdBackupTypeS3, nil
	}
}
